package clientdb

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/mitchellh/go-homedir"
)

const subscriptionVersion = 1

type subscription struct {
	Version   uint64 `json:"version"`
	From      UserID `json:"from"` // Who sent update
	Timestamp int64  `json:"timestamp"`
}

// PostSummFromMetadata creates the basic post summary info from the given post
// metadata.
func PostSummFromMetadata(post *rpc.PostMetadata, from UserID) PostSummary {
	var authorID UserID
	if id, ok := post.Attributes[rpc.RMPStatusFrom]; ok {
		_ = authorID.FromString(id) // Ok to ingore error
	}
	var authorNick string
	if nick, ok := post.Attributes[rpc.RMPStatusFrom]; ok {
		authorNick = nick
	}

	var pid PostID
	_ = pid.FromString(post.Attributes[rpc.RMPIdentifier])

	const maxTitleLen = 255
	title := clientintf.PostTitle(post)
	if len(title) > maxTitleLen {
		title = title[:maxTitleLen]
	}
	return PostSummary{
		ID:         pid,
		From:       from,
		AuthorID:   authorID,
		AuthorNick: authorNick,
		Title:      title,
	}
}

// SubscribeToPosts registers the given remote user as subscribed to posts of
// the local user.
func (db *DB) SubscribeToPosts(tx ReadWriteTx, user UserID) error {
	dir := filepath.Join(db.root, postsDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	filename := filepath.Join(dir, postsSubscribers)

	f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		defer f.Close()

		d := json.NewDecoder(f)
		for {
			var s subscription
			err = d.Decode(&s)
			if err != nil {
				if err == io.EOF {
					break
				}
				return err
			}
			if s.From == user {
				return ErrAlreadySubscribed
			}
		}
	}

	// If we get here we are at the end of the file
	s := subscription{
		Version:   subscriptionVersion,
		From:      user,
		Timestamp: time.Now().Unix(),
	}
	e := json.NewEncoder(f)
	err = e.Encode(s)
	if err != nil {
		return err
	}

	return nil
}

// UnsubscribeToPosts removes the subscription of the given user from the posts
// of the local user.
func (db *DB) UnsubscribeToPosts(tx ReadWriteTx, user UserID) error {
	dir := filepath.Join(db.root, postsDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	filename := filepath.Join(dir, postsSubscribers)

	f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	d := json.NewDecoder(f)
	unsubscribed := false
	ss := make([]subscription, 0, 16)
	for {
		var s subscription
		err = d.Decode(&s)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if s.From == user {
			// Skip, thus not saving identity back
			unsubscribed = true
			continue
		}
		ss = append(ss, s)
	}
	if !unsubscribed {
		return ErrNotSubscribed
	}

	// If we get here we can write the file back
	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	err = f.Truncate(0)
	if err != nil {
		return err
	}
	e := json.NewEncoder(f)
	for k := range ss {
		err = e.Encode(ss[k])
		if err != nil {
			return err
		}
	}

	return nil
}

// ListSubscribers lists all users that are subscribed to our posts.
func (db *DB) ListPostSubscribers(tx ReadTx) ([]UserID, error) {
	dir := filepath.Join(db.root, postsDir)
	filename := filepath.Join(dir, postsSubscribers)

	f, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	d := json.NewDecoder(f)
	subs := make([]UserID, 0, 16)
	for {
		var s subscription
		err = d.Decode(&s)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		subs = append(subs, s.From)
	}

	return subs, nil
}

// IsPostSubscriber returns whether the given uid is a subscriber to the local
// client's posts.
func (db *DB) IsPostSubscriber(tx ReadTx, uid UserID) (bool, error) {
	dir := filepath.Join(db.root, postsDir)
	filename := filepath.Join(dir, postsSubscribers)

	f, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer f.Close()

	d := json.NewDecoder(f)
	for {
		var s subscription
		err = d.Decode(&s)
		if err != nil {
			if err == io.EOF {
				break
			}
			return false, err
		}
		if s.From == uid {
			return true, nil
		}
	}

	return false, nil
}

// CreatePost creates the given post in the local DB.
//
// Note that the optional file specified in fname is **copied** to the post.
func (db *DB) CreatePost(tx ReadWriteTx, post, descr string, fname string,
	extraAttrs map[string]string, me *zkidentity.FullIdentity) (PostSummary, rpc.PostMetadata, error) {

	var pid PostID
	var summ PostSummary
	var p rpc.PostMetadata
	if post == "" {
		return summ, p, errors.New("post cannot be empty")
	}

	// Read optional file.
	var fblob string
	if fname != "" {
		filename, err := homedir.Expand(fname)
		if err != nil {
			return summ, p, err
		}
		blob, err := os.ReadFile(filename)
		if err != nil {
			return summ, p, err
		}
		fblob = base64.StdEncoding.EncodeToString(blob)
	}

	dir := filepath.Join(db.root, postsDir, me.Public.Identity.String())
	if err := os.MkdirAll(dir, 0700); err != nil {
		return summ, p, err
	}

	// Create post metadata.
	attrs := make(map[string]string, 4+len(extraAttrs))
	for k, v := range extraAttrs {
		attrs[k] = v
	}
	attrs[rpc.RMPStatusFrom] = me.Public.Identity.String()
	attrs[rpc.RMPFromNick] = me.Public.Nick
	attrs[rpc.RMPMain] = post
	if descr != "" {
		attrs[rpc.RMPDescription] = descr
	}
	if fblob != "" {
		attrs[rpc.RMPAttachment] = fblob
	}

	// Sign it.
	p = rpc.PostMetadata{
		Version:    rpc.PostMetadataVersion,
		Attributes: attrs,
	}
	pmHash := p.Hash()
	copy(pid[:], pmHash[:])
	signature := me.SignMessage(pmHash[:])
	attrs[rpc.RMPSignature] = hex.EncodeToString(signature[:])
	attrs[rpc.RMPIdentifier] = pid.String()

	// Save the post.
	postFname := filepath.Join(dir, pid.String())
	f, err := os.Create(postFname)
	if err != nil {
		return summ, p, err
	}
	defer f.Close()
	w := json.NewEncoder(f)
	err = w.Encode(p)
	if err != nil {
		return summ, p, err
	}

	finfo, err := f.Stat()
	if err != nil {
		return summ, p, err
	}

	summ = PostSummFromMetadata(&p, me.Public.Identity)
	summ.Date = finfo.ModTime()
	return summ, p, nil
}

// verifyPostStatusUpdate verifies the status update sent by from on the given
// post is valid. This can be called for both local and received posts.
func (db *DB) verifyPostStatusUpdate(statusFname string, from UserID, pid PostID,
	pms *rpc.PostMetadataStatus) error {

	attr := pms.Attributes

	// Validate the individual status update.
	for k, v := range attr {
		switch k {
		case rpc.RMPSHeart:
			switch v {
			case rpc.RMPSHeartYes:
			case rpc.RMPSHeartNo:
			default:
				return fmt.Errorf("%w: unknown heart value %q",
					ErrPostStatusValidation, v)
			}

		case rpc.RMPSComment:
			if strings.TrimSpace(v) == "" {
				return fmt.Errorf("%w: empty comment", ErrPostStatusValidation)
			}

		case rpc.RMPSignature, rpc.RMPNonce, rpc.RMPFromNick, rpc.RMPTimestamp:
			// Ignore.

		case rpc.RMPVersion:
			if _, err := strconv.Atoi(v); err != nil {
				return fmt.Errorf("%w: %s is not a number: %v",
					ErrPostStatusValidation, rpc.RMPVersion, err)
			}

		case rpc.RMPIdentifier, rpc.RMPStatusFrom, rpc.RMPParent:
			// Ensure a valid ID.
			var id clientintf.ID
			if err := id.FromString(v); err != nil {
				return fmt.Errorf("%w: %s is not a valid id: %v",
					ErrPostStatusValidation, k, err)
			}

		default:
			return fmt.Errorf("%w: unknown status update %q",
				ErrPostStatusValidation, k)
		}
	}

	// Validate this status update doesn't conflict with an existing one
	// from the same user.
	//
	// TODO: this is slow as it involves loading the entire status update
	// file. Please improve.
	f, err := os.Open(statusFname)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	fromStr := from.String()
	_, hearting := attr[rpc.RMPSHeart]
	_, commenting := attr[rpc.RMPSComment]

	var lastHeart string
	var lastComment string
	hash := pms.Hash()

	d := json.NewDecoder(f)
	for {
		var old rpc.PostMetadataStatus
		err := d.Decode(&old)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		// If this isn't the sender of the status update, skip.
		if old.From != fromStr {
			continue
		}

		if old.Hash() == hash {
			return ErrDuplicatePostStatus
		}

		if oldv, ok := old.Attributes[rpc.RMPSHeart]; ok && hearting {
			lastHeart = oldv
		}
		if oldc, ok := old.Attributes[rpc.RMPSComment]; ok && commenting {
			lastComment = oldc
		}
	}

	if hearting && lastHeart == attr[rpc.RMPSHeart] {
		return fmt.Errorf("%w: cannot send the same heart value twice", ErrPostStatusValidation)
	}
	if commenting && lastComment == attr[rpc.RMPSComment] {
		return fmt.Errorf("%w: cannot send the exact same comment twice", ErrPostStatusValidation)
	}

	return nil
}

// AddPostStatus adds a status update to a post. postFrom is the author of the
// post, while statusFrom is who sent the status update.
func (db *DB) AddPostStatus(tx ReadWriteTx, postFrom, statusFrom UserID, pid PostID,
	pms *rpc.PostMetadataStatus) error {

	// Ensure post exists.
	dir := filepath.Join(db.root, postsDir, postFrom.String())
	postFname := filepath.Join(dir, pid.String())
	if _, err := os.Stat(postFname); err != nil {
		return err
	}

	// Verify this status update is valid when coming from the given user.
	statusFname := filepath.Join(dir, pid.String()+postsStatusExt)
	if err := db.verifyPostStatusUpdate(statusFname, statusFrom, pid, pms); err != nil {
		return err
	}

	// Append to the status update of the post.
	f, err := os.OpenFile(statusFname, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	e := json.NewEncoder(f)
	err = e.Encode(pms)
	if err != nil {
		return err
	}
	return nil
}

func (db *DB) SaveReceivedPost(tx ReadWriteTx, from UserID, p rpc.PostMetadata) (PostID, PostSummary, error) {
	var pid PostID
	var summ PostSummary

	if s, ok := p.Attributes[rpc.RMPIdentifier]; !ok {
		return pid, summ, fmt.Errorf("post does not have an identifier")
	} else {
		err := pid.FromString(s)
		if err != nil {
			return pid, summ, err
		}
	}

	dir := filepath.Join(db.root, postsDir, from.String())
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return pid, summ, fmt.Errorf("unable to make received posts dir: %v", err)
	}
	fname := filepath.Join(dir, pid.String())
	f, err := os.Create(fname)
	if err != nil {
		return pid, summ, err
	}
	defer f.Close()
	w := json.NewEncoder(f)
	if err := w.Encode(p); err != nil {
		return pid, summ, err
	}

	finfo, err := f.Stat()
	if err != nil {
		return pid, summ, err
	}

	summ = PostSummFromMetadata(&p, from)
	summ.Date = finfo.ModTime()
	return pid, summ, nil
}

// AddPostStatusUpdate saves the specified post metadata as a status update for
// the given post.
func (db *DB) AddPostStatusUpdate(tx ReadWriteTx, from UserID, p rpc.PostMetadata) (UserID, rpc.PostMetadataStatus, error) {
	var pid clientintf.PostID
	var statusFrom UserID

	fail := func(err error) (UserID, rpc.PostMetadataStatus, error) {
		return UserID{}, rpc.PostMetadataStatus{}, err
	}

	if version, ok := p.Attributes[rpc.RMPVersion]; !ok {
		return fail(fmt.Errorf("post status does not have a version field"))
	} else if version != strconv.Itoa(rpc.PostMetadataStatusVersion) {
		return fail(fmt.Errorf("cannot accept status updates with version different then %d",
			rpc.PostMetadataStatusVersion))
	}

	if s, ok := p.Attributes[rpc.RMPIdentifier]; !ok {
		return fail(fmt.Errorf("post status does not have an identifier"))
	} else if err := pid.FromString(s); err != nil {
		return fail(err)
	}

	// Ensure it has a "from" field.
	if s, ok := p.Attributes[rpc.RMPStatusFrom]; !ok {
		return fail(fmt.Errorf("post status does not have a from field"))
	} else if err := statusFrom.FromString(s); err != nil {
		return fail(err)
	}

	update := rpc.PostMetadataStatus{
		Version:    rpc.PostMetadataStatusVersion,
		From:       statusFrom.String(),
		Link:       pid.String(),
		Attributes: p.Attributes,
	}

	// Verify this status update is valid when coming from the given user.
	statusFname := filepath.Join(db.root, postsDir, from.String(), pid.String()+postsStatusExt)
	if err := db.verifyPostStatusUpdate(statusFname, statusFrom, pid, &update); err != nil {
		return fail(err)
	}

	// Append to the status update of the post.
	f, err := os.OpenFile(statusFname, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return fail(err)
	}
	defer f.Close()
	e := json.NewEncoder(f)
	err = e.Encode(update)
	if err != nil {
		return fail(err)
	}

	return statusFrom, update, nil
}

func (db *DB) readPost(fname string) (*rpc.PostMetadata, error) {
	data, err := os.ReadFile(fname)
	if err != nil {
		return nil, err
	}
	pm := new(rpc.PostMetadata)
	err = json.Unmarshal(data, pm)
	if err != nil {
		return nil, err
	}
	return pm, err
}

// ListPosts returns a summary of all received posts.
func (db *DB) ListPosts(tx ReadTx) ([]PostSummary, error) {
	rootDir := filepath.Join(db.root, postsDir)
	authorDirs, err := os.ReadDir(rootDir)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	var res []PostSummary
	for _, dir := range authorDirs {
		if !dir.IsDir() {
			continue
		}

		fullDir := filepath.Join(rootDir, dir.Name())
		from := new(UserID)
		if err := from.FromString(dir.Name()); err != nil {
			db.log.Warnf("Entry %s is not a UserID: %v",
				fullDir, err)
			continue
		}

		postDirs, err := os.ReadDir(fullDir)
		if err != nil {
			return nil, err
		}
		for _, postFile := range postDirs {
			if postFile.IsDir() {
				continue
			}

			// Skip if it's the status update file.
			if strings.HasSuffix(postFile.Name(), postsStatusExt) {
				continue
			}

			fullPath := filepath.Join(fullDir, postFile.Name())
			pid := new(PostID)
			if err := pid.FromString(postFile.Name()); err != nil {
				db.log.Warnf("Entry %s is not a PostID: %v",
					fullPath, err)
				continue
			}

			finfo, err := postFile.Info()
			if err != nil {
				db.log.Warnf("Unable to get FINFO of %s: %v",
					fullPath, err)
				continue
			}

			post, err := db.readPost(fullPath)
			if err != nil {
				db.log.Warnf("Unable to read post %s: %v", fullPath, err)
				continue
			}

			// Check time of last status.
			var lastStatusTime time.Time
			statusFname := fullPath + postsStatusExt
			if finfo, err := os.Stat(statusFname); err == nil {
				lastStatusTime = finfo.ModTime()
			}

			summ := PostSummFromMetadata(post, *from)
			summ.Date = finfo.ModTime()
			summ.LastStatusTS = lastStatusTime
			res = append(res, summ)
		}
	}

	return res, nil
}

// ListUserPosts lists all posts made by the given user.
func (db *DB) ListUserPosts(tx ReadTx, from UserID) ([]rpc.PostMetadata, error) {
	rootDir := filepath.Join(db.root, postsDir)
	authorDir := filepath.Join(rootDir, from.String())

	postFiles, err := os.ReadDir(authorDir)
	if err != nil {
		return nil, err
	}
	var res []rpc.PostMetadata
	for _, postFile := range postFiles {
		if postFile.IsDir() {
			continue
		}

		// Skip if it's the status update file.
		if strings.HasSuffix(postFile.Name(), postsStatusExt) {
			continue
		}

		fullPath := filepath.Join(authorDir, postFile.Name())
		pid := new(PostID)
		if err := pid.FromString(postFile.Name()); err != nil {
			db.log.Warnf("Entry %s is not a PostID: %v",
				fullPath, err)
			continue
		}

		post, err := db.readPost(fullPath)
		if err != nil {
			db.log.Warnf("Unable to read post %s: %v", fullPath, err)
			continue
		}
		res = append(res, *post)
	}

	return res, nil
}

// ReadPost returns the post data for the given user/post.
func (db *DB) ReadPost(tx ReadTx, from UserID, post PostID) (rpc.PostMetadata, error) {
	filepath := filepath.Join(db.root, postsDir, from.String(),
		post.String())
	pm, err := db.readPost(filepath)
	if os.IsNotExist(err) {
		err = fmt.Errorf("post %s: %w", post, ErrNotFound)
	}
	if err != nil {
		return rpc.PostMetadata{}, err
	}
	return *pm, nil
}

// PostExists verifies whether the given received post already exists.
func (db *DB) PostExists(tx ReadTx, from UserID, post PostID) (bool, error) {
	filepath := filepath.Join(db.root, postsDir, from.String(),
		post.String())
	_, err := os.Stat(filepath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// ListPostRelayers lists everyone that has relayed (to us) the specified post.
func (db *DB) ListPostRelayers(tx ReadTx, post PostID) ([]UserID, error) {
	pattern := filepath.Join(db.root, postsDir, "*", post.String())
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	var res []UserID
	for _, f := range files {
		idStr := filepath.Base(filepath.Dir(f))
		var fromID UserID
		if err := fromID.FromString(idStr); err != nil {
			db.log.Debugf("Not an ID while listing relayers: %s: %v",
				idStr, err)
			continue
		}

		res = append(res, fromID)
	}
	return res, nil
}

// ListPostStatusUpdates lists the status updates of the currently received
// posts.
func (db *DB) ListPostStatusUpdates(tx ReadTx, from UserID,
	post PostID) ([]rpc.PostMetadataStatus, error) {

	statusFname := filepath.Join(db.root, postsDir, from.String(),
		post.String()+postsStatusExt)
	f, err := os.Open(statusFname)
	if err != nil && os.IsNotExist(err) {
		return nil, nil // Empty list of status updates.
	} else if err != nil {
		return nil, err
	}
	defer f.Close()

	d := json.NewDecoder(f)
	var res []rpc.PostMetadataStatus
	for {
		var pms rpc.PostMetadataStatus
		err := d.Decode(&pms)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		res = append(res, pms)
	}
	return res, nil
}

func (db *DB) replacePostSubscription(to UserID, add bool) error {
	fname := filepath.Join(db.root, postsDir, postsSubscriptions)
	if _, err := os.Stat(fname); os.IsNotExist(err) {
		// Subscriptions file does not exist, so do the quick action.
		if !add {
			return nil // Nothing to do.
		}

		// Add the subscription
		sub := PostSubscription{To: to, Date: time.Now()}
		f, err := os.OpenFile(fname, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			return err
		}
		defer f.Close()
		enc := json.NewEncoder(f)
		if err := enc.Encode(sub); err != nil {
			return err
		}
		return nil
	}

	// Subscription file exists. Create a new one and copy over contents.

	// Cleanup changes depending on which actions complete from now on, so
	// close over a modifiable cleanup function.
	cleanup := func() {}
	defer func() { cleanup() }()

	// Create new file and clean it up by deleting it as well.
	newFname := filepath.Join(db.root, postsDir, "."+postsSubscriptions+".new")
	newf, err := os.OpenFile(newFname, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	cleanup = func() {
		newf.Close()
		os.Remove(newFname)
	}

	// Open old file for reading.
	oldf, err := os.Open(fname)
	if err != nil {
		return err
	}
	cleanup = func() {
		oldf.Close()
		newf.Close()
		os.Remove(newFname)
	}

	// Start reading from the old file until we find the entry we want to
	// replace (if it exists).
	dec := json.NewDecoder(oldf)
	enc := json.NewEncoder(newf)
	var sub PostSubscription
	for err = dec.Decode(&sub); err == nil; err = dec.Decode(&sub) {
		if sub.To == to {
			// Found it! Skip this entry.
			continue
		}
		if eerr := enc.Encode(&sub); eerr != nil { // Rewrite old entry
			err = eerr
			break
		}
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}

	// Write (or skip) the target entry.
	if add {
		sub := PostSubscription{To: to, Date: time.Now()}
		if err = enc.Encode(sub); err != nil {
			return err
		}
	}

	if err := newf.Sync(); err != nil {
		return err
	}

	// Close both files.
	oldf.Close()
	newf.Close()
	cleanup = func() {
		os.Remove(newFname)
	}

	// Rename new file to old file.
	if err := os.Rename(newFname, fname); err != nil {
		return err
	}
	cleanup = func() {}

	// All done!
	return nil
}

// StorePostSubscription stores that the local user has subscribed to the posts
// of the given user.
func (db *DB) StorePostSubscription(tx ReadWriteTx, to UserID) error {
	dir := filepath.Join(db.root, postsDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	return db.replacePostSubscription(to, true)
}

// StorePostUnsubscription stores that the local user has unsubscribed to the
// posts of the given remote user.
func (db *DB) StorePostUnsubscription(tx ReadWriteTx, to UserID) error {
	dir := filepath.Join(db.root, postsDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	return db.replacePostSubscription(to, false)
}

// ListPostSubscriptions returns a list of posts this user has subscribed to.
func (db *DB) ListPostSubscriptions(tx ReadTx) ([]PostSubscription, error) {
	fname := filepath.Join(db.root, postsDir, postsSubscriptions)

	// Open old file for reading.
	f, err := os.Open(fname)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	var sub PostSubscription
	var res []PostSubscription

	// Iterate file.
	for err = dec.Decode(&sub); err == nil; err = dec.Decode(&sub) {
		res = append(res, sub)
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}

	return res, nil
}

// IsPostSubscription returns true if the local client is subscribed to the posts
// of the given user.
func (db *DB) IsPostSubscription(tx ReadTx, uid UserID) (bool, error) {
	fname := filepath.Join(db.root, postsDir, postsSubscriptions)

	// Open old file for reading.
	f, err := os.Open(fname)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	var sub PostSubscription

	// Iterate file.
	for err = dec.Decode(&sub); err == nil; err = dec.Decode(&sub) {
		if sub.To == uid {
			return true, nil
		}
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}

	return false, nil
}

// RelayPost marks the post as being relayed by the local user. It returns a
// copy of the post contents. The bool return value indicates whether this is
// the first time this is relayed.
func (db *DB) RelayPost(tx ReadWriteTx, from UserID, pid PostID,
	me *zkidentity.FullIdentity) (rpc.PostMetadata, bool, error) {

	dstFilename := filepath.Join(db.root, postsDir, me.Public.Identity.String(),
		pid.String())

	firstTime := !fileExists(dstFilename)
	if firstTime {
		if err := os.MkdirAll(filepath.Dir(dstFilename), 0o700); err != nil {
			return rpc.PostMetadata{}, firstTime, err
		}

		// Relayed post does not exist. Copy from source to dest.
		srcFilename := filepath.Join(db.root, postsDir, from.String(),
			pid.String())
		if err := copyFile(srcFilename, dstFilename); err != nil {
			return rpc.PostMetadata{}, firstTime, err
		}
	}

	pm, err := db.readPost(dstFilename)
	if os.IsNotExist(err) {
		err = fmt.Errorf("post %s: %w", pid, ErrNotFound)
	}
	if err != nil {
		return rpc.PostMetadata{}, firstTime, err
	}
	return *pm, firstTime, nil
}
