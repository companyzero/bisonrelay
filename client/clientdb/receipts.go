package clientdb

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
)

// StoreReceiveReceipt stores receive scripts for all domains.
func (db *DB) StoreReceiveReceipt(tx ReadWriteTx, sender, localID UserID, rr *rpc.RMReceiveReceipt,
	serverRecvTime time.Time) error {

	var fpath string
	switch rr.Domain {
	case rpc.ReceiptDomainPosts:
		if rr.ID == nil {
			return fmt.Errorf("post receive receipt with nil ID")
		}

		postPath := filepath.Join(db.root, postsDir, localID.String(),
			rr.ID.String())
		if !fileExists(postPath) {
			return fmt.Errorf("post %s/%s does not exist", localID, rr.ID)
		}

		fpath = postPath + postRecvReceiptSuff

	case rpc.ReceiptDomainPostComments:
		if rr.ID == nil {
			return fmt.Errorf("post comment receive receipt with nil ID")
		}
		if rr.SubID == nil {
			return fmt.Errorf("post comment receive receipt with nil SubID")
		}

		postPath := filepath.Join(db.root, postsDir, localID.String(),
			rr.ID.String())
		if !fileExists(postPath) {
			return fmt.Errorf("post %s/%s does not exist", localID, rr.ID)
		}

		dir := postPath + postCommentRecvReceiptDir
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}

		fpath = filepath.Join(dir, rr.SubID.String())

	default:
		return fmt.Errorf("unknown domain %q of receive receipt", rr.Domain)
	}

	// Open file. If that receive receipt for this user is not yet stored,
	// store it.
	f, err := os.OpenFile(fpath, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	var dbrr ReceiveReceipt
	dec := json.NewDecoder(f)
	err = dec.Decode(&dbrr)
	for ; err == nil; err = dec.Decode(&dbrr) {
		if dbrr.User == sender {
			// Already stored.
			return nil
		}
	}
	if !errors.Is(err, io.EOF) {
		return err
	}

	if _, err := f.Seek(0, 2); err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	dbrr = ReceiveReceipt{
		User:       sender,
		ServerTime: serverRecvTime.UnixMilli(),
		ClientTime: rr.ClientTime,
	}
	return enc.Encode(dbrr)
}

// listReceiveReceipts reads all receive receipts from a file.
func (db *DB) listReceiveReceipts(fpath string) ([]*ReceiveReceipt, error) {
	f, err := os.Open(fpath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var res []*ReceiveReceipt
	dbrr := new(ReceiveReceipt)
	dec := json.NewDecoder(f)
	err = dec.Decode(&dbrr)
	for ; err == nil; err = dec.Decode(&dbrr) {
		res = append(res, dbrr)
		dbrr = new(ReceiveReceipt)
	}
	if !errors.Is(err, io.EOF) {
		return nil, err
	}
	return res, nil
}

// ListPostReceiveReceipts lists receive receipts for a post.
func (db *DB) ListPostReceiveReceipts(tx ReadTx, postFrom UserID, pid PostID) ([]*ReceiveReceipt, error) {
	fpath := filepath.Join(db.root, postsDir, postFrom.String(),
		pid.String()+postRecvReceiptSuff)
	return db.listReceiveReceipts(fpath)
}

// ListPostCommentReceiveReceipts lists receive receipts for a post comment.
func (db *DB) ListPostCommentReceiveReceipts(tx ReadTx, postFrom UserID, pid PostID,
	commentID zkidentity.ShortID) ([]*ReceiveReceipt, error) {
	fpath := filepath.Join(db.root, postsDir, postFrom.String(),
		pid.String()+postCommentRecvReceiptDir, commentID.String())
	return db.listReceiveReceipts(fpath)
}
