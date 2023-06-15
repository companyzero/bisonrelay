package clientdb

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/inidb"
	"github.com/companyzero/bisonrelay/internal/jsonfile"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"golang.org/x/exp/slices"
)

const (
	gcAliasesFile  = "gcaliases.json"
	invitesTable   = "invites"
	gcBlockListExt = ".blocklist"
)

type GCInvite struct {
	Invite   rpc.RMGroupInvite
	User     UserID
	ID       uint64
	Accepted bool
}

func (i *GCInvite) marshal() (string, error) {
	blob, err := json.Marshal(i)
	if err != nil {
		return "", fmt.Errorf("could not marshal invite record: %v", err)
	}
	return hex.EncodeToString(blob), nil
}

func (i *GCInvite) unmarshal(s string) error {
	blob, err := hex.DecodeString(s)
	if err != nil {
		return err
	}

	return json.Unmarshal(blob, i)
}

func (db *DB) AddGCInvite(tx ReadWriteTx, user UserID, invite rpc.RMGroupInvite) (uint64, error) {
	db.invites.NewTable(invitesTable)

	newID := func() uint64 {
		return 100000 + (db.mustRandomUint64() % (1000000 - 100000))
	}

	dbi := GCInvite{
		Invite: invite,
		User:   user,
		ID:     newID(),
	}

	// Get a random invite id.
	_, err := db.invites.Get(invitesTable, itoa(dbi.ID))
	for err == nil {
		dbi.ID = newID()
		_, err = db.invites.Get(invitesTable, itoa(dbi.ID))
	}
	if !errors.Is(err, inidb.ErrNotFound) {
		return 0, err
	}

	blob, err := dbi.marshal()
	if err != nil {
		return 0, err
	}

	if err := db.invites.Set(invitesTable, itoa(dbi.ID), blob); err != nil {
		return 0, err
	}

	if err := db.invites.Save(); err != nil {
		return 0, err
	}

	return dbi.ID, nil
}

func (db *DB) GetGCInvite(tx ReadTx, inviteID uint64) (rpc.RMGroupInvite, UserID, error) {
	var invite rpc.RMGroupInvite

	blob, err := db.invites.Get(invitesTable, itoa(inviteID))
	if err != nil {
		if errors.Is(err, inidb.ErrNotFound) {
			return invite, UserID{}, fmt.Errorf("invite %d: %w", inviteID, ErrNotFound)
		}
		return invite, UserID{}, err
	}

	var dbi GCInvite
	err = dbi.unmarshal(blob)
	if err != nil {
		return invite, UserID{}, fmt.Errorf("unable to unmarshal db gc invite")
	}

	return dbi.Invite, dbi.User, nil
}

func (db *DB) MarkGCInviteAccepted(tx ReadWriteTx, inviteID uint64) error {
	blob, err := db.invites.Get(invitesTable, itoa(inviteID))
	if err != nil {
		if errors.Is(err, inidb.ErrNotFound) {
			return fmt.Errorf("invite %d: %w", inviteID, ErrNotFound)
		}
		return err
	}

	var dbi GCInvite
	if err := dbi.unmarshal(blob); err != nil {
		return fmt.Errorf("unable to unmarshal db gc invite")
	}

	dbi.Accepted = true

	blob, err = dbi.marshal()
	if err != nil {
		return err
	}

	if err := db.invites.Set(invitesTable, itoa(dbi.ID), blob); err != nil {
		return err
	}
	if err := db.invites.Save(); err != nil {
		return err
	}

	return nil
}

func (db *DB) DelGCInvite(tx ReadWriteTx, inviteID uint64) error {
	if err := db.invites.Del(invitesTable, itoa(inviteID)); err != nil {
		return err
	}
	if err := db.invites.Save(); err != nil {
		return err
	}
	return nil
}

// DelAllInvitesToGC removes all invites to the given GC.
func (db *DB) DelAllInvitesToGC(tx ReadWriteTx, gcid zkidentity.ShortID) error {
	records := db.invites.Records(invitesTable)
	for k, v := range records {
		dbi := new(GCInvite)
		err := dbi.unmarshal(v)
		if err != nil {
			return fmt.Errorf("unable to unmarshal db gc invite: %v", err)
		}

		if dbi.Invite.ID == gcid {
			db.invites.Del(invitesTable, k)
		}
	}
	return db.invites.Save()
}

// ListGCInvites lists the GC invites. If gc is specified, lists only invites
// for the specified GCID.
func (db *DB) ListGCInvites(tx ReadTx, gc *zkidentity.ShortID) ([]*GCInvite, error) {
	records := db.invites.Records(invitesTable)
	res := make([]*GCInvite, 0, len(records))
	for _, v := range records {
		dbi := new(GCInvite)
		err := dbi.unmarshal(v)
		if err != nil {
			return nil, fmt.Errorf("unable to unmarshal db gc invite: %v", err)
		}

		if gc != nil && !dbi.Invite.ID.ConstantTimeEq(gc) {
			continue
		}

		res = append(res, dbi)
	}

	return res, nil
}

// FindGCInvite looks for an invite to a GC with a given token.
func (db *DB) FindGCInvite(tx ReadTx, gcID zkidentity.ShortID, token uint64) (rpc.RMGroupInvite, UserID, uint64, error) {
	fail := func(err error) (rpc.RMGroupInvite, UserID, uint64, error) {
		return rpc.RMGroupInvite{}, UserID{}, 0, err
	}

	var dbi GCInvite
	records := db.invites.Records(invitesTable)
	for k, v := range records {
		id, err := atoi(k)
		if err != nil {
			return fail(fmt.Errorf("invalid invite key: %v", err))
		}

		err = dbi.unmarshal(v)
		if err != nil {
			return fail(fmt.Errorf("unable to unmarshal db gc invite: %v", err))
		}

		if dbi.Invite.ID == gcID && dbi.Invite.Token == token {
			return dbi.Invite, dbi.User, id, nil
		}
	}

	return fail(fmt.Errorf("gc invite: %w", ErrNotFound))
}

// FindAcceptedGCInvite looks for an invite to a GC sent by the specified user
// that has been previously marked as accepted.
func (db *DB) FindAcceptedGCInvite(tx ReadTx, gcID, uid zkidentity.ShortID) (rpc.RMGroupInvite, uint64, error) {
	fail := func(err error) (rpc.RMGroupInvite, uint64, error) {
		return rpc.RMGroupInvite{}, 0, err
	}

	var dbi GCInvite
	records := db.invites.Records(invitesTable)
	for k, v := range records {
		id, err := atoi(k)
		if err != nil {
			return fail(fmt.Errorf("invalid invite key: %v", err))
		}

		err = dbi.unmarshal(v)
		if err != nil {
			return fail(fmt.Errorf("unable to unmarshal db gc invite"))
		}

		if dbi.Accepted && dbi.Invite.ID == gcID && dbi.User == uid {
			return dbi.Invite, id, nil
		}
	}

	return fail(fmt.Errorf("gc invite: %w", ErrNotFound))
}

// readGC reads the gc from the given filename into gl.
func (db *DB) readGC(filename string, gc *rpc.RMGroupList) error {
	gcJSON, err := os.ReadFile(filename)
	if err != nil && os.IsNotExist(err) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(gcJSON, &gc)
}

func (db *DB) GetGC(tx ReadTx, id zkidentity.ShortID) (rpc.RMGroupList, error) {
	var gc rpc.RMGroupList
	filename := filepath.Join(db.root, groupchatDir, id.String())
	err := db.readGC(filename, &gc)
	if errors.Is(err, ErrNotFound) {
		return gc, fmt.Errorf("gc %s: %w", id, ErrNotFound)
	}
	return gc, err
}

func (db *DB) GetGCAliases(tx ReadTx) (map[string]zkidentity.ShortID, error) {
	var aliasMap map[string]zkidentity.ShortID

	filename := filepath.Join(db.root, gcAliasesFile)
	err := db.readJsonFile(filename, &aliasMap)
	if errors.Is(err, ErrNotFound) {
		// New map.
		return map[string]zkidentity.ShortID{}, nil
	}
	return aliasMap, err
}

// SetGCAlias sets the alias for the specified GC ID as the name. If gcID is
// empty, the name is removed from the alias map. If gcID is filled but name is
// empty, the entry that points to the specified gcID is removed.
func (db *DB) SetGCAlias(tx ReadWriteTx, gcID zkidentity.ShortID, name string) (
	map[string]zkidentity.ShortID, error) {

	aliasMap, err := db.GetGCAliases(tx)
	if err != nil {
		return nil, err
	}

	if gcID.IsEmpty() {
		delete(aliasMap, name)
	} else {
		// Remove old aliases.
		for name, id := range aliasMap {
			if id == gcID {
				delete(aliasMap, name)
			}
		}

		if name != "" {
			aliasMap[name] = gcID
		}
	}

	filename := filepath.Join(db.root, gcAliasesFile)
	if err := db.saveJsonFile(filename, &aliasMap); err != nil {
		return nil, err
	}

	return aliasMap, nil
}

func (db *DB) SaveGC(tx ReadWriteTx, gc rpc.RMGroupList) error {
	gcDir := filepath.Join(db.root, groupchatDir)
	filename := filepath.Join(gcDir, gc.ID.String())
	return db.saveJsonFile(filename, gc)
}

func (db *DB) DeleteGC(tx ReadWriteTx, gcID zkidentity.ShortID) error {
	gcDir := filepath.Join(db.root, groupchatDir)
	filename := filepath.Join(gcDir, gcID.String())
	if err := os.Remove(filename); err != nil {
		return err
	}
	blockListFname := filename + gcBlockListExt
	if fileExists(blockListFname) {
		return os.Remove(blockListFname)
	}
	return nil
}

func (db *DB) ListGCs(tx ReadTx) ([]rpc.RMGroupList, error) {
	gcDir := filepath.Join(db.root, groupchatDir)
	entries, err := os.ReadDir(gcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	groups := make([]rpc.RMGroupList, 0, len(entries))
	for _, v := range entries {
		if v.IsDir() {
			continue
		}

		fname := filepath.Join(gcDir, v.Name())
		if strings.HasSuffix(fname, gcBlockListExt) {
			continue
		}

		var gc rpc.RMGroupList
		err := db.readGC(fname, &gc)
		if err != nil {
			db.log.Warnf("Unable to read gc file for listing %s: %v",
				fname, err)
			continue
		}

		groups = append(groups, gc)
	}

	return groups, nil
}

// ListGCsWithMember returns IDs for GCs that have the specified user as a
// member.
func (db *DB) ListGCsWithMember(tx ReadTx, uid UserID) ([]zkidentity.ShortID, error) {
	gcDir := filepath.Join(db.root, groupchatDir)
	entries, err := os.ReadDir(gcDir)
	if err != nil && os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var res []UserID
	for _, v := range entries {
		if v.IsDir() {
			continue
		}

		fname := filepath.Join(gcDir, v.Name())
		if strings.HasSuffix(fname, gcBlockListExt) {
			continue
		}

		var gc rpc.RMGroupList
		err := db.readGC(fname, &gc)
		if err != nil {
			db.log.Warnf("Unable to read gc file for listing %s: %v",
				fname, err)
			continue
		}

		if slices.Contains(gc.Members, uid) {
			res = append(res, gc.ID)
		}
	}

	return res, nil
}

type GCBlockList map[string]struct{}

// FilterMembers filters the list of members, removing any that is in the block list.
func (gcbl GCBlockList) FilterMembers(members []UserID) []UserID {
	res := make([]UserID, 0, len(members))
	for _, uid := range members {
		if _, ok := gcbl[uid.String()]; ok {
			continue
		}
		res = append(res, uid)
	}
	return res
}

// IsBlocked returns true if the given UID is part of the blocklist.
func (gcbl GCBlockList) IsBlocked(uid UserID) bool {
	_, ok := gcbl[uid.String()]
	return ok
}

// AddToGCBlocklist adds the given UID to the block list of the specified GC.
func (db *DB) AddToGCBlockList(tx ReadWriteTx, gcid zkidentity.ShortID, uid UserID) error {
	filename := filepath.Join(db.root, groupchatDir, gcid.String()+
		gcBlockListExt)

	var entries GCBlockList
	err := db.readJsonFile(filename, &entries)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}

	if entries == nil {
		entries = make(map[string]struct{})
	}
	entries[uid.String()] = struct{}{}

	return db.saveJsonFile(filename, &entries)
}

// RemoveFromGCBlockList removes the given UID from the block list of the
// specified GC.
func (db *DB) RemoveFromGCBlockList(tx ReadWriteTx, gcid zkidentity.ShortID, uid UserID) error {
	filename := filepath.Join(db.root, groupchatDir, gcid.String()+
		gcBlockListExt)

	var entries GCBlockList
	err := db.readJsonFile(filename, &entries)
	if errors.Is(err, ErrNotFound) {
		// Not in blocklist. NOP.
		return nil
	}
	if err != nil {
		return err
	}

	delete(entries, uid.String())
	return db.saveJsonFile(filename, entries)
}

// GetGCBlockList returns the block list of the specified GC. Returns nil if
// there is no block list.
func (db *DB) GetGCBlockList(tx ReadTx, gcid zkidentity.ShortID) (GCBlockList, error) {
	filename := filepath.Join(db.root, groupchatDir, gcid.String()+
		gcBlockListExt)

	var entries GCBlockList
	err := db.readJsonFile(filename, &entries)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	return entries, err

}

// CacheReceivedGCM stores a cached received GC message.
func (db *DB) CacheReceivedGCM(tx ReadWriteTx, rgcm clientintf.ReceivedGCMsg) error {
	filename := filepath.Join(db.root, cachedGCMsDir, rgcm.MsgID.String())
	return db.saveJsonFile(filename, rgcm)
}

// RemoveCachedRGCM removes a previously cached received GC message if it exists.
func (db *DB) RemoveCachedRGCM(tx ReadWriteTx, rgcm clientintf.ReceivedGCMsg) error {
	filename := filepath.Join(db.root, cachedGCMsDir, rgcm.MsgID.String())
	return jsonfile.RemoveIfExists(filename)
}

// ListCachedRGCMs returns any existing cached RGCM.
func (db *DB) ListCachedRGCMs(tx ReadTx) ([]clientintf.ReceivedGCMsg, error) {
	dir := filepath.Join(db.root, cachedGCMsDir)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	res := make([]clientintf.ReceivedGCMsg, 0, len(entries))
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}

		var rgcm clientintf.ReceivedGCMsg
		fname := filepath.Join(dir, entry.Name())
		if err := db.readJsonFile(fname, &rgcm); err != nil {
			db.log.Warnf("Unable to read file %s as a ReceivedGCM file: %v",
				entry.Name(), err)
			continue
		}
		res = append(res, rgcm)
	}

	return res, nil
}
