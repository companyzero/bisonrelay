package clientdb

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
)

// KXExists returns true if there's a KX procedure with the specified RV.
func (db *DB) KXExists(tx ReadTx, initialRV RawRVID) bool {
	fname := filepath.Join(db.root, kxDir, initialRV.String())
	return fileExists(fname)
}

func (db *DB) SaveKX(tx ReadWriteTx, kx KXData) error {
	dir := filepath.Join(db.root, kxDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("unable to make kx dir: %v", err)
	}

	blob, err := json.Marshal(kx)
	if err != nil {
		return fmt.Errorf("unable to marshal kxdata: %v", err)
	}

	fname := filepath.Join(dir, kx.InitialRV.String())
	if _, err := os.Stat(fname); !os.IsNotExist(err) {
		if err != nil {
			return err
		}
		return fmt.Errorf("kx with initial RV %s: %w", kx.InitialRV, ErrAlreadyExists)
	}
	return os.WriteFile(fname, blob, 0o600)
}

func (db *DB) DeleteKX(tx ReadWriteTx, initialRV RawRVID) error {
	fname := filepath.Join(db.root, kxDir, initialRV.String())
	return os.Remove(fname)
}

func (db *DB) GetKX(tx ReadTx, initialRV RawRVID) (KXData, error) {
	fname := filepath.Join(db.root, kxDir, initialRV.String())
	blob, err := os.ReadFile(fname)
	if err != nil {
		if os.IsNotExist(err) {
			return KXData{}, fmt.Errorf("kx %s: %w",
				initialRV.ShortLogID(), ErrNotFound)
		}
		return KXData{}, err
	}

	var kxd KXData
	err = json.Unmarshal(blob, &kxd)
	if err != nil {
		return KXData{}, nil
	}
	return kxd, nil
}

func (db *DB) ListKXs(tx ReadTx) ([]KXData, error) {
	dir := filepath.Join(db.root, kxDir)
	dirEntries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("unable to read content dir: %v", err)
	}

	var testID UserID
	res := make([]KXData, 0, len(dirEntries))
	for _, f := range dirEntries {
		if f.IsDir() {
			continue
		}

		fname := filepath.Join(dir, f.Name())
		if err := testID.FromString(filepath.Base(fname)); err != nil {
			db.log.Warnf("Filename %q is not a KX file", fname)
			continue
		}

		blob, err := os.ReadFile(fname)
		if err != nil {
			return nil, err
		}

		var kxd KXData
		err = json.Unmarshal(blob, &kxd)
		if err != nil {
			return nil, fmt.Errorf("unable to unmarshal KX file %s: %v",
				fname, err)
		}
		res = append(res, kxd)
	}

	return res, nil
}

// HasKXWithUser returns any outstanding KX attempt with the given user. This
// will return KXs which were created with an invitee filled to the target
// ID or when they were accepted and the remote user has the target ID.
func (db *DB) HasKXWithUser(tx ReadTx, target UserID) ([]KXData, error) {
	dir := filepath.Join(db.root, kxDir)
	dirEntries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("unable to read content dir: %v", err)
	}

	var testID UserID
	var res []KXData
	for _, f := range dirEntries {
		if f.IsDir() {
			continue
		}

		fname := filepath.Join(dir, f.Name())
		if err := testID.FromString(filepath.Base(fname)); err != nil {
			db.log.Warnf("Filename %q is not a KX file", fname)
			continue
		}

		var kxd KXData
		if err := db.readJsonFile(fname, &kxd); err != nil {
			db.log.Warnf("Unable to unmarshal %s as a KX file: %v",
				fname, err)
			continue
		}

		if (kxd.Invitee != nil && kxd.Invitee.Identity == target) ||
			kxd.Public.Identity == target {
			res = append(res, kxd)
		}
	}

	return res, nil
}

// StoreMediateIDRequested marks that a mediate id request was made on mediator
// to invite us to target.
func (db *DB) StoreMediateIDRequested(tx ReadWriteTx, mediator, target UserID) error {
	filepath := filepath.Join(db.root, inboundDir, mediator.String(),
		miRequestsDir, target.String())
	req := MediateIDRequest{
		Mediator: mediator,
		Target:   target,
		Date:     time.Now(),
	}
	return db.saveJsonFile(filepath, req)
}

// HasMediateID returns info about a request to mediate ID made to a mediator
// for introduction to a target.
func (db *DB) HasMediateID(tx ReadTx, mediator, target UserID) (MediateIDRequest, error) {
	filepath := filepath.Join(db.root, inboundDir, mediator.String(),
		miRequestsDir, target.String())
	var req MediateIDRequest
	err := db.readJsonFile(filepath, &req)
	return req, err
}

// HasAnyRecentMediateID looks if there are any attempts to mediate identity to
// the given target user with any mediator that is no older than the specified
// recentThreshold.
func (db *DB) HasAnyRecentMediateID(tx ReadTx, target UserID, recentThreshold time.Duration) (bool, error) {
	pattern := filepath.Join(db.root, inboundDir, "*", miRequestsDir,
		target.String())
	files, err := filepath.Glob(pattern)
	if err != nil {
		return false, err
	}
	if len(files) == 0 {
		return false, nil
	}

	for _, fname := range files {
		var req MediateIDRequest
		err := db.readJsonFile(fname, &req)
		if err != nil {
			return false, err
		}

		if req.Date.Before(time.Now().Add(-recentThreshold)) {
			continue
		}

		return true, nil
	}

	return false, nil
}

// RemoveMediateID removes the given request to mediate an ID.
func (db *DB) RemoveMediateID(tx ReadWriteTx, mediator, target UserID) error {
	filepath := filepath.Join(db.root, inboundDir, mediator.String(),
		miRequestsDir, target.String())
	err := os.Remove(filepath)
	if os.IsNotExist(err) {
		return nil // Not an error.
	}
	return err
}

// ListMediateIDs lists all existing mediate id requests.
func (db *DB) ListMediateIDs(tx ReadTx) ([]MediateIDRequest, error) {
	pattern := filepath.Join(db.root, inboundDir, "*", miRequestsDir, "*")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	res := make([]MediateIDRequest, 0, len(files))
	for _, fname := range files {
		var req MediateIDRequest
		err := db.readJsonFile(fname, &req)
		if err != nil {
			db.log.Warnf("Unable to read mediate id request %s: %v",
				fname, err)
			continue
		}

		res = append(res, req)
	}

	return res, nil
}

// StoreUnkxdUserInfo stores information about an unxked user.
func (db *DB) StoreUnkxdUserInfo(tx ReadWriteTx, info UnkxdUserInfo) error {
	fname := filepath.Join(db.root, unkxdUsersDir, info.UID.String())
	return db.saveJsonFile(fname, &info)
}

// ReadUnxkdUserInfo returns information about an unkxed user.
func (db *DB) ReadUnxkdUserInfo(tx ReadTx, uid UserID) (UnkxdUserInfo, error) {
	fname := filepath.Join(db.root, unkxdUsersDir, uid.String())
	var info UnkxdUserInfo
	err := db.readJsonFile(fname, &info)
	return info, err
}

// RemoveUnkxUserInfo removes the information about an unkxed user if it exists.
func (db *DB) RemoveUnkxUserInfo(tx ReadWriteTx, uid UserID) error {
	fname := filepath.Join(db.root, unkxdUsersDir, uid.String())
	return removeIfExists(fname)
}

// AddKXSearchQuery updates the search for a given KX opportunity with the
// target user and adds the specified query to the list of attempted queries.
func (db *DB) AddKXSearchQuery(tx ReadWriteTx, target UserID, search rpc.RMKXSearch, query KXSearchQuery) error {
	filename := filepath.Join(db.root, kxSearches, target.String())
	var kxs KXSearch
	err := db.readJsonFile(filename, &kxs)
	if err == nil {
		// Already exists. Replace search and look for an existing query
		// to replace or add.
		kxs.Search = search

		found := false
		for i := range kxs.Queries {
			if kxs.Queries[i].User == query.User {
				kxs.Queries[i] = query
				found = true
				break
			}
		}
		if !found {
			kxs.Queries = append(kxs.Queries, query)
		}
	} else if errors.Is(err, ErrNotFound) {
		// Does not exist. Just create a new one.
		kxs.Target = target
		kxs.Search = search
		kxs.Queries = []KXSearchQuery{query}
	} else {
		return err
	}

	return db.saveJsonFile(filename, kxs)
}

// GetKXSearch returns the KX search for a given target, if it exists.
func (db *DB) GetKXSearch(tx ReadTx, target UserID) (KXSearch, error) {
	filename := filepath.Join(db.root, kxSearches, target.String())
	var kxs KXSearch
	err := db.readJsonFile(filename, &kxs)
	return kxs, err
}

// RemoveKXSearch removes the kx search for the given target if it exists.
func (db *DB) RemoveKXSearch(tx ReadWriteTx, target UserID) error {
	filename := filepath.Join(db.root, kxSearches, target.String())
	return removeIfExists(filename)
}

// ListKXSearches lists the IDs of all outstanding users being KX searched for.
func (db *DB) ListKXSearches(tx ReadTx) ([]UserID, error) {
	dir := filepath.Join(db.root, kxSearches)
	var res []UserID
	files, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		// No KX searches yet.
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}

		var uid UserID
		if err := uid.FromString(f.Name()); err != nil {
			db.log.Debugf("File %q is not a UserID: %s", f.Name(), err)
			continue
		}

		res = append(res, uid)
	}

	return res, nil
}

// AddInitialKXAction adds an action to be taken after kx completes with the given
// initial rendezvous.
func (db *DB) AddInitialKXAction(tx ReadWriteTx, initialRV zkidentity.ShortID, action PostKXAction) error {
	filename := filepath.Join(db.root, initKXActionsDir, initialRV.String())
	var actions []PostKXAction
	err := db.readJsonFile(filename, &actions)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	actions = append(actions, action)
	return db.saveJsonFile(filename, actions)
}

// RemoveInitialKXActions removes the initial-kx actions registered for the given
// initial rendezvous.
func (db *DB) RemoveInitialKXActions(tx ReadWriteTx, initialRV zkidentity.ShortID) error {
	filename := filepath.Join(db.root, initKXActionsDir, initialRV.String())
	err := os.Remove(filename)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// InitialToPostKXAction converts an action based on initial rendezvous to a known user id.
func (db *DB) InitialToPostKXActions(tx ReadWriteTx, initialRV, target zkidentity.ShortID) error {
	filename := filepath.Join(db.root, initKXActionsDir, initialRV.String())
	var actions []PostKXAction
	err := db.readJsonFile(filename, &actions)
	switch {
	case errors.Is(err, ErrNotFound):
		return nil
	case err != nil:
		return err
	default:
	}
	for _, action := range actions {
		if err = db.AddUniquePostKXAction(tx, target, action); err != nil {
			return err
		}
	}
	return db.RemoveInitialKXActions(tx, initialRV)
}

// AddPostKXAction adds an action to be taken after kx completes with the given
// target user.
func (db *DB) AddPostKXAction(tx ReadWriteTx, target UserID, action PostKXAction) error {
	filename := filepath.Join(db.root, postKXActionsDir, target.String())
	var actions []PostKXAction
	err := db.readJsonFile(filename, &actions)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	actions = append(actions, action)
	return db.saveJsonFile(filename, actions)
}

// AddUniquePostKXAction adds a post-kx action unless an action of the same
// type and data already exists.
func (db *DB) AddUniquePostKXAction(tx ReadWriteTx, target UserID, action PostKXAction) error {
	filename := filepath.Join(db.root, postKXActionsDir, target.String())
	var actions []PostKXAction
	err := db.readJsonFile(filename, &actions)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}

	for _, act := range actions {
		if act.Type == action.Type && act.Data == action.Data {
			// Duplicated.
			return nil
		}
	}
	actions = append(actions, action)
	return db.saveJsonFile(filename, actions)
}

// ListPostKXActions lists the post-kx actions registered for the given target
// user.
func (db *DB) ListPostKXActions(tx ReadTx, target UserID) ([]PostKXAction, error) {
	filename := filepath.Join(db.root, postKXActionsDir, target.String())
	var actions []PostKXAction
	err := db.readJsonFile(filename, &actions)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	return actions, err
}

// RemovePostKXActions removes the post-kx actions registered for the given
// target user.
func (db *DB) RemovePostKXActions(tx ReadWriteTx, target UserID) error {
	filename := filepath.Join(db.root, postKXActionsDir, target.String())
	err := os.Remove(filename)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// ReadOnboardState fetches the existing onboard state of the client. It
// returns an error if there is no onboard state.
func (db *DB) ReadOnboardState(tx ReadTx) (clientintf.OnboardState, error) {
	filename := filepath.Join(db.root, onboardStateFile)
	var res clientintf.OnboardState
	err := db.readJsonFile(filename, &res)
	return res, err
}

// UpdateOnboardState updates the client onboard state.
func (db *DB) UpdateOnboardState(tx ReadWriteTx, st *clientintf.OnboardState) error {
	filename := filepath.Join(db.root, onboardStateFile)
	return db.saveJsonFile(filename, st)
}

// RemoveOnboardState removes any existing onboard state.
func (db *DB) RemoveOnboardState(tx ReadWriteTx) error {
	filename := filepath.Join(db.root, onboardStateFile)
	return removeIfExists(filename)
}

// HasOnboardState returns true if there is an existing onboard state.
func (db *DB) HasOnboardState(tx ReadTx) bool {
	filename := filepath.Join(db.root, onboardStateFile)
	return fileExists(filename)
}
