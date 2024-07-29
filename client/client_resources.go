package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/client/resources"
	"github.com/companyzero/bisonrelay/internal/strescape"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/davecgh/go-spew/spew"
	"github.com/decred/slog"
)

// NewPagesSession creates a new namespace for resource requests. A "session"
// is roughly equivalent to a browser tab: multiple requests may be performed
// associated with a single session.
func (c *Client) NewPagesSession() (clientintf.PagesSessionID, error) {
	var id clientintf.PagesSessionID
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		id, err = c.db.NewPagesSession(tx)
		return err
	})
	c.log.Debugf("Starting new session %d", id)
	return id, err
}

// FetchLocalResource fetches the local resource and triggers a correspoding
// ResourceFetched call.
func (c *Client) FetchLocalResource(path []string, meta map[string]string, data json.RawMessage) error {
	if c.cfg.ResourcesProvider == nil {
		return fmt.Errorf("resources provider not configured")
	}

	rm := rpc.RMFetchResource{
		Path: path,
		Meta: meta,
		Data: data,
	}
	reqTS := time.Now()

	res, err := c.cfg.ResourcesProvider.Fulfill(c.ctx, c.PublicID(), &rm)
	if errors.Is(err, resources.ErrProviderNotFound) {
		return fmt.Errorf("Provider not found for local request path %s",
			strescape.ResourcesPath(path))
	} else if err != nil {
		return err
	}

	fr := clientdb.FetchedResource{
		UID:        c.PublicID(),
		SessionID:  0,
		ParentPage: 0,
		PageID:     0,
		RequestTS:  reqTS,
		ResponseTS: time.Now(),
		Request:    rm,
		Response:   *res,
	}
	var overv clientdb.PageSessionOverview

	c.ntfns.notifyResourceFetched(nil, fr, overv)
	return nil
}

// FetchResource requests the specified resource from the client. Once the
// resource is returned the ResourceFetched handler will be called with
// the response using the returned tag.
func (c *Client) FetchResource(uid UserID, path []string, meta map[string]string,
	sess, parentPage clientintf.PagesSessionID, data json.RawMessage,
	asyncTargetID string) (rpc.ResourceTag, error) {

	ru, err := c.UserByID(uid)
	if err != nil {
		return 0, err
	}

	rm := rpc.RMFetchResource{
		Path: path,
		Meta: meta,
		Data: data,
	}

	err = c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		return c.db.StoreResourceRequest(tx, uid, sess, parentPage,
			&rm, asyncTargetID)
	})
	if err != nil {
		return 0, err
	}

	if ru.log.Level() < slog.LevelInfo {
		ru.log.Debugf("Requesting resource tag %s path %s meta %s",
			rm.Tag, strescape.ResourcesPath(path), spew.Sdump(meta))
	} else {
		ru.log.Infof("Requesting resource tag %s path %s",
			rm.Tag, strescape.ResourcesPath(path))
	}

	payEvent := "fetchresource." + strescape.ResourcesPath(path)
	err = c.sendWithSendQ(payEvent, rm, uid)
	if err != nil {
		return 0, err
	}

	return rm.Tag, err
}

// handleFetchResource handles receiving a request to send a resource to the
// remote client.
func (c *Client) handleFetchResource(ru *RemoteUser, fr rpc.RMFetchResource) error {
	// TODO: support chunked data request.
	if fr.Index != 0 || fr.Count != 0 {
		return fmt.Errorf("chunked request not implemented")
	}

	if c.cfg.ResourcesProvider == nil {
		return fmt.Errorf("resources provider not configured")
	}

	if ru.log.Level() < slog.LevelInfo {
		ru.log.Debugf("Fullfilling request %d/%d tag %s for resource %s data %d meta %s",
			fr.Index, fr.Count, fr.Tag, strescape.ResourcesPath(fr.Path),
			len(fr.Data), spew.Sdump(fr.Meta))
	} else {
		ru.log.Infof("Fullfilling request %d/%d tag %s for resource %s data %d",
			fr.Index, fr.Count, fr.Tag, strescape.ResourcesPath(fr.Path),
			len(fr.Data))
	}

	res, err := c.cfg.ResourcesProvider.Fulfill(c.ctx, ru.ID(), &fr)
	if errors.Is(err, resources.ErrProviderNotFound) {
		ru.log.Infof("Provider not found for request tag %s path %s",
			fr.Tag, strescape.ResourcesPath(fr.Path))
		return nil
	} else if err != nil {
		return err
	}
	res.Tag = fr.Tag // Ensure response tag is same as request tag

	if len(res.Data) > rpc.MaxChunkSize {
		return fmt.Errorf("resource %s returned more data (%d) than "+
			"max chunk size %d", strescape.ResourcesPath(fr.Path),
			len(res.Data), rpc.MaxChunkSize)
	}

	ru.log.Debugf("Fulfilled request tag %s with status %s chunk %d/%d len %d",
		res.Tag, res.Status, res.Index, res.Count, len(res.Data))

	payEvent := "resource." + strescape.ResourcesPath(fr.Path)
	return c.sendWithSendQ(payEvent, *res, ru.ID())
}

// handleFetchResourceReply handles the reply to a requested resource.
func (c *Client) handleFetchResourceReply(ru *RemoteUser, frr rpc.RMFetchResourceReply) error {
	// TODO: support chunked response.
	if frr.Index != frr.Count || frr.Index != 0 {
		return fmt.Errorf("chunked resource reply not implemented")
	}

	var req rpc.RMFetchResource
	var fr clientdb.FetchedResource
	var sess clientdb.PageSessionOverview
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		fr, sess, err = c.db.StoreFetchedResource(tx, ru.ID(), frr.Tag, frr)
		return err
	})
	if err != nil {
		return err
	}

	ru.log.Infof("Received resource reply %d tag %s path %s chunk %d/%d %d bytes",
		frr.Status, frr.Tag, strescape.ResourcesPath(req.Path),
		frr.Index, frr.Count, len(frr.Data))
	c.ntfns.notifyResourceFetched(ru, fr, sess)
	return nil
}

// LoadFetchedResource loads an already fetched resource for the given session
// and page. The first element of the returned slice is the page, the others
// are async requests that originated from the same page.
func (c *Client) LoadFetchedResource(uid UserID, session,
	page clientintf.PagesSessionID) ([]*clientdb.FetchedResource, error) {

	var res []*clientdb.FetchedResource
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		res, err = c.db.LoadFetchedResource(tx, uid, session, page)
		return err
	})
	return res, err
}
