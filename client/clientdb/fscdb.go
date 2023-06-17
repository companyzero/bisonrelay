package clientdb

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/companyzero/bisonrelay/inidb"
	"github.com/companyzero/bisonrelay/internal/jsonfile"
	"github.com/companyzero/bisonrelay/internal/strescape"
	"github.com/companyzero/bisonrelay/ratchet"
	"github.com/companyzero/bisonrelay/ratchet/disk"
	"github.com/companyzero/bisonrelay/zkidentity"
)

const (
	lockFileName        = "db.lock"
	zkcServerDir        = "myserver"
	zkcServerFile       = "myserver.ini"
	ratchetFilename     = "ratchet.json"
	inboundDir          = "inbound"
	identityFilename    = "publicidentity.json"
	groupchatDir        = "groupchat"
	invitesDir          = "invites"
	contentDir          = "content"
	postsDir            = "posts"
	postsSubscribers    = "subscribers"
	postsSubscriptions  = "subscriptns"
	postsStatusExt      = ".status"
	kxDir               = "kx"
	transResetFile      = "transreset.json"
	sendqDir            = "sendqueue"
	blockedUsersFile    = "blockedusers.json"
	paidRVsDir          = "paidrvs"
	paidPushesDir       = "paidpushes"
	kxSearches          = "kxsearches"
	miRequestsDir       = "mirequests"
	postKXActionsDir    = "postkxactions"
	initKXActionsDir    = "initkxactions"
	payStatsFile        = "paystats.json"
	unackedRMsDir       = "unackedrms"
	lastConnDateFile    = "lastconndate.json"
	tipsDir             = "tips"
	onboardStateFile    = "onboard.json"
	reqResourcesDir     = "reqresources"
	recvAddrForUserFile = "onchainrecvaddr.json"
	cachedGCMsDir       = "cachedgcms"
	unkxdUsersDir       = "unkxd"

	pageSessionsDir         = "pagesessions"
	pageSessionOverviewFile = "overview.json"
)

var (
	pageFnamePattern   = jsonfile.MakeDecimalFilePattern("page-", ".json", false)
	pageSessDirPattern = jsonfile.MakeDecimalFilePattern("", "", true)

	// logLineRegexp matches the start of log lines. This matches the
	// following line examples:
	// 2023-06-01T13:08:47 <some nick> some message
	// 2023-06-01T13:08:47 * Some internal message
	//
	// Note that this uses multiline mode (?m), therefore it matches both
	// start of line and new lines when the source string is a full message.
	logLineRegexp = regexp.MustCompile(`(?m)^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}) (\*|<[^>]+>)`)
)

func (db *DB) LocalID(tx ReadTx) (*zkidentity.FullIdentity, error) {
	// myidentity
	myidb64, err := db.idb.Get("", "myidentity")
	if errors.Is(err, inidb.ErrNotFound) {
		return nil, ErrLocalIDEmpty
	} else if err != nil {
		return nil, fmt.Errorf("could not obtain myidentity record")
	}
	myidJSON, err := base64.StdEncoding.DecodeString(myidb64)
	if err != nil {
		return nil, fmt.Errorf("could not decode myidentity")
	}
	var id zkidentity.FullIdentity
	err = json.Unmarshal(myidJSON, &id)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal myidentity")
	}

	return &id, nil
}

func (db *DB) UpdateLocalID(tx ReadWriteTx, id *zkidentity.FullIdentity) error {
	myid, err := json.Marshal(id)
	if err != nil {
		return fmt.Errorf("Could not marshal identity: %v", err)
	}

	err = db.idb.Set("", "myidentity",
		base64.StdEncoding.EncodeToString(myid))
	if err != nil {
		return fmt.Errorf("could not insert record myidentity")
	}
	err = db.idb.Save()
	if err != nil {
		return fmt.Errorf("could not save server: %v", err)
	}

	return nil
}

func (db *DB) ServerID(tx ReadTx) ([]byte, zkidentity.PublicIdentity, error) {
	fail := func(err error) ([]byte, zkidentity.PublicIdentity, error) {
		return nil, zkidentity.PublicIdentity{}, err
	}

	// serveridentity
	pib64, err := db.idb.Get("", "serveridentity")
	if errors.Is(err, inidb.ErrNotFound) {
		return fail(ErrServerIDEmpty)
	} else if err != nil {
		return fail(err)
	}
	pc64, err := db.idb.Get("", "servercert")
	if errors.Is(err, inidb.ErrNotFound) {
		return fail(ErrServerIDEmpty)
	} else if err != nil {
		return fail(err)
	}

	if err != nil {
		return fail(fmt.Errorf("could not obtain serveridentity record"))
	}
	piJSON, err := base64.StdEncoding.DecodeString(pib64)
	if err != nil {
		return fail(fmt.Errorf("could not decode serveridentity"))
	}

	var spid zkidentity.PublicIdentity
	err = json.Unmarshal(piJSON, &spid)
	if err != nil {
		return fail(fmt.Errorf("could not unmarshal serveridentity"))
	}
	tlsCert, err := base64.StdEncoding.DecodeString(pc64)
	if err != nil {
		return fail(fmt.Errorf("could not decode servercert"))
	}

	return tlsCert, spid, nil
}

func (db *DB) UpdateServerID(tx ReadWriteTx, tlsCert []byte, pid *zkidentity.PublicIdentity) error {
	// save server as our very own
	b, err := json.Marshal(pid)
	if err != nil {
		return fmt.Errorf("Could not marshal server identity: %v", err)
	}
	err = db.idb.Set("", "serveridentity",
		base64.StdEncoding.EncodeToString(b))
	if err != nil {
		return fmt.Errorf("could not insert record serveridentity: %v", err)
	}
	err = db.idb.Set("", "servercert",
		base64.StdEncoding.EncodeToString(tlsCert))
	if err != nil {
		return fmt.Errorf("could not insert record servercert: %v", err)
	}
	err = db.idb.Save()
	if err != nil {
		return fmt.Errorf("could not save server: %v", err)
	}

	return nil
}

func (db *DB) UpdateRatchet(tx ReadWriteTx, r *ratchet.Ratchet, theirID zkidentity.ShortID) error {
	diskState := r.DiskState(31 * 24 * time.Hour)
	jsonState, err := json.Marshal(diskState)
	if err != nil {
		return fmt.Errorf("failed to marshal ratchet: %v", err)
	}

	// save to tempfile
	ids := hex.EncodeToString(theirID[:])
	fullPath := filepath.Join(db.root, inboundDir, ids)

	if err := os.MkdirAll(fullPath, 0o700); err != nil {
		return fmt.Errorf("could not create ratchet dir: %v", err)
	}

	f, err := os.CreateTemp(fullPath, ratchetFilename)
	if err != nil {
		return fmt.Errorf("could not create ratchet file: %v", err)
	}
	if _, err = f.Write(jsonState); err != nil {
		f.Close()
		return fmt.Errorf("failed to write ratchet: %v", err)
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("unable to fsync ratchet data: %v", err)
	}
	if err = f.Close(); err != nil {
		return fmt.Errorf("failed to close temporary ratchet file: %v", err)
	}

	// rename tempfile to actual file
	filename := filepath.Join(fullPath, ratchetFilename)
	if err = os.Rename(f.Name(), filename); err != nil {
		return fmt.Errorf("could not rename ratchet file: %v", err)
	}

	return nil
}

func (db *DB) UpdateAddressBookEntry(tx ReadWriteTx,
	id *zkidentity.PublicIdentity, myResetRV, theirResetRV RawRVID, ignored bool) error {

	// make identity dirs
	ids := hex.EncodeToString(id.Identity[:])
	fullPath := filepath.Join(db.root, inboundDir, ids)
	err := os.MkdirAll(fullPath, 0o700)
	if err != nil {
		return err
	}

	// save identity
	ab := AddressBookEntry{
		ID:           id,
		MyResetRV:    myResetRV,
		TheirResetRV: theirResetRV,
		Ignored:      ignored,
	}
	blob, err := json.Marshal(ab)
	if err != nil {
		return fmt.Errorf("unable to marshal AddressBookEntry: %v", err)
	}
	filename := filepath.Join(fullPath, identityFilename)
	err = os.WriteFile(filename, blob, 0o600)
	if err != nil {
		return fmt.Errorf("write to %v: %v", filename, err)
	}

	return nil
}

func (db *DB) AddressBookEntryExists(tx ReadTx, id UserID) bool {
	fname := filepath.Join(db.root, inboundDir, id.String(),
		identityFilename)
	_, err := os.Stat(fname)
	return err == nil
}

// getBaseABEntry returns the base address book entry, without the ratchet info
// setup.
func (db *DB) getBaseABEntry(id UserID) (*AddressBookEntry, error) {
	filename := filepath.Join(db.root, inboundDir, id.String(),
		identityFilename)
	blob, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("identity file %s: %w", id.String(), ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("unable to read identity %v: %v", filename, err)
	}

	entry := new(AddressBookEntry)
	err = json.Unmarshal(blob, &entry)
	if err != nil {
		return nil, fmt.Errorf("unmarshal public identity %v: %v",
			filename, err)
	}
	if !entry.ID.Verify() {
		return nil, fmt.Errorf("verify public identity failed: %v", entry.ID)
	}
	return entry, nil
}

func (db *DB) GetAddressBookEntry(tx ReadTx, id UserID,
	localID *zkidentity.FullIdentity) (*AddressBookEntry, error) {

	entry, err := db.getBaseABEntry(id)
	if err != nil {
		return nil, err
	}

	// Read Ratchet.
	filename := filepath.Join(db.root, inboundDir, id.String(), ratchetFilename)
	ratchetJSON, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("ReadFile ratchet: %v", err)
	}

	var rs disk.RatchetState
	err = json.Unmarshal(ratchetJSON, &rs)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal ratchet state: %v", err)
	}

	// recreate ratchet
	r := ratchet.New(rand.Reader)
	err = r.Unmarshal(&rs)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal ratchet: %v", err)
	}

	// XXX verify this
	if localID != nil {
		r.MyPrivateKey = &localID.PrivateKey
	}
	r.TheirPublicKey = &entry.ID.Key

	entry.R = r
	return entry, nil
}

// LoadAddressBook returns the full client address book. Note that invalid or
// otherwise incomplete entries do not cause the addressbook loading to fail,
// only diagnostic messages are returned in that case.
func (db *DB) LoadAddressBook(tx ReadTx, localID *zkidentity.FullIdentity) ([]*AddressBookEntry, error) {
	fi, err := os.ReadDir(filepath.Join(db.root, inboundDir))
	if err != nil {
		return nil, err
	}

	res := make([]*AddressBookEntry, 0, len(fi))
	id := &UserID{}

	for _, v := range fi {
		// Read ID.
		if err := id.FromString(v.Name()); err != nil {
			db.log.Warnf("Unable to identify addressbook entry %s: %v",
				v.Name(), err)
			continue
		}

		entry, err := db.GetAddressBookEntry(tx, *id, localID)
		if err != nil {
			db.log.Warnf("Unable to load addressbook entry %s: %v",
				id, err)
			continue
		}
		res = append(res, entry)
	}

	return res, nil
}

// StoreTransResetHalfKX stores the given ratchet as a half-ratchet used for a
// transitive reset call with the given user.
func (db *DB) StoreTransResetHalfKX(tx ReadWriteTx, r *ratchet.Ratchet, theirID zkidentity.ShortID) error {
	state := r.DiskState(31 * 24 * time.Hour)
	jsonState, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal ratchet: %v", err)
	}

	ids := theirID.String()
	dir := filepath.Join(db.root, inboundDir, ids)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("could not create trans reset dir: %v", err)
	}

	filename := filepath.Join(dir, transResetFile)
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	if _, err = f.Write(jsonState); err != nil {
		f.Close()
		return fmt.Errorf("could not write ratchet: %v", err)
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("unable to fsync ratchet data: %v", err)
	}
	return f.Close()
}

// LoadTransResetHalfKX returns the existing trans reset half kx.
func (db *DB) LoadTransResetHalfKX(tx ReadTx, id UserID,
	localID *zkidentity.FullIdentity) (*ratchet.Ratchet, error) {

	// Read Ratchet.
	dir := filepath.Join(db.root, inboundDir, id.String())
	filename := filepath.Join(dir, transResetFile)
	ratchetJSON, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("ReadFile ratchet: %v", err)
	}

	var rs disk.RatchetState
	err = json.Unmarshal(ratchetJSON, &rs)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal RatchetState: %v", err)
	}

	// recreate ratchet
	r := ratchet.New(rand.Reader)
	err = r.Unmarshal(&rs)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal Ratchet")
	}

	entry, err := db.getBaseABEntry(id)
	if err != nil {
		return nil, err
	}

	// XXX verify this
	if localID != nil {
		r.MyPrivateKey = &localID.PrivateKey
	}
	r.TheirPublicKey = &entry.ID.Key
	return r, nil
}

// DeleteTransResetHalfKX removes the trans reset half-ratchet associated with
// the given ID.
//
// It returns the contents of the trans reset ratchet file.
func (db *DB) DeleteTransResetHalfKX(tx ReadWriteTx, id UserID) error {
	dir := filepath.Join(db.root, inboundDir, id.String())
	filename := filepath.Join(dir, transResetFile)
	return os.Remove(filename)
}

func (db *DB) readLogMsg(logFname string, pageSize, pageNum int) ([]PMLogEntry, error) {
	if db.cfg.MsgsRoot == "" {
		return nil, nil
	}

	filename := filepath.Join(db.cfg.MsgsRoot, logFname)
	f, err := os.Open(filename)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// TODO: instead of reading the entire log, track the total nb of
	// messages read and only start creating the LogEntry elements once the
	// target page is read.
	//
	// TODO: use a streaming regexp impl instead of reading string lines.
	loggedMessages := make([]PMLogEntry, 0)
	reader := bufio.NewReader(f)
	prevLine := ""
	prevLineTimestamp := int64(0)
	prevName := ""
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			// Log any left over message if we're at the end
			if prevLine != "" && prevName != "" && prevLineTimestamp != 0 {
				loggedMessages = append(loggedMessages, PMLogEntry{Message: prevLine,
					From: prevName, Timestamp: prevLineTimestamp})
			}
			break
		}
		if len(line) == 0 {
			// Should not happen, but guard against index out of
			// range.
			break
		}

		// Drop newline delimiter.
		line = line[:len(line)-1]

		matches := logLineRegexp.FindStringSubmatchIndex(line)
		if len(matches) != 6 {
			// No new time to parse at the front of the line so just add the
			// message to the previous line, then go to the next line.
			prevLine += "\n" + line
			continue
		}

		// Try to read timestamp
		strTimestamp := line[matches[2]:matches[3]]
		t, err := time.ParseInLocation("2006-01-02T15:04:05", strTimestamp, time.Local)
		if err != nil {
			// No new time to parse at the front of the line so just add the
			// message to the previous line, then go to the next line.
			prevLine += "\n" + line
			continue
		}

		// This means there was a new timestamp in the current line so
		if prevLine != "" && prevName != "" && prevLineTimestamp != 0 {
			loggedMessages = append(loggedMessages, PMLogEntry{Message: prevLine,
				From: prevName, Timestamp: prevLineTimestamp})
		}

		// This surely means there is a new log line if there is a parsable timestamp at the front.
		name := line[matches[4]:matches[5]]
		message := ""
		if len(name) > 0 && name[0] == '<' {
			name = name[1 : len(name)-1]
			if len(line) > matches[5]+1 {
				message = line[matches[5]+1:]
			}
		} else if name == "*" {
			// Not a message to pass through, so reset prev info and move on.
			prevLine = ""
			prevName = ""
			prevLineTimestamp = 0
			continue
		}

		prevLine = message
		prevName = name
		prevLineTimestamp = t.Unix()
	}

	// Return only the requested page/pageNum
	pageEnd := len(loggedMessages) - pageSize*pageNum
	pageStart := pageEnd - pageSize
	if pageEnd < 0 || pageEnd > len(loggedMessages) {
		pageEnd = len(loggedMessages)
	}
	if pageStart < 0 {
		pageStart = 0
	}
	return loggedMessages[pageStart:pageEnd], nil
}

func (db *DB) logMsg(logFname string, internal bool, from, msg string, ts time.Time) error {
	if db.cfg.MsgsRoot == "" {
		return nil
	}

	// Escape lines that match the logLineRegexp, so that when loading the
	// message back, they won't be erroneously detected as log messages.
	matches := logLineRegexp.FindAllStringSubmatchIndex(msg, -1)
	if len(matches) > 0 {
		b := bytes.NewBuffer(nil)
		b.Grow(len([]byte(msg)) + len(matches)) // Each match adds 1 byte
		lastEnd := 0
		for _, match := range matches {
			// Copy until start of match.
			b.WriteString(msg[lastEnd:match[0]])

			// Add a space to escape (unless the match is the start
			// of the string, in which case the prefix added below
			// will be sufficient to escape it).
			if match[0] != 0 {
				b.WriteRune(' ')
			}

			// Add the log line prefix.
			b.WriteString(msg[match[0]:match[1]])
			lastEnd = match[1]
		}

		// Copy rest of string (end of last match to end of string)
		b.WriteString(msg[lastEnd:])
		msg = b.String()
	}

	filename := filepath.Join(db.cfg.MsgsRoot, logFname)
	f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	b := new(bytes.Buffer)
	lastMsgTs, ok := db.lastMsgTS[logFname]
	if !ok {
		b.WriteString(ts.Format("2006-01-02T15:04:05 "))
		b.WriteString(fmt.Sprintf("* Conversation started %s", ts.Format("2006-01-02")))
		b.WriteRune('\n')
	} else if lastMsgTs.Day() != ts.Day() {
		b.WriteString(ts.Format("2006-01-02T15:04:05 "))
		b.WriteString(fmt.Sprintf("* Day Changed to %s", ts.Format("2006-01-02")))
		b.WriteRune('\n')
	}
	db.lastMsgTS[logFname] = ts

	b.WriteString(ts.Format("2006-01-02T15:04:05 "))

	if internal {
		b.WriteString("* ")
	} else {
		b.WriteString("<")
		b.WriteString(strescape.Nick(from))
		b.WriteString("> ")
	}

	b.WriteString(strescape.Content(msg))
	b.WriteRune('\n')

	_, err = f.Write(b.Bytes())
	if err != nil {
		return err
	}

	if err := f.Sync(); err != nil {
		return err
	}
	return nil
}

func (db *DB) IsBlocked(tx ReadTx, id UserID) bool {
	_, exists := db.blockedIDs[id.String()]
	return exists
}

// RemoveUser deletes the user from the database
func (db *DB) RemoveUser(tx ReadWriteTx, id UserID, block bool) error {
	if block {
		db.blockedIDs[id.String()] = time.Now()

		filename := filepath.Join(db.root, blockedUsersFile)
		if err := db.saveJsonFile(filename, db.blockedIDs); err != nil {
			return err
		}
	}
	dir := filepath.Join(db.root, inboundDir, id.String())
	return os.RemoveAll(dir)
}

// LogPM logs a PM message from the given user.
func (db *DB) LogPM(tx ReadWriteTx, uid UserID, internal bool, from, msg string, ts time.Time) error {
	entry, err := db.getBaseABEntry(uid)
	if err != nil {
		return err
	}

	nick := entry.ID.Nick
	logFname := fmt.Sprintf("%s.%s.log", escapeNickForFname(nick), uid)
	return db.logMsg(logFname, internal, from, msg, ts)
}

// LogGCMsg logs a GC message sent in the given GC.
func (db *DB) LogGCMsg(tx ReadWriteTx, gcName string, gcID zkidentity.ShortID,
	internal bool, from, msg string, ts time.Time) error {

	logFname := fmt.Sprintf("groupchat.%s.%s.log", escapeNickForFname(gcName), gcID)
	return db.logMsg(logFname, internal, from, msg, ts)
}

// ReadLogPM reads the log of PM messages from the given user.
func (db *DB) ReadLogPM(tx ReadTx, uid UserID, page, pageNum int) ([]PMLogEntry, error) {
	entry, err := db.getBaseABEntry(uid)
	if err != nil {
		return nil, err
	}

	nick := entry.ID.Nick
	logFname := fmt.Sprintf("%s.%s.log", escapeNickForFname(nick), uid)
	return db.readLogMsg(logFname, page, pageNum)
}

// ReadLogGCMsg reads the log a GC messages sent in the given GC.
func (db *DB) ReadLogGCMsg(tx ReadTx, gcName string, gcID zkidentity.ShortID, page, pageNum int) ([]PMLogEntry, error) {

	logFname := fmt.Sprintf("groupchat.%s.%s.log", escapeNickForFname(gcName), gcID)
	return db.readLogMsg(logFname, page, pageNum)
}

// ReplaceLastConnDate replaces the last connection date of the local client to
// the server with the specified one. Returns the old connection date.
func (db *DB) ReplaceLastConnDate(tx ReadWriteTx, date time.Time) (time.Time, error) {
	fname := filepath.Join(db.root, lastConnDateFile)
	var oldDate time.Time
	err := db.readJsonFile(fname, &oldDate)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return oldDate, err
	}

	err = db.saveJsonFile(fname, date)
	return oldDate, err
}
