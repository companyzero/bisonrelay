package clientdb

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/rpc"
)

// NewPagesSession starts a new session for fetching related pages.
func (db *DB) NewPagesSession(tx ReadWriteTx) (clientintf.PagesSessionID, error) {
	baseDir := filepath.Join(db.root, pageSessionsDir)
	last, err := pageSessDirPattern.Last(baseDir)
	if err != nil {
		return 0, err
	}
	id := clientintf.PagesSessionID(last.ID + 1)
	sessDir := filepath.Join(baseDir, pageSessDirPattern.FilenameFor(uint64(id)))
	err = os.MkdirAll(sessDir, 0o700)
	return id, err
}

// StoreResourceRequest stores the specified requested resource. This generates
// a random tag for the request, which is set in the passed request Tag field.
func (db *DB) StoreResourceRequest(tx ReadWriteTx, uid UserID,
	sess, parentPage clientintf.PagesSessionID, req *rpc.RMFetchResource,
	asyncTargetID string) error {

	userReqsDir := filepath.Join(db.root, inboundDir, uid.String(), reqResourcesDir)

	// Generate an unused tag for this user.
	tag := rpc.ResourceTag(db.mustRandomUint64())
	filename := path.Join(userReqsDir, tag.String())
	for fileExists(filename) {
		tag = rpc.ResourceTag(db.mustRandomUint64())
		filename = path.Join(userReqsDir, tag.String())
	}
	req.Tag = tag

	rr := ResourceRequest{
		UID:           uid,
		SesssionID:    sess,
		ParentPage:    parentPage,
		Request:       *req,
		Timestamp:     time.Now(),
		AsyncTargetID: asyncTargetID,
	}
	if err := db.saveJsonFile(filename, rr); err != nil {
		return err
	}

	// Update the overview of the session with the new request.
	overv, err := db.readResourcesSessionOverview(sess)
	if err != nil {
		return err
	}
	overv.appendRequest(uid, tag, asyncTargetID)

	if err := db.saveResourcesSessionOverview(sess, &overv); err != nil {
		return err
	}
	return nil
}

// readResourceRequest returns the resource request corresponding to the
// specified tag.
func (db *DB) readResourceRequest(tx ReadTx, uid UserID,
	tag rpc.ResourceTag) (ResourceRequest, error) {

	dir := filepath.Join(db.root, inboundDir, uid.String(), reqResourcesDir)
	filename := path.Join(dir, tag.String())
	var res ResourceRequest
	err := db.readJsonFile(filename, &res)
	return res, err
}

// removeResourceRequest deletes the request with the corresponding tag.
func (db *DB) removeResourceRequest(tx ReadWriteTx, uid UserID, tag rpc.ResourceTag) error {
	dir := filepath.Join(db.root, inboundDir, uid.String(), reqResourcesDir)
	filename := path.Join(dir, tag.String())
	return removeIfExists(filename)
}

// readResourcesSessionOverview reads the overview data of a pages session. It
// returns a new, empty overview if one does not exist for the session.
func (db *DB) readResourcesSessionOverview(sessID clientintf.PagesSessionID) (PageSessionOverview, error) {
	sessionDir := filepath.Join(db.root, pageSessionsDir, sessID.String())
	fname := filepath.Join(sessionDir, pageSessionOverviewFile)
	var res PageSessionOverview
	err := db.readJsonFile(fname, &res)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return res, err
	}
	return res, nil
}

func (db *DB) saveResourcesSessionOverview(sessID clientintf.PagesSessionID, overview *PageSessionOverview) error {
	sessionDir := filepath.Join(db.root, pageSessionsDir, sessID.String())
	fname := filepath.Join(sessionDir, pageSessionOverviewFile)
	return db.saveJsonFile(fname, overview)
}

// StoreFetchedResource removes an existing request sent to the specified
// user with the tag, and stores the resulting fetched response.
func (db *DB) StoreFetchedResource(tx ReadWriteTx, uid UserID, tag rpc.ResourceTag,
	reply rpc.RMFetchResourceReply) (FetchedResource, PageSessionOverview, error) {

	var fr FetchedResource
	var sess PageSessionOverview

	// Double check request exists.
	req, err := db.readResourceRequest(tx, uid, tag)
	if err != nil {
		return fr, sess, err
	}

	sessionDir := filepath.Join(db.root, pageSessionsDir, pageSessDirPattern.FilenameFor(uint64(req.SesssionID)))
	last, err := pageFnamePattern.Last(sessionDir)
	if err != nil {
		return fr, sess, err
	}
	pageID := last.ID + 1

	// Store the fetched resource.
	fr = FetchedResource{
		UID:           uid,
		SessionID:     req.SesssionID,
		ParentPage:    req.ParentPage,
		PageID:        clientintf.PagesSessionID(pageID),
		RequestTS:     req.Timestamp,
		ResponseTS:    time.Now(),
		Request:       req.Request,
		Response:      reply,
		AsyncTargetID: req.AsyncTargetID,
	}

	fname := filepath.Join(sessionDir, pageFnamePattern.FilenameFor(pageID))
	err = db.saveJsonFile(fname, fr)
	if err != nil {
		return fr, sess, err
	}

	// Update the overview of this session, adding the new page.
	sess, err = db.readResourcesSessionOverview(req.SesssionID)
	if err != nil {
		return fr, sess, err
	}
	sess.removeRequest(uid, tag)
	sess.appendResponse(req.ParentPage, clientintf.PagesSessionID(pageID),
		req.AsyncTargetID)
	if err := db.saveResourcesSessionOverview(req.SesssionID, &sess); err != nil {
		return fr, sess, err
	}

	// Finally, remove the old request.
	if err := db.removeResourceRequest(tx, uid, tag); err != nil {
		return fr, sess, err
	}

	return fr, sess, nil
}

// LoadFetchedResource loads resources that have already been fetched from a
// remote host. If the requested page has async resources that were already
// fetched, they are returned as well.
func (db *DB) LoadFetchedResource(tx ReadTx, uid UserID, sessionId, pageId clientintf.PagesSessionID) ([]*FetchedResource, error) {
	sess, err := db.readResourcesSessionOverview(sessionId)
	if err != nil {
		return nil, err
	}

	pages := sess.pageAndAsyncChildren(pageId)
	if len(pages) == 0 {
		return nil, fmt.Errorf("%w: page %s does not have a response",
			ErrNotFound, pageId)
	}

	sessionDir := filepath.Join(db.root, pageSessionsDir, pageSessDirPattern.FilenameFor(uint64(sessionId)))

	res := make([]*FetchedResource, 0, len(pages))
	for _, r := range pages {
		fname := filepath.Join(sessionDir, pageFnamePattern.FilenameFor(uint64(r.ID)))
		fr := new(FetchedResource)
		err := db.readJsonFile(fname, fr)
		if err != nil {
			db.log.Warnf("Unable to load sesion resource %s/%s: %v",
				sessionId, r.ID, err)
			continue
		}

		res = append(res, fr)
	}

	return res, nil
}
