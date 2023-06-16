package client

import (
	"context"
	"fmt"
	"regexp"

	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/zkidentity"
	"golang.org/x/exp/slices"
)

// loadContentFilters reloads content filters from the DB.
func (c *Client) loadContentFilters(ctx context.Context) error {
	var filters []clientdb.ContentFilter
	err := c.db.View(ctx, func(tx clientdb.ReadTx) error {
		var err error
		filters, err = c.db.ListContentFilters(tx)
		return err
	})
	if err != nil {
		return err
	}

	c.filtersMtx.Lock()
	c.filters = filters
	c.filtersRegexps = make(map[uint64]*regexp.Regexp, len(filters))
	c.filtersMtx.Unlock()

	if len(filters) > 0 {
		c.log.Infof("Loaded %d content filters", len(filters))
	} else {
		c.log.Debugf("No content filters added to client")
	}

	return nil
}

// StoreContentFilter adds or updates a content filter. The filter starts
// applying immediately to received messages.
func (c *Client) StoreContentFilter(cf *clientdb.ContentFilter) error {
	// Double check filter regexp if valid before proceeding.
	if _, err := regexp.Compile(cf.Regexp); err != nil {
		return fmt.Errorf("invalid content filter regexp: %v", err)
	}

	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		return c.db.StoreContentFilter(tx, cf)
	})
	if err != nil {
		return err
	}

	// Store the updated filter.
	c.filtersMtx.Lock()
	updated := false
	for i := range c.filters {
		if c.filters[i].ID != cf.ID {
			continue
		}

		c.filters[i] = *cf
		delete(c.filtersRegexps, cf.ID)
		updated = true
		break
	}
	if !updated {
		c.filters = append(c.filters, *cf)
	}
	c.filtersMtx.Unlock()
	return nil
}

// RemoveContentFilter removes the content filter. The filter immediately stops
// aplying to newly received messages.
func (c *Client) RemoveContentFilter(id uint64) error {
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		return c.db.RemoveContentFilter(tx, id)
	})
	if err != nil {
		return err
	}

	// Store the updated filter.
	c.filtersMtx.Lock()
	for i := range c.filters {
		if c.filters[i].ID != id {
			continue
		}

		c.filters = slices.Delete(c.filters, i, i+1)
		break
	}
	c.filtersMtx.Unlock()
	return nil
}

// RemoveAllContentFilters removes all current content filters from the client.
func (c *Client) RemoveAllContentFilters() error {
	// Store the updated filter.
	c.filtersMtx.Lock()
	oldFilters := c.filters
	c.filters = nil
	c.filtersMtx.Unlock()

	return c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		for i := range oldFilters {
			err := c.db.RemoveContentFilter(tx, oldFilters[i].ID)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// ListContentFilters lists the active content filters.
func (c *Client) ListContentFilters() []clientdb.ContentFilter {
	c.filtersMtx.Lock()
	res := slices.Clone(c.filters)
	c.filtersMtx.Unlock()
	return res
}

// shouldFilter determines if any of the content filtering rules applies to
// the data. It returns the id of the rule that filters the data.
func (c *Client) shouldFilter(uid clientintf.UserID, gcid *zkidentity.ShortID,
	pid *clientintf.PostID, postFrom *clientintf.UserID, data string) (bool, uint64) {

	var filter bool
	var id uint64

	isGCM := gcid != nil
	isPostComment := postFrom != nil
	isPost := pid != nil && !isPostComment
	isPM := !isPost && !isPostComment && !isGCM

	c.filtersMtx.Lock()
	for _, cf := range c.filters {
		// Determine if this cf applies to this message.
		if isPM && cf.SkipPMs {
			continue
		}
		if isGCM && cf.SkipGCMs {
			continue
		}
		if isPost && cf.SkipPosts {
			continue
		}
		if isPostComment && cf.SkipPostComments {
			continue
		}
		if cf.UID != nil && !cf.UID.ConstantTimeEq(&uid) {
			continue
		}
		if cf.GC != nil && gcid != nil && !cf.GC.ConstantTimeEq(gcid) {
			continue
		}

		// This cf does in fact apply to this message. Check the regexp.
		re, ok := c.filtersRegexps[cf.ID]
		if !ok {
			// First time this regexp is being used, initialize it.
			var err error
			re, err = regexp.Compile(cf.Regexp)
			if err != nil {
				c.log.Warnf("Invalid content filter regexp (filter %d): %v",
					cf.ID, err)
			}

			// Store nil in case of errors, so that we don't attempt
			// to compile again.
			c.filtersRegexps[cf.ID] = re
		}
		if re == nil {
			// Invalid filter, skip it.
			continue
		}

		if !re.MatchString(data) {
			continue
		}

		// Should filter!
		c.log.Tracef("Filtering msg from %s due to rule %d", uid, cf.ID)
		filter = true
		id = cf.ID

		// Only create the notification object if there are handlers
		// for the event registered, to avoid unnecessary work.
		if c.ntfns.AnyRegistered(OnMsgContentFilteredNtfn(nil)) {
			event := MsgContentFilteredEvent{
				UID:           uid,
				GC:            gcid,
				PID:           pid,
				PostFrom:      postFrom,
				IsPostComment: isPostComment,
				Msg:           data,
				Rule:          cf,
			}
			c.ntfns.notifyMsgContentFiltered(event)
		}
		break
	}
	c.filtersMtx.Unlock()

	return filter, id
}

// FilterPM returns true if the pm sent by the specified user should be filtered.
func (c *Client) FilterPM(uid UserID, msg string) (bool, uint64) {
	return c.shouldFilter(uid, nil, nil, nil, msg)
}

// FilterGCM returns true if the GCM sent by the specified user in the GC should
// be filtered.
func (c *Client) FilterGCM(uid UserID, gcid zkidentity.ShortID, msg string) (bool, uint64) {
	return c.shouldFilter(uid, &gcid, nil, nil, msg)
}

// FilterPost returns true if the post sent by the specified user should be
// filtered.
func (c *Client) FilterPost(uid UserID, pid clientintf.PostID, post string) (bool, uint64) {
	return c.shouldFilter(uid, nil, &pid, nil, post)
}

// FilterPostComment returns true if the post comment sent by the specified
// user should be filtered.
func (c *Client) FilterPostComment(uid, postFrom UserID, pid clientintf.PostID, comment string) (bool, uint64) {
	return c.shouldFilter(uid, nil, &pid, &postFrom, comment)
}
