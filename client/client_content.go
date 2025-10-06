package client

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/decred/slog"
)

// The list content flow is:
//
//          Alice                                    Bob
//         -------                                  -----
//
//    ListUserContent()
//             \-------- RMFTList -->
//
//                                               handleFTList()
//                                 <-- RMFTListReply ---/
//
//    handleFTListReply()
//
//
//
//
// The fetch content flow is:
//
//          Alice                                    Bob
//         -------                                  -----
//   GetUserContent()
//         \--------- RMFTGet -->
//
//                                              handleFTGet()
//                             <-- RMFTGetReply ------/
//
//   handleFTGetReply()
//         \-------- RMFTGetFileChunk -->
//
//
//                                              handleFTGetFileChunk()
//                              <-- RMFTPayForChunk ------/
//                            <-- RMFTGetChunkReply ------/
//                                 (one of chunk data or invoice)
//
//
//   handleFTPayForChunk()
//   (out-of-band payment)
//
//                                              ftPaymentForChunkCompleted()
//                              <-- RMFTGetChunkReply -----/
//
//   handleFTGetChunkReply()
//

// ShareFile shares the given filename with the given user (or to all users if
// none is specified).
//
// Cost is in atoms.
func (c *Client) ShareFile(fname string, uid *UserID,
	cost uint64, descr string,
) (clientdb.SharedFile, rpc.FileMetadata, error) {

	var f clientdb.SharedFile
	var md rpc.FileMetadata
	sign := func(hash []byte) ([]byte, error) {
		sig := c.localID.signMessage(hash)
		return sig[:], nil
	}

	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		f, md, err = c.db.ShareFile(tx, fname, uid, cost, descr, sign)
		return err
	})

	if uid == nil {
		c.log.Infof("Shared global file %q", filepath.Base(fname))
	} else {
		c.log.Infof("Shared file %q with user %s", filepath.Base(fname), uid)
	}

	return f, md, err
}

// FindSharedFileID finds the file ID of a shared file with the given filename.
func (c *Client) FindSharedFileID(fname string) (clientdb.FileID, error) {
	var fid clientdb.FileID
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		fid, err = c.db.FindSharedFileID(tx, fname)
		return err
	})
	return fid, err
}

// UnshareFile stops sharing the given file with the given user (or all users
// if unspecified).
func (c *Client) UnshareFile(fid clientdb.FileID, uid *UserID) error {
	return c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		return c.db.UnshareFile(tx, fid, uid)
	})
}

// ListLocalSharedFiles lists all locally shared files.
func (c *Client) ListLocalSharedFiles() ([]clientdb.SharedFileAndShares, error) {
	var files []clientdb.SharedFileAndShares
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		files, err = c.db.ListAllSharedFiles(tx)
		return err
	})
	return files, err
}

// ListUserContent lists the content shared by the given remote user. Dirs must
// be one of the supported dirs (rpc.RMFTDGlobal or rpc.RMFTDShared).
func (c *Client) ListUserContent(uid UserID, dirs []string, filter string) error {
	ru, err := c.rul.byID(uid)
	if err != nil {
		return err
	}

	if len(dirs) == 0 {
		return fmt.Errorf("empty list of dirs")
	}
	if len(dirs) == 1 && dirs[0] == "*" {
		// Special case * to list all known dirs.
		dirs = []string{rpc.RMFTDGlobal, rpc.RMFTDShared}
	}

	// Handle regex
	if filter != "" {
		_, err := regexp.Compile(filter)
		if err != nil {
			return fmt.Errorf("invalid regex: %v", err)
		}
	}

	// Send the request.
	return ru.sendRM(rpc.RMFTList{
		Directories: dirs,
		Filter:      filter,
	}, "ftlist")
}

// handleFTList handles listing of local user files requested by a remote user.
func (c *Client) handleFTList(ru *RemoteUser, ftls rpc.RMFTList) error {
	var global, shared []rpc.FileMetadata
	err := c.dbView(func(tx clientdb.ReadTx) error {
		// Ensure unique list of dirs.
		dirs := make(map[string]struct{}, 2)
		for _, v := range ftls.Directories {
			switch v {
			case rpc.RMFTDGlobal, rpc.RMFTDShared:
				dirs[v] = struct{}{}
			default:
				return fmt.Errorf("unknown ftls dir %q", v)
			}
		}

		var err error
		if _, ok := dirs[rpc.RMFTDGlobal]; ok {
			global, err = c.db.ListSharedFiles(tx, nil)
			if err != nil {
				return err
			}
		}
		if _, ok := dirs[rpc.RMFTDShared]; ok {
			id := ru.ID()
			shared, err = c.db.ListSharedFiles(tx, &id)
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		if !errors.Is(err, clientintf.ErrSubsysExiting) && !errors.Is(err, context.Canceled) {
			errStr := err.Error()
			err := ru.sendRM(rpc.RMFTListReply{
				Tag:   ftls.Tag,
				Error: &errStr,
			}, "ftlistreply")
			if err != nil {
				ru.log.Warnf("Error sending RMFTListReply: %v", err)
			}
		}

		return err
	}

	return ru.sendRM(rpc.RMFTListReply{
		Tag:    ftls.Tag,
		Global: global,
		Shared: shared,
	}, "ftlistreply")
}

// HasDownloadedFile returns the path to a downloaded file if it exists.
func (c *Client) HasDownloadedFile(fid zkidentity.ShortID) (string, error) {
	var res string
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		res, err = c.db.HasDownloadedFile(tx, fid)
		return err
	})
	return res, err
}

// handleFTListReply handles a reply for list from a remote user.
func (c *Client) handleFTListReply(ru *RemoteUser, ftrp rpc.RMFTListReply) error {
	if ftrp.Error != nil {
		err := errors.New(*ftrp.Error)
		c.ntfns.notifyContentListReceived(ru, nil, err)
		return err
	}

	files := append(ftrp.Global, ftrp.Shared...)
	var res []clientdb.RemoteFile
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		res, err = c.db.HasDownloadedFiles(tx, ru.Nick(), ru.ID(), files)
		return err
	})
	if err != nil {
		return err
	}
	c.log.Infof("User listed %d files", len(res))
	c.ntfns.notifyContentListReceived(ru, res, nil)

	return nil
}

// GetUserContent starts the process to fetch the given file from the remote
// user.
func (c *Client) GetUserContent(uid UserID, fid clientdb.FileID) error {
	// Ensure user exists.
	ru, err := c.rul.byID(uid)
	if err != nil {
		return err
	}

	ru.log.Infof("Starting download of file %s", fid)

	// Store that we want to download this file. This replaces any earlier
	// attempts at downloading this file.
	err = c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		_, err = c.db.StartFileDownload(tx, uid, fid, false)
		return err
	})
	if err != nil {
		return err
	}

	// Send request for file metadata.
	rmftg := rpc.RMFTGet{
		FileID: fid.String(),
	}
	payEvent := fmt.Sprintf("ftget.%s", fid.ShortLogID())
	return c.sendWithSendQ(payEvent, rmftg, uid)
}

// CancelDownload cancels downloading this file.
func (c *Client) CancelDownload(fid clientdb.FileID) error {
	return c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		return c.db.CancelFileDownload(tx, fid)
	})
}

// handleFTGet handles starting the download process for a file.
func (c *Client) handleFTGet(ru *RemoteUser, ftg rpc.RMFTGet) error {
	var fid clientdb.FileID

	// Helper to reply with an error.
	replyWithErr := func(err error) {
		errStr := err.Error()
		reply := rpc.RMFTGetReply{
			Tag:   ftg.Tag,
			Error: &errStr,
		}
		payEvent := fmt.Sprintf("ftgetreply.%s", fid.ShortLogID())
		errSend := ru.sendRM(reply, payEvent)
		if errSend != nil && !errors.Is(errSend, clientintf.ErrSubsysExiting) {
			ru.log.Warnf("Error sending FTGetReply: %v", errSend)
		}
	}

	if err := fid.FromString(ftg.FileID); err != nil {
		replyWithErr(fmt.Errorf("specified FileID is not valid: %v", err))
		return err
	}

	var md rpc.FileMetadata
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		_, md, err = c.db.GetSharedFileForUpload(tx, ru.ID(), fid)
		return err
	})
	if err != nil {
		if errors.Is(err, clientdb.ErrNotFound) {
			replyWithErr(err)
		}
		return err // Shadow other db errors.
	}

	ru.log.Infof("Sending file metadata about %q to user", md.Filename)

	reply := rpc.RMFTGetReply{
		Tag:      ftg.Tag,
		Metadata: md,
	}
	payEvent := fmt.Sprintf("ftgetreply.%s", fid.ShortLogID())
	return ru.sendRM(reply, payEvent)
}

// requestFileChunk sends a request to a remote host for one chunk of one of
// its files.
func (c *Client) requestFileChunk(ru *RemoteUser, fid clientdb.FileID, chunkIdx int,
	fm rpc.FileMetadata) error {

	chunkHash := fm.Manifest[chunkIdx].Hash

	if ru.log.Level() <= slog.LevelDebug {
		ru.log.Debugf("Requesting chunk %d (%x) of file %q (%s)",
			chunkIdx, chunkHash, fm.Filename, fid)
	} else {
		ru.log.Debugf("Requesting chunk %d of file %q",
			chunkIdx, fm.Filename)
	}

	rm := rpc.RMFTGetChunk{
		FileID: fid.String(),
		Index:  chunkIdx,
		Hash:   chunkHash,
	}
	payEvent := fmt.Sprintf("ftgetchunk.%s.%d", fid.ShortLogID(), rm.Index)
	if err := ru.sendRM(rm, payEvent); err != nil {
		return err
	}

	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		fd, err := c.db.ReadFileDownload(tx, ru.ID(), fid)
		if err != nil {
			return err
		}
		return c.db.ReplaceFileDownloadChunkState(tx, &fd, chunkIdx,
			clientdb.ChunkStateRequestedChunk)
	})
	if err != nil {
		return err
	}
	return nil
}

// payFileChunkInvoice pays for the invoice to download a chunk.
func (c *Client) payFileChunkInvoice(ru *RemoteUser, fid clientdb.FileID,
	chunkIdx int, invoice string, matoms int64) error {

	// Mark invoice as attempting to pay.
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		fd, err := c.db.ReadFileDownload(tx, ru.ID(), fid)
		if err != nil {
			return err
		}

		cs := fd.GetChunkState(chunkIdx)
		if cs != clientdb.ChunkStateHasInvoice {
			// Could've received 2 invoices and is already
			// attempting to pay one, for example.
			return fmt.Errorf("invalid chunkstate before attempting "+
				"to pay invoice: %q", cs)
		}

		return c.db.ReplaceFileDownloadChunkState(tx, &fd, chunkIdx,
			clientdb.ChunkStatePayingInvoice)
	})
	if err != nil {
		return err
	}

	// Attempt to pay invoice.
	fees, invErr := c.pc.PayInvoice(c.ctx, invoice)
	if invErr == nil {
		ru.log.Debugf("Paid for chunk %d of file download %s", chunkIdx, fid)
	}

	// Record result of attempting the payment.
	dbErr := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		fd, dbErr := c.db.ReadFileDownload(tx, ru.ID(), fid)
		if dbErr != nil {
			return dbErr
		}

		if invErr != nil {
			// Unable to pay for invoice. Clear it to request a new
			// one on the next attempt.
			invoices := map[int]string{chunkIdx: ""}
			return c.db.ReplaceFileDownloadInvoices(tx, &fd, invoices)
		}

		// Register the payment event for statistics purposes.
		payEvent := fmt.Sprintf("ftpaychunk.%s.%d", fid.ShortLogID(), chunkIdx)
		amount := -matoms
		fees := -fees
		if err := c.db.RecordUserPayEvent(tx, ru.ID(), payEvent, amount, fees); err != nil {
			return err
		}

		// Invoice paid, we expect the sender to detect this and send
		// the chunk.
		return c.db.ReplaceFileDownloadChunkState(tx, &fd, chunkIdx,
			clientdb.ChunkStatePaid)
	})
	if dbErr != nil {
		ru.log.Errorf("Unable to update invoice for file get in DB: %v", dbErr)
	}

	// Decide which error to return.
	err = invErr
	if err == nil && dbErr != nil {
		err = dbErr
	}
	return err
}

// downloadChunks is the main workhorse for chunked file download. It is called
// both for initial download and for restarting old downloads (on client
// startup).
//
// It determines the state of each chunk of the given download and takes
// actions as appropriate.
func (c *Client) downloadChunks(ru *RemoteUser, fd clientdb.FileDownload) error {
	if fd.Metadata == nil {
		// Shouldn't happen, but avoid panic.
		return fmt.Errorf("unable to start download with nil metadata")
	}

	var missing []int
	err := c.dbView(func(tx clientdb.ReadTx) error {
		missing = c.db.MissingFileDownloadChunks(tx, &fd)
		return nil
	})
	if err != nil {
		return err
	}

	c.log.Infof("Starting to downloading %d missing chunks of file %q (%s)",
		len(missing), fd.Metadata.Filename, fd.FID)

	for _, chunkIdx := range missing {
		chunkIdx := chunkIdx

		// Track which action to take, depending on the current state
		// of the chunk.
		var actionToTake string
		const actRequest = "request invoice"
		const actSendPayment = "send payment"

		// Helper func to log errors in goroutines.
		logErr := func(err error, msg string) {
			if err != nil && !errors.Is(err, clientintf.ErrSubsysExiting) {
				ru.log.Errorf(msg, err)
			}
		}

		// Decide what to do with this missing chunk. This chunk could
		// be in one of a number of states:
		// - Not requested
		// - Requested, but no reply received yet
		// - Received invoice, but not acted on it
		// - Received invoice, but it expired before acting on it
		// - Attempt to pay invoice in-flight
		// - Attempt to pay invoice succeeded, but not received chunk
		// - Received chunk
		err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
			fd, err := c.db.ReadFileDownload(tx, ru.ID(), fd.FID)
			if err != nil {
				return err
			}

			if chunkIdx >= len(fd.Metadata.Manifest) {
				// Shouldn't happen, but avoid panic.
				return fmt.Errorf("assertion error: chunkIdx %d >= len(manifest) %d",
					chunkIdx, len(fd.Metadata.Manifest))
			}

			var payMAtoms int64
			chunkState := fd.ChunkStates[chunkIdx]
			switch chunkState {
			case "":
				// Safe to request again.
				actionToTake = actRequest

			case clientdb.ChunkStateRequestedChunk:
				// Request again if it's been at least one day
				// since we last requested (to avoid sending
				// multiple redundant requests).
				var chunkUpdtTime time.Time
				if fd.ChunkUpdatedTime != nil {
					chunkUpdtTime = fd.ChunkUpdatedTime[chunkIdx]
				}
				if chunkUpdtTime.Before(time.Now().Add(-time.Hour * 24)) {
					actionToTake = actRequest
				}

			case clientdb.ChunkStateHasInvoice:
				// Have invoice, but haven't tried paying. See
				// if it's still valid to attempt payment.
				invoice := fd.GetChunkInvoice(chunkIdx)
				decoded, err := c.pc.DecodeInvoice(c.ctx, invoice)
				if err != nil {
					return fmt.Errorf("unable to decode chunk invoice: %v", err)
				}

				if decoded.IsExpired(0) {
					actionToTake = actRequest
				} else {
					actionToTake = actSendPayment
					payMAtoms = decoded.MAtoms
				}

			case clientdb.ChunkStatePayingInvoice:
				// Already attempting to pay this invoice. Do
				// nothing. Ordinarily, we would't expect to
				// reach this state here, so log a warning for now.
				//
				// TODO: could've gotten to this state and
				// crashed, so we need to actually check in the
				// payment client if the payment is in flight,
				// succeeded or failed.
				ru.log.Warnf("Chunk %d of file %s has in-flight payment",
					chunkIdx, fd.FID)

			case clientdb.ChunkStatePaid:
				// Paid for chunk, but haven't received it yet.
				// Wait for the remote client to send it.
				//
				// TODO: deal with unresponsive remotes.
				// Re-request it?  Alert user? Ban remote?
				ru.log.Warnf("Chunk %d of file %s was paid for "+
					"but hasn't been received yet",
					chunkIdx, fd.FID)

			case clientdb.ChunkStateDownloaded:
				// Already downloaded chunk, nothing to do.
			}

			// Actually take an action on this chunk.
			switch actionToTake {
			case actRequest:
				// Re-request it.
				go func() {
					err := c.requestFileChunk(ru, fd.FID, chunkIdx, *fd.Metadata)
					logErr(err, "Unable to request file chunk: %v")
				}()

			case actSendPayment:
				// Attempt payment.
				go func() {
					invoice := fd.GetChunkInvoice(chunkIdx)
					err := c.payFileChunkInvoice(ru, fd.FID,
						chunkIdx, invoice, payMAtoms)
					logErr(err, "unable to pay for chunk: %v")
				}()
			}

			return nil
		})
		if err != nil {
			return err
		}

		// Small sleep to bias downloading sequentially.
		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

// handleFTGetReply handles a reply to start a new file download.
func (c *Client) handleFTGetReply(ru *RemoteUser, gr rpc.RMFTGetReply) error {
	var fid clientdb.FileID = gr.Metadata.MetadataHash()
	var fd clientdb.FileDownload
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		fd, err = c.db.ReadFileDownload(tx, ru.ID(), fid)
		if err != nil {
			return err
		}

		return c.db.UpdateFileDownloadMetadata(tx, &fd, gr.Metadata)
	})
	if err != nil {
		return err
	}

	// Ignore this request when download is supposed to be entirely sent
	// by the uploader.
	if fd.IsSentFile {
		return fmt.Errorf("download %s is supposed to be uploader-sent",
			fid)
	}

	// Ask user for confirmation before downloading file (specially
	// due to cost).
	if c.cfg.FileDownloadConfirmer != nil {
		if !c.cfg.FileDownloadConfirmer(ru, gr.Metadata) {
			// Canceled. Remove download.
			ru.log.Infof("User canceled download of file %s", fid)
			return c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
				return c.db.CancelFileDownload(tx, fid)
			})
		}
	}

	// Fetched metadata for the given file. Request chunks.
	go func() {
		err := c.downloadChunks(ru, fd)
		if err != nil && !errors.Is(err, clientintf.ErrSubsysExiting) {
			ru.log.Errorf("Unable to download file chunk: %v", err)
		}
	}()
	return nil
}

// sendFileChunk sends the chunk data to the remote user. This is the last
// step of a chunk upload process, where the actual data is sent to the remote
// user.
func (c *Client) sendFileChunk(ru *RemoteUser, sf clientdb.SharedFile,
	chunkIdx int, cid clientdb.ChunkID, tag uint32) error {

	var data []byte
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		data, err = c.db.GetSharedFileChunkData(tx, &sf, chunkIdx)
		return err
	})
	if err != nil {
		return err
	}

	rm := rpc.RMFTGetChunkReply{
		FileID: sf.FID.String(),
		Index:  chunkIdx,
		Chunk:  data,
		Tag:    tag,
	}
	payEvent := fmt.Sprintf("ftchunkupload.%s.%d", sf.FID.ShortLogID(), chunkIdx)
	err = ru.sendRMPriority(rm, payEvent, priorityUpload, nil)
	if err != nil {
		return err
	}

	ru.log.Debugf("Sent chunk %d of file %s to remote user", chunkIdx, sf.FID)

	// Sent successfully (to server)! Mark chunk as sent.
	return c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		return c.db.MarkChunkUploadSent(tx, ru.ID(), sf.FID, cid, chunkIdx)
	})
}

// ftPaymentForChunkCompleted is called as a callback when the payment for the
// given chunk completes.
func (c *Client) ftPaymentForChunkCompleted(ru *RemoteUser, sf clientdb.SharedFile,
	chunkIdx int, cid clientdb.ChunkID, invoice string, receivedMAtoms int64) error {

	// Mark payment as completed on the DB.
	uid := ru.ID()
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		err := c.db.MarkChunkUploadPaid(tx, uid, sf.FID, cid, chunkIdx, invoice)
		if err != nil {
			return err
		}
		payEvent := fmt.Sprintf("ftrecvforchunk.%s.%d", sf.FID.ShortLogID(), chunkIdx)
		return c.db.RecordUserPayEvent(tx, ru.ID(), payEvent, receivedMAtoms, 0)
	})
	if err != nil {
		return err
	}

	ru.log.Debugf("Marked chunk %d of file %s paid by remote user",
		chunkIdx, sf.FID)

	// Attempt to send chunk to remote user.
	return c.sendFileChunk(ru, sf, chunkIdx, cid, 0)
}

func (c *Client) genInvoiceForFTUpload(tx clientdb.ReadWriteTx,
	ru *RemoteUser, sf clientdb.SharedFile,
	cid clientdb.ChunkID, chunkIdx int, amountMAtoms uint64) (string, error) {

	// cb will be called once the payment completes.
	var inv string
	cb := func(mat int64) {
		var err error
		if mat < int64(amountMAtoms) {
			err = fmt.Errorf("user paid wrong amount for chunk (paid %d, wanted %d)",
				mat, amountMAtoms)
		} else {
			err = c.ftPaymentForChunkCompleted(ru, sf, chunkIdx, cid, inv, mat)
		}
		if err != nil && !errors.Is(err, clientintf.ErrSubsysExiting) {
			ru.log.Errorf("Error processing chunk payment: %v", err)
		}
	}
	var err error
	inv, err = c.pc.GetInvoice(c.ctx, int64(amountMAtoms), cb)
	if err != nil {
		return inv, err
	}

	// Track this invoice as a chunk upload to the user.
	if err := c.db.AddChunkUploadInvoice(tx, ru.ID(), sf.FID, cid, chunkIdx, inv); err != nil {
		return inv, err
	}

	ru.log.Debugf("Generated invoice for %d atoms for user to fetch "+
		"chunk %d of file %s", amountMAtoms/1e3, chunkIdx, sf.FID)

	return inv, nil
}

// sendInvoiceForChunk sends the given pay for chunk message to the remote user
// and marks the invoice as sent in the DB afterwards. Should be called as a
// goroutine.
func (c *Client) sendInvoiceForChunk(ru *RemoteUser, fid clientdb.FileID,
	invoice string, chunkIdx int, cid clientdb.ChunkID) {

	// Invoice needed. Send reply to pay for chunk.
	rm := rpc.RMFTPayForChunk{
		FileID:  fid.String(),
		Invoice: invoice,
		Index:   chunkIdx,
		Hash:    cid[:],
	}
	payEvent := fmt.Sprintf("ftinvoiceforchunk.%s.%d", fid.ShortLogID(), chunkIdx)
	err := ru.sendRM(rm, payEvent)
	if err == nil {
		// Mark invoice sent.
		c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
			return c.db.MarkChunkUploadInvoiceSent(tx, ru.ID(), fid,
				cid, chunkIdx)
		})
	} else if !errors.Is(err, clientintf.ErrSubsysExiting) {
		ru.log.Errorf("Unable to send invoice to remote user: %v", err)
	}
}

// handleFTGetChunk is called when a chunk is requested for one of the local
// client file's.
//
// This replies either with the chunk data (if the data is free) or with an
// invoice that the remote user needs to pay before the given chunk is sent.
func (c *Client) handleFTGetChunk(ru *RemoteUser, gc rpc.RMFTGetChunk) error {
	var fid clientdb.FileID
	if err := fid.FromString(gc.FileID); err != nil {
		return err
	}

	var cid clientdb.ChunkID
	if err := cid.FromBytes(gc.Hash); err != nil {
		return err
	}

	var f clientdb.SharedFile
	var md rpc.FileMetadata
	var inv string
	chunkIdx := gc.Index
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		f, md, err = c.db.GetSharedFileForUpload(tx, ru.ID(), fid)
		if err != nil {
			return err
		}

		// Ensure chunk index is correct.
		if !clientintf.ChunkIndexMatches(&md, chunkIdx, gc.Hash) {
			return fmt.Errorf("data does not hash to specified chunk index")
		}

		// Generate invoice for the given amount.
		amountMAtoms := clientintf.FileChunkMAtoms(chunkIdx, &md)
		if amountMAtoms < 1000 {
			// File is free to download.
			return nil
		}

		// See if there's an existing, unexpired, unpaid invoice.
		cup, err := c.db.GetFileChunkUpload(tx, ru.ID(), fid, cid)
		if err == nil && len(cup.Invoices) > 0 {
			oldInv := cup.Invoices[len(cup.Invoices)-1]
			err := c.pc.IsInvoicePaid(c.ctx, int64(amountMAtoms), inv)
			// err == nil only if the invoice is settled (in which
			// case we still want to generate a new one).
			if err != nil {
				decodedInv, err := c.pc.DecodeInvoice(c.ctx, oldInv)
				if err == nil && !decodedInv.IsExpired(0) {
					// Invoice is unsettled and hasn't
					// expired. Use it.
					inv = oldInv
					return nil
				}
			}
		}

		inv, err = c.genInvoiceForFTUpload(tx, ru, f, cid,
			chunkIdx, amountMAtoms)
		return err
	})
	if err != nil {
		return err
	}

	if inv == "" {
		// No need to pay an invoice. Send chunk directly.
		return c.sendFileChunk(ru, f, chunkIdx, cid, gc.Tag)
	}

	// Invoice needed. Send reply to pay for chunk.
	go c.sendInvoiceForChunk(ru, f.FID, inv, chunkIdx, cid)
	return nil
}

// handleFTPayForChunk this is called when we receive an invoice to pay for a
// chunk of data.
func (c *Client) handleFTPayForChunk(ru *RemoteUser, pfc rpc.RMFTPayForChunk) error {
	// Verify this is a valid invoice.
	inv, err := c.pc.DecodeInvoice(c.ctx, pfc.Invoice)
	if err != nil {
		return err
	}

	var fid clientdb.FileID
	if err := fid.FromString(pfc.FileID); err != nil {
		return err
	}

	chunkIdx := pfc.Index
	err = c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		fd, err := c.db.ReadFileDownload(tx, ru.ID(), fid)
		if err != nil {
			return err
		}

		if fd.IsSentFile {
			return fmt.Errorf("chunk %d is supposed to be sent by uploader",
				chunkIdx)
		}

		if !clientintf.ChunkIndexMatches(fd.Metadata, chunkIdx, pfc.Hash) {
			return fmt.Errorf("data does not hash to specified chunk index")
		}

		if fd.ChunkStates[chunkIdx] == clientdb.ChunkStateDownloaded {
			return fmt.Errorf("already downloaded chunk %d", chunkIdx)
		}

		if fd.ChunkStates[chunkIdx] == clientdb.ChunkStatePaid {
			return fmt.Errorf("already paid for chunk %d", chunkIdx)
		}

		// TODO: check whether the invoice has a payment attempt in
		// flight or is already expired.

		// Double check amount to pay for chunk.
		wantMAtoms := clientintf.FileChunkMAtoms(chunkIdx, fd.Metadata)
		if uint64(inv.MAtoms) > wantMAtoms {
			return fmt.Errorf("unexpected value of invoice (got %d, want %d)",
				inv.MAtoms, wantMAtoms)
		}

		// Replace the outstanding invoice for this chunk.
		invoices := map[int]string{chunkIdx: pfc.Invoice}
		if err := c.db.ReplaceFileDownloadInvoices(tx, &fd, invoices); err != nil {
			return err
		}
		return err
	})
	if err != nil {
		return err
	}

	// Start to pay for this chunk.
	return c.payFileChunkInvoice(ru, fid, chunkIdx, pfc.Invoice, inv.MAtoms)
}

// handleFTGetChunkReply is called to handle received chunk data for a download.
func (c *Client) handleFTGetChunkReply(ru *RemoteUser, gcr rpc.RMFTGetChunkReply) error {
	var fid clientdb.FileID
	if err := fid.FromString(gcr.FileID); err != nil {
		return err
	}

	// Save the chunk.
	var fd clientdb.FileDownload
	var completedFname string
	var nbMissingChunks int
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		fd, err = c.db.ReadFileDownload(tx, ru.ID(), fid)
		if err != nil {
			return err
		}

		completedFname, err = c.db.SaveFileDownloadChunk(tx, ru.Nick(), &fd, gcr.Index, gcr.Chunk)
		nbMissingChunks = len(c.db.MissingFileDownloadChunks(tx, &fd))
		return err
	})
	if err != nil {
		return err
	}

	ru.log.Debugf("Downloaded chunk %d of file %s", gcr.Index, fd.FID)

	if completedFname != "" {
		baseName := filepath.Base(completedFname)
		ru.log.Infof("Completed file download %q (%s, saved as %q",
			fd.Metadata.Filename, fd.FID, baseName)
		c.ntfns.notifyFileDownloadCompleted(ru, *fd.Metadata, completedFname)
	} else {
		c.ntfns.notifyFileDownloadProgress(ru, *fd.Metadata, nbMissingChunks)
	}
	return err
}

// ListDownloads lists all outstanding downloads.
func (c *Client) ListDownloads() ([]clientdb.FileDownload, error) {
	var fds []clientdb.FileDownload
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		fds, err = c.db.ListDownloads(tx)
		return err
	})
	return fds, err
}

// SendFile sends a file to the given user without requesting a payment for it.
//
// This blocks until all chunks have been sent to brserver and acknowledged by
// it. If specified, progressChan will get reports of every sent chunk.
//
// If chunkSize is zero, the client will use the chunk size specified by the
// currently connected server.
func (c *Client) SendFile(uid UserID, chunkSize uint64, filepath string,
	progressChan chan SendProgress) error {

	// Automatically determine chunk size.
	if chunkSize == 0 {
		serverSess := c.ServerSession()
		if serverSess == nil {
			return fmt.Errorf("cannot use chunksize 0 when not connected to a server")
		}

		maxSizeVersion := serverSess.Policy().MaxMsgSizeVersion
		maxPayloadSize := rpc.MaxPayloadSizeForVersion(maxSizeVersion)
		if maxPayloadSize == 0 {
			return fmt.Errorf("server did not define max payload "+
				"size for version %d", maxSizeVersion)
		}
		chunkSize = uint64(maxPayloadSize)
	}

	ru, err := c.rul.byID(uid)
	if err != nil {
		return err
	}

	sign := func(hash []byte) ([]byte, error) {
		sig := c.localID.signMessage(hash)
		return sig[:], nil
	}

	// Calculate file chunks.
	var fm *rpc.FileMetadata
	err = c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		fm, err = c.db.CalcFileChunks(tx, filepath, chunkSize, sign)
		return err
	})
	if err != nil {
		return err
	}

	ru.log.Infof("Sending file %s in %d chunks to user (total size %d)",
		filepath, len(fm.Manifest), fm.Size)

	fmdHash := fm.MetadataHash()
	fileId := hex.EncodeToString(fmdHash[:])
	fileShortId := fileId[:16]

	// Prepare to send file metadata.
	sendqItems := make([]*preparedSendqItem, 0, len(fm.Manifest)+1)
	rmSF := rpc.RMFTSendFile{
		Metadata: *fm,
	}
	payEvent := fmt.Sprintf("ftsendfile.%s.fm", fileShortId)
	sqi, err := c.prepareSendqItem(payEvent, rmSF, priorityUpload, nil, uid)
	if err != nil {
		return err
	}
	sendqItems = append(sendqItems, sqi)

	// Save chunks in sendq.
	//
	// TODO: rewind and remove items in case any of the following chunks
	// error?
	var offset int64
	for i := range fm.Manifest {
		fc := &clientdb.SendQueueFileChunk{
			Filename: filepath,
			Offset:   offset,
			Size:     int64(fm.Manifest[i].Size),
			Index:    i,
			RMType:   rpc.RMCFTGetChunkReply,
			FileID:   fileId,
		}

		offset += fc.Size
		payEvent := fmt.Sprintf("ftsendfile.%s.%d", fileShortId, i)

		sqi, err := c.prepareSendqItem(payEvent, fc, priorityUpload, nil, uid)
		if err != nil {
			return err
		}
		sendqItems = append(sendqItems, sqi)
	}

	ru.log.Debugf("Finished adding %d chunks to sendq", len(fm.Manifest))

	// Now the items (file metadata and all chunks) are saved in the DB,
	// start sending process.
	err = c.sendPreparedSendqItemListSync(sendqItems, progressChan)
	if err != nil {
		return nil
	}

	ru.log.Infof("Finished sending file %s to user", filepath)
	return nil
}

func (c *Client) handleFTSendFile(ru *RemoteUser, sf rpc.RMFTSendFile) error {
	var fid clientdb.FileID = sf.Metadata.MetadataHash()

	// Store that we'll receive this file.
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		fd, err := c.db.StartFileDownload(tx, ru.ID(), fid, true)
		if err != nil {
			return err
		}

		err = c.db.UpdateFileDownloadMetadata(tx, &fd, sf.Metadata)
		return err
	})
	if err != nil {
		return err
	}

	ru.log.Infof("Starting remote-user-initiated download of %q (%s)",
		sf.Metadata.Filename, fid)
	return nil
}

func (c *Client) SaveEmbed(data []byte, typ string) (string, error) {
	var filePath string
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		sp := strings.Split(typ, "/")
		if len(sp) != 2 {
			return fmt.Errorf("invalid mimetype")
		}

		var err error
		fileName := fmt.Sprintf("%x.%s", sha256.Sum256(data), sp[1])

		filePath, err = c.db.SaveEmbed(fileName, data)
		return err
	})

	return filePath, err
}

// restartDownloads is called during client startup to restart all downloads.
func (c *Client) restartDownloads(ctx context.Context) error {
	var fds []clientdb.FileDownload
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		fds, err = c.db.ListOutstandingDownloads(tx)
		return err
	})
	if err != nil {
		return nil
	}

	for _, fd := range fds {
		fd := fd
		ru, err := c.rul.byID(fd.UID)
		if err != nil {
			// This could happen if we removed the ratchet/user
			// before the download completed.
			c.log.Warnf("Outstanding download %s for unknown user %s",
				fd.FID, fd.UID)
			continue
		}

		if fd.IsSentFile {
			// Skip if it's a remote-user-initiated file (they will
			// send all chunks).
			c.log.Debugf("Skiping restart of download %s from %s due to "+
				"being remotely sent", fd.FID, ru)
			continue
		}

		// Start to re-process the download.
		go func() {
			err := c.downloadChunks(ru, fd)
			if err != nil && !errors.Is(err, clientintf.ErrSubsysExiting) {
				ru.log.Errorf("Error downloading chunks of file %s: %v",
					fd.FID, err)
			}
		}()
	}

	return nil
}

// restartUploads is called during client startup to restart all uploads.
func (c *Client) restartUploads(ctx context.Context) error {
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		cups, err := c.db.ListOutstandingUploads(tx)
		if err != nil {
			return err
		}

		for _, cup := range cups {
			cup := cup
			ru, err := c.rul.byID(cup.UID)
			if err != nil {
				c.log.Warnf("Chunk upload found for unknown user %s",
					cup.UID)
				continue
			}

			sf, md, err := c.db.GetSharedFileForUpload(tx, cup.UID, cup.FID)
			if err != nil {
				c.log.Warnf("Unable to fetch file for chunk upload: %v", err)
				continue
			}

			chunkIdx := cup.Index

			// This chunk can be in one of several states:
			// - Invoices generated but unsent
			// - Invoices sent but unpaid
			// - Invoices expired (sent or unsent)
			// - Invoices paid

			wantMAtoms := int64(clientintf.FileChunkMAtoms(chunkIdx, &md))

			// First: Verify how many invoices were paid or expired.
			unexpiredInvoice := ""
			for _, inv := range cup.Invoices {
				err := c.pc.IsInvoicePaid(ctx, wantMAtoms, inv)
				if err == nil {
					// Paid! Increase nb of paid invoices.
					err := c.db.MarkChunkUploadPaid(tx,
						cup.UID, cup.FID, cup.CID, chunkIdx, inv)
					if err != nil {
						return err
					}

					payEvent := fmt.Sprintf("ftrecvforchunk.%s.%d",
						cup.FID.ShortLogID(), chunkIdx)
					err = c.db.RecordUserPayEvent(tx, cup.UID,
						payEvent, wantMAtoms, 0)
					if err != nil {
						return err
					}

					cup.Paid += 1
					continue
				}

				// Unpaid. If it's expired, remove it.
				decoded, err := c.pc.DecodeInvoice(c.ctx, inv)
				if err != nil {
					return fmt.Errorf("unable to decode chunk invoice: %v", err)
				}
				if decoded.IsExpired(0) {
					err := c.db.MarkChunkUploadInvoiceExpired(tx,
						cup.UID, cup.FID, cup.CID, chunkIdx, inv)
					if err != nil {
						return err
					}
				} else {
					unexpiredInvoice = inv
				}
			}

			// Re-generate invoice if it was requested, it expired
			// without being paid and was not sent.
			if unexpiredInvoice == "" && cup.Paid == 0 && cup.State != clientdb.ChunkStateSentInvoice {
				// Generate a new one.
				unexpiredInvoice, err = c.genInvoiceForFTUpload(tx, ru, sf, cup.CID,
					chunkIdx, uint64(wantMAtoms))
				if err != nil {
					return err
				}
			}

			// If we still have an unexpired, unsent invoice, send it.
			if unexpiredInvoice != "" && cup.Paid == 0 && cup.State != clientdb.ChunkStateSentInvoice {
				go c.sendInvoiceForChunk(ru, sf.FID, unexpiredInvoice, chunkIdx, cup.CID)
			}

			// Then: send as many copies of the chunk as were
			// already paid (but unsent).
			for i := 0; i < cup.Paid; i++ {
				go func() {
					err := c.sendFileChunk(ru, sf, chunkIdx, cup.CID, 0)
					if err != nil && !errors.Is(err, clientintf.ErrSubsysExiting) {
						ru.log.Errorf("Unable to send chunk %s: %v",
							cup.CID, err)
					}
				}()
			}

			// Small sleep to bias uploading sequentially.
			time.Sleep(100 * time.Millisecond)
		}

		return err
	})
	return err
}
