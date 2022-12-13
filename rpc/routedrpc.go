package rpc

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/companyzero/bisonrelay/ratchet"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/decred/dcrd/crypto/blake256"
)

// Header that describes the payload that follows.
const (
	RMHeaderVersion = 1

	// maxRMEncodedSize is the absolute maximum value of a a decrompressed
	// routed message (plus its header). This is the upper bound used when
	// decompressing RMs to avoid decompressing bombs and should be larger
	// then the max actual RM payload and low enough to not cause DoS in
	// clients in case of such attacks.
	maxRMDecompressSize = MaxMsgSize

	// Use NoCompression by default
	RMDefaultCompressionLevel = zlib.NoCompression
)

type RMHeader struct {
	Version   uint64 `json:"version"`
	Timestamp int64  `json:"timestamp"`
	Command   string `json:"command"`
	Tag       uint32 `json:"tag"`

	Signature zkidentity.FixedSizeSignature `json:"signature,omitempty"`
}

// Private message to other client
const (
	RMCPrivateMessage = "pm"

	RMPrivateMessageModeNormal = 0
	RMPrivateMessageModeMe     = 1 // XXX not rigged up yet
)

type RMPrivateMessage struct {
	Mode    uint32 `json:"mode"`
	Message string `json:"message"`
}

type RMBlock struct {
}

const RMCBlock = "block"

// RMMediateIdentity as target to send a RMInvite on the caller's behalf. This
// should kick of an autokx.
type RMMediateIdentity struct {
	Identity [zkidentity.IdentitySize]byte `json:"identity"`
}

const RMCMediateIdentity = "mediateidentity"

// XXX does RMCMediateIdentity need a reply?

// RMInvite request an invite for third party.
type RMInvite struct {
	Invitee zkidentity.PublicIdentity `json:"invitee"` // XXX why aren't we using Identity here?
}

const RMCInvite = "invite"

// RMKXSearchRefType identifies the type of a reference used in a kx search
// message.
type RMKXSearchRefType string

const (
	KXSRTPostAuthor RMKXSearchRefType = "postauthor"
)

// RMKXSearchRef identifies a specific reference that is being used to search
// for a user.
type RMKXSearchRef struct {
	Type RMKXSearchRefType `json:"type"`
	Ref  string            `json:"ref"`
}

// RMKXSearch is sent when a user wishes to perform a transitive/recursive
// KX search for someone.
type RMKXSearch struct {
	Refs []RMKXSearchRef `json:"refs"`
}

const RMCKXSearch = "kxsearch"

// RMKXSearchReply is sent with a list of candidates the user might attempt
// to use to connect to target.
type RMKXSearchReply struct {
	TargetID zkidentity.ShortID   `json:"target_id"`
	IDs      []zkidentity.ShortID `json:"ids"`
}

const RMCKXSearchReply = "kxsearchreply"

// RMTransitiveReset ask proxy to forward reset message to another client.
const RMCTransitiveReset = "transitivereset"

type RMTransitiveReset struct {
	HalfKX ratchet.KeyExchange `json:"halfkx"` // Half ratchet
}

// RMTransitiveResetReply ask proxy to forward reset message reply to another
// client.
const RMCTransitiveResetReply = "transitiveresetreply"

type RMTransitiveResetReply struct {
	FullKX ratchet.KeyExchange `json:"fullkx"` // Full ratchet
}

const RMCGetInvoice = "getinvoice"

type RMGetInvoice struct {
	PayScheme  string
	MilliAtoms uint64
	Tag        uint32
}

const RMCInvoice = "invoice"

type RMInvoice struct {
	Invoice string
	Tag     uint32
	Error   *string `json:"error,omitempty"`
}

const RMCKXSuggestion = "kxsuggestion"

type RMKXSuggestion struct {
	Target zkidentity.PublicIdentity
}

// ComposeCompressedRM creates a blobified message that has a header and a
// payload that can then be encrypted and transmitted to the other side. The
// contents are zlib compressed with the specified level.
func ComposeCompressedRM(from *zkidentity.FullIdentity, rm interface{}, zlibLevel int) ([]byte, error) {
	h := RMHeader{
		Version:   RMHeaderVersion,
		Timestamp: time.Now().Unix(),
	}
	switch rm.(type) {
	case RMPrivateMessage:
		h.Command = RMCPrivateMessage

	case OOBPublicIdentityInvite:
		h.Command = OOBCPublicIdentityInvite // XXX this if overloaded

	case RMBlock:
		h.Command = RMCBlock

	case RMInvite:
		h.Command = RMCInvite

	case RMMediateIdentity:
		h.Command = RMCMediateIdentity

	case RMTransitiveReset:
		h.Command = RMCTransitiveReset

	case RMTransitiveResetReply:
		h.Command = RMCTransitiveResetReply

	case RMGetInvoice:
		h.Command = RMCGetInvoice

	case RMInvoice:
		h.Command = RMCInvoice

	case RMTransitiveMessage:
		h.Command = RMCTransitiveMessage

	case RMTransitiveMessageReply:
		h.Command = RMCTransitiveMessageReply

	case RMTransitiveMessageForward:
		h.Command = RMCTransitiveMessageForward

	case RMKXSearch:
		h.Command = RMCKXSearch

	case RMKXSearchReply:
		h.Command = RMCKXSearchReply

	case RMKXSuggestion:
		h.Command = RMCKXSuggestion

	// Group chat
	case RMGroupInvite:
		h.Command = RMCGroupInvite

	case RMGroupJoin:
		h.Command = RMCGroupJoin

	case RMGroupPart:
		h.Command = RMCGroupPart

	case RMGroupKill:
		h.Command = RMCGroupKill

	case RMGroupKick:
		h.Command = RMCGroupKick

	case RMGroupUpdate:
		h.Command = RMCGroupUpdate

	case RMGroupList:
		h.Command = RMCGroupList

	case RMGroupMessage:
		h.Command = RMCGroupMessage

	// File transfer
	case RMFTList:
		h.Command = RMCFTList

	case RMFTListReply:
		h.Command = RMCFTListReply

	case RMFTGet:
		h.Command = RMCFTGet

	case RMFTGetReply:
		h.Command = RMCFTGetReply

	case RMFTGetChunk:
		h.Command = RMCFTGetChunk

	case RMFTGetChunkReply:
		h.Command = RMCFTGetChunkReply

	case RMFTPayForChunk:
		h.Command = RMCFTPayForChunk

	case RMFTSendFile:
		h.Command = RMCFTSendFile

	// User
	case RMUser:
		h.Command = RMCUser

	case RMUserReply:
		h.Command = RMCUserReply

	// Post
	case RMListPosts:
		h.Command = RMCListPosts

	case RMListPostsReply:
		h.Command = RMCListPostsReply

	case RMGetPost:
		h.Command = RMCGetPost

	case RMPostShare:
		h.Command = RMCPostShare

	case RMPostsSubscribe:
		h.Command = RMCPostsSubscribe

	case RMPostsSubscribeReply:
		h.Command = RMCPostsSubscribeReply

	case RMPostsUnsubscribe:
		h.Command = RMCPostsUnsubscribe

	case RMPostsUnsubscribeReply:
		h.Command = RMCPostsUnsubscribeReply

	case RMPostGet:
		h.Command = RMCPostGet

	case RMPostGetReply:
		h.Command = RMCPostGetReply

	case RMPostStatus:
		h.Command = RMCPostStatus

	case RMPostStatusReply:
		h.Command = RMCPostStatusReply

	// Purely transitive commands

	default:
		return nil, fmt.Errorf("unknown routed message type: %T", rm)
	}

	// Encode payload
	payload, err := json.Marshal(rm)
	if err != nil {
		return nil, err
	}

	// Sign payload
	h.Signature = from.SignMessage(payload)

	// Create payload
	// Create header, note that the encoder appends a '\n'
	mb := &bytes.Buffer{}
	w, err := zlib.NewWriterLevel(mb, zlibLevel)
	if err != nil {
		return nil, err
	}
	e := json.NewEncoder(w)
	err = e.Encode(h)
	if err != nil {
		return nil, err
	}

	n, err := w.Write(payload)
	if err != nil {
		return nil, err
	}
	if n != len(payload) {
		return nil, fmt.Errorf("assert: n(%v) != len(%v)",
			n, len(payload))
	}
	err = w.Close()
	if err != nil {
		return nil, err
	}

	return mb.Bytes(), nil
}

// ComposeRM creates a blobified message that has a header and a
// payload that can then be encrypted and transmitted to the other side.
func ComposeRM(from *zkidentity.FullIdentity, rm interface{}) ([]byte, error) {
	return ComposeCompressedRM(from, rm, RMDefaultCompressionLevel)
}

func DecomposeRM(id *zkidentity.PublicIdentity, mb []byte) (*RMHeader, interface{}, error) {
	// Decompress everything
	cr, err := zlib.NewReader(bytes.NewReader(mb))
	if err != nil {
		return nil, nil, err
	}
	lr := &limitedReader{R: cr, N: maxRMDecompressSize}
	all, err := io.ReadAll(lr)
	closeErr := cr.Close()
	if err != nil {
		return nil, nil, fmt.Errorf("zlib read err: %w", err)
	}
	if closeErr != nil {
		return nil, nil, fmt.Errorf("zlib close err: %w", closeErr)
	}

	var h RMHeader
	d := json.NewDecoder(bytes.NewReader(all))
	err = d.Decode(&h)
	if err != nil {
		return nil, nil, fmt.Errorf("header decode err: %w", err)
	}

	offset := int(d.InputOffset() + 1)
	if len(all) < offset {
		return nil, nil, fmt.Errorf("invalid message length: %v",
			len(all))
	}
	if all[offset-1] != '\n' {
		return nil, nil, fmt.Errorf("not \\n")
	}

	pmd := json.NewDecoder(bytes.NewReader(all[offset:]))
	var payload interface{}
	switch h.Command {
	case RMCPrivateMessage:
		var pm RMPrivateMessage
		err = pmd.Decode(&pm)
		payload = pm

	case OOBCPublicIdentityInvite: // XXX this is overloaded
		var pii OOBPublicIdentityInvite
		err = pmd.Decode(&pii)
		payload = pii

	case RMCBlock:
		var block RMBlock
		err = pmd.Decode(&block)
		payload = block

	case RMCInvite:
		var invite RMInvite
		err = pmd.Decode(&invite)
		payload = invite

	case RMCMediateIdentity:
		var mediateIdentity RMMediateIdentity
		err = pmd.Decode(&mediateIdentity)
		payload = mediateIdentity

	case RMCTransitiveReset:
		var transitiveReset RMTransitiveReset
		err = pmd.Decode(&transitiveReset)
		payload = transitiveReset

	case RMCTransitiveResetReply:
		var transitiveResetReply RMTransitiveResetReply
		err = pmd.Decode(&transitiveResetReply)
		payload = transitiveResetReply

	case RMCGetInvoice:
		var getinv RMGetInvoice
		err = pmd.Decode(&getinv)
		payload = getinv

	case RMCInvoice:
		var inv RMInvoice
		err = pmd.Decode(&inv)
		payload = inv

	case RMCTransitiveMessage:
		var transitiveMessage RMTransitiveMessage
		err = pmd.Decode(&transitiveMessage)
		payload = transitiveMessage

	case RMCTransitiveMessageReply:
		var transitiveMessageReply RMTransitiveMessageReply
		err = pmd.Decode(&transitiveMessageReply)
		payload = transitiveMessageReply

	case RMCTransitiveMessageForward:
		var transitiveMessageForward RMTransitiveMessageForward
		err = pmd.Decode(&transitiveMessageForward)
		payload = transitiveMessageForward

	case RMCKXSearch:
		var kxs RMKXSearch
		err = pmd.Decode(&kxs)
		payload = kxs

	case RMCKXSearchReply:
		var kxsr RMKXSearchReply
		err = pmd.Decode(&kxsr)
		payload = kxsr

	case RMCKXSuggestion:
		var kxsg RMKXSuggestion
		err = pmd.Decode(&kxsg)
		payload = kxsg

		// Group vhat
	case RMCGroupInvite:
		var groupInvite RMGroupInvite
		err = pmd.Decode(&groupInvite)
		payload = groupInvite

	case RMCGroupJoin:
		var groupJoin RMGroupJoin
		err = pmd.Decode(&groupJoin)
		payload = groupJoin

	case RMCGroupPart:
		var groupPart RMGroupPart
		err = pmd.Decode(&groupPart)
		payload = groupPart

	case RMCGroupKill:
		var groupKill RMGroupKill
		err = pmd.Decode(&groupKill)
		payload = groupKill

	case RMCGroupKick:
		var groupKick RMGroupKick
		err = pmd.Decode(&groupKick)
		payload = groupKick

	case RMCGroupUpdate:
		var groupUpdate RMGroupUpdate
		err = pmd.Decode(&groupUpdate)
		payload = groupUpdate

	case RMCGroupList:
		var groupList RMGroupList
		err = pmd.Decode(&groupList)
		payload = groupList

	// File transfer
	case RMCFTList:
		var ftList RMFTList
		err = pmd.Decode(&ftList)
		payload = ftList

	case RMCFTListReply:
		var ftListReply RMFTListReply
		err = pmd.Decode(&ftListReply)
		payload = ftListReply

	case RMCFTGet:
		var ftGet RMFTGet
		err = pmd.Decode(&ftGet)
		payload = ftGet

	case RMCFTGetReply:
		var ftGetReply RMFTGetReply
		err = pmd.Decode(&ftGetReply)
		payload = ftGetReply

	case RMCFTGetChunk:
		var ftGetChunk RMFTGetChunk
		err = pmd.Decode(&ftGetChunk)
		payload = ftGetChunk

	case RMCFTGetChunkReply:
		var ftGetChunkReply RMFTGetChunkReply
		err = pmd.Decode(&ftGetChunkReply)
		payload = ftGetChunkReply

	case RMCFTPayForChunk:
		var ftPayForChunk RMFTPayForChunk
		err = pmd.Decode(&ftPayForChunk)
		payload = ftPayForChunk

	case RMCFTSendFile:
		var ftSendFile RMFTSendFile
		err = pmd.Decode(&ftSendFile)
		payload = ftSendFile

	case RMCGroupMessage:
		var groupMessage RMGroupMessage
		err = pmd.Decode(&groupMessage)
		payload = groupMessage

	// User
	case RMCUser:
		var user RMUser
		err = pmd.Decode(&user)
		payload = user

	case RMCUserReply:
		var userReply RMUserReply
		err = pmd.Decode(&userReply)
		payload = userReply

	// Post
	case RMCListPosts:
		var listPosts RMListPosts
		err = pmd.Decode(&listPosts)
		payload = listPosts

	case RMCListPostsReply:
		var listPostsReply RMListPostsReply
		err = pmd.Decode(&listPostsReply)
		payload = listPostsReply

	case RMCGetPost:
		var getPost RMGetPost
		err = pmd.Decode(&getPost)
		payload = getPost

	case RMCPostShare:
		var postShare RMPostShare
		err = pmd.Decode(&postShare)
		payload = postShare

	case RMCPostsSubscribe:
		var postsSubscribe RMPostsSubscribe
		err = pmd.Decode(&postsSubscribe)
		payload = postsSubscribe

	case RMCPostsSubscribeReply:
		var postsSubscribeReply RMPostsSubscribeReply
		err = pmd.Decode(&postsSubscribeReply)
		payload = postsSubscribeReply

	case RMCPostsUnsubscribe:
		var postsUnsubscribe RMPostsUnsubscribe
		err = pmd.Decode(&postsUnsubscribe)
		payload = postsUnsubscribe

	case RMCPostsUnsubscribeReply:
		var postsUnsubscribeReply RMPostsUnsubscribeReply
		err = pmd.Decode(&postsUnsubscribeReply)
		payload = postsUnsubscribeReply

	case RMCPostGet:
		var postGet RMPostGet
		err = pmd.Decode(&postGet)
		payload = postGet

	case RMCPostGetReply:
		var postGetReply RMPostGetReply
		err = pmd.Decode(&postGetReply)
		payload = postGetReply

	case RMCPostStatus:
		var postStatus RMPostStatus
		err = pmd.Decode(&postStatus)
		payload = postStatus

	case RMCPostStatusReply:
		var postStatusReply RMPostStatusReply
		err = pmd.Decode(&postStatusReply)
		payload = postStatusReply

	// Purely transitive commands

	default:
		return nil, nil, fmt.Errorf("unknown routed message command: %v",
			h.Command)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("decode command %v: %v", h.Command,
			err)
	}

	// Verify signature if an identity was provided
	if id != nil {
		if !id.VerifyMessage(all[offset:], h.Signature) {
			return nil, nil, fmt.Errorf("message authentication " +
				"failed")
		}
	}

	return &h, payload, nil
}

// RMTransitiveMessage is a request to forward a message
type RMTransitiveMessage struct {
	// For is the invitee identity and the corresponding public key that
	// was used to encrypt the InviteBlob.
	For zkidentity.ShortID `json:"for"`

	// CipherText contains a sntrup4591761 encapsulated shared key that is
	// used to encrypt the message. This ciphertext is decrypted by the
	// intended final recipient.
	CipherText zkidentity.FixedSizeSntrupCiphertext `json:"ciphertext,omitempty"`

	// Message is an encrypted json encoded structure.
	Message []byte `json:"message,omitempty"`
}

const RMCTransitiveMessage = "transitivemessage"

// RMTransitiveMessageReply is a reply to a transitive message.
type RMTransitiveMessageReply struct {
	// For is the intended recipient that needs Message routed.
	For zkidentity.ShortID `json:"for"`

	// Error is set if the other side encountered an error.
	Error *string `json:"error,omitempty"`
}

const RMCTransitiveMessageReply = "transitivemessagereply"

// RMTransitiveMessageForward forwards a transitive message to a user.
type RMTransitiveMessageForward struct {
	// From is the sender identity. This is used as a hint to verify the
	// signature and identity inside Message.
	From zkidentity.ShortID `json:"from"`

	// CipherText contains a sntrup4591761 encapsulated shared key that is
	// used to encrypt the InviteBlob.
	CipherText zkidentity.FixedSizeSntrupCiphertext `json:"ciphertext"`

	// Message is an encrypted json encoded structure.
	Message []byte `json:"message"`
}

const RMCTransitiveMessageForward = "tmessageforward"

// RMGroupInvite invites a user to a group chat.
type RMGroupInvite struct {
	ID          zkidentity.ShortID `json:"id"`          // group id
	Name        string             `json:"name"`        // requested group name
	Token       uint64             `json:"token"`       // invite token
	Description string             `json:"description"` // group description
	Expires     int64              `json:"expires"`     // unix time when this invite expires
}

const RMCGroupInvite = "groupinvite"

// RMGroupJoin instructs inviter that a user did or did not join the group.
type RMGroupJoin struct {
	// XXX who sent this?
	ID    zkidentity.ShortID `json:"id"`    // group id
	Token uint64             `json:"token"` // invite token, implicitly identifies sender
	Error string             `json:"error"` // accept or deny Invite
}

const RMCGroupJoin = "groupjoin"

// RMGroupPart is sent to tell the group chat that a user has departed.
type RMGroupPart struct {
	// XXX who sent this?
	ID     zkidentity.ShortID `json:"id"`     // group id
	Reason string             `json:"reason"` // reason to depart group
}

const RMCGroupPart = "grouppart"

// RMGroupKill, sender is implicit to CRPC
type RMGroupKill struct {
	// XXX who sent this?
	ID     zkidentity.ShortID `json:"id"`     // group id
	Reason string             `json:"reason"` // reason to disassemble group
}

const RMCGroupKill = "groupkill"

// RMGroupKick kicks a naughty member out of the group chat.
type RMGroupKick struct {
	Member       [zkidentity.IdentitySize]byte `json:"member"`       // kickee
	Reason       string                        `json:"reason"`       // why member was kicked
	Parted       bool                          `json:"parted"`       // kicked/parted
	NewGroupList RMGroupList                   `json:"newgrouplist"` // new GroupList
}

const RMCGroupKick = "groupkick"

// RMGroupUpdate is a forced update from the admin. This can be used in case of
// gc' generation getting out of sync.
type RMGroupUpdate struct {
	Reason       string      `json:"reason"`       // why member was kicked
	NewGroupList RMGroupList `json:"newgrouplist"` // new GroupList
}

const RMCGroupUpdate = "groupupdate"

// RMGroupList, currently we detect spoofing by ensuring the origin of the
// message.  This may not be sufficient and we may have to add a signature of
// sorts.  For now roll with this assumption.
type RMGroupList struct {
	ID         zkidentity.ShortID `json:"id"` // group id
	Name       string             `json:"name"`
	Generation uint64             `json:"generation"` // incremented every time list changes
	Timestamp  int64              `json:"timestamp"`  // unix time last generation changed

	// all participants, [0] is administrator
	// receiver must check [0] == originator
	Members []zkidentity.ShortID `json:"members"`
}

const RMCGroupList = "grouplist"

// RMGroupMessage is a message to a group.
type RMGroupMessage struct {
	ID         zkidentity.ShortID `json:"id"`         // group name
	Generation uint64             `json:"generation"` // Generation used
	Message    string             `json:"message"`    // Actual message
	Mode       MessageMode        `json:"mode"`       // 0 regular mode, 1 /me
}

const RMCGroupMessage = "groupmessage"

// RMFTList asks other side for a list of files. Directories are constants that
// describe which directories it should access. Currently only "global" and
// "shared" are allowed.
type RMFTList struct {
	Directories []string `json:"directories"`      // Which directories to obtain
	Filter      string   `json:"filter,omitempty"` // Filter list by this regex
	Tag         uint32   `json:"tag"`              // Tag to copy in replies
}

const (
	RMCFTList = "ftls"

	RMFTDGlobal = "global" // Globally accessible files
	RMFTDShared = "shared" // Files shared between two users
)

type FileManifest struct {
	Index uint64 `json:"index"`
	Size  uint64 `json:"size"`
	Hash  []byte `json:"hash"`
}

type FileMetadata struct {
	Version     uint64            `json:"version"`
	Cost        uint64            `json:"cost"`
	Size        uint64            `json:"size"`
	Directory   string            `json:"directory"`
	Filename    string            `json:"filename"`
	Description string            `json:"description"`
	Hash        string            `json:"hash"`
	Manifest    []FileManifest    `json:"manifest"` // len == number of chunks
	Signature   string            `json:"signature"`
	Attributes  map[string]string `json:"attributes,omitempty"`
}

const FileMetadataVersion = 1

// MetadataHash calculates the hash of the metadata info. Note that the specific
// information that is hashed depends on the version of the metadata.
func (fm *FileMetadata) MetadataHash() [32]byte {
	h := sha256.New()
	var b [32]byte

	writeUint64 := func(i uint64) {
		binary.LittleEndian.PutUint64(b[:], i)
		h.Write(b[:])
	}

	writeStr := func(s string) {
		h.Write([]byte(s))
	}

	writeUint64(fm.Version)
	writeUint64(fm.Size)
	writeStr(fm.Filename)
	writeStr(fm.Hash)
	writeStr(fm.Signature)

	// In the future, add new fields conditional on the metadata version so
	// that old versions will still calculate the same hash.

	copy(b[:], h.Sum(nil))
	return b
}

type RMFTListReply struct {
	Global []FileMetadata `json:"global,omitempty"`
	Shared []FileMetadata `json:"shared,omitempty"`
	Tag    uint32
	Error  *string `json:"error,omitempty"`
}

const RMCFTListReply = "ftlsreply"

// RMFTGet attempts to retrieve a file from another user
type RMFTGet struct {
	Directory string `json:"directory"` // Which directory **DEPRECATED
	Filename  string `json:"filename"`  // Which file **DEPRECATED
	Tag       uint32 `json:"tag"`       // Tag to copy in replies
	FileID    string `json:"file_id"`   // Equals metadata hash
}

const RMCFTGet = "ftget"

// RMFTGetReply file metadata get reply
type RMFTGetReply struct {
	Metadata FileMetadata `json:"metadata"`
	Tag      uint32       `json:"tag"`
	Error    *string      `json:"error,omitempty"`
}

const RMCFTGetReply = "ftgetreply"

// RMFTGetChunk attempts to retrieve a file chunk from another user.
type RMFTGetChunk struct {
	FileID string `json:"file_id"`
	Hash   []byte `json:"hash"` // Chunk to retrieve
	Index  int    `json:"index"`
	Tag    uint32 `json:"tag"` // Tag to copy in replies
}

const RMCFTGetChunk = "ftgetchunk"

// RMFTGetChunkReply chunked file get reply
type RMFTGetChunkReply struct {
	FileID string  `json:"file_id"`
	Index  int     `json:"index"`
	Chunk  []byte  `json:"chunk"` // Actual data, needs to be hashed to verify
	Tag    uint32  `json:"tag"`
	Error  *string `json:"error,omitempty"`
}

const RMCFTGetChunkReply = "ftgetchunkreply"

type RMFTPayForChunk struct {
	Tag     uint32  `json:"tag"`
	FileID  string  `json:"file_id"`
	Invoice string  `json:"invoice"`
	Index   int     `json:"index"`
	Hash    []byte  `json:"hash"`
	Error   *string `json:"error,omitempty"`
}

const RMCFTPayForChunk = "ftpayforchunk"

type RMFTSendFile struct {
	Metadata FileMetadata `json:"metadata"`
}

const RMCFTSendFile = "ftsendfile"

// RMUser retrieves user attributes such as status, profile etc. Attributes is a
// key value store that is used to describe the user attributes.
type RMUser struct{}

type RMUserReply struct {
	Identity   [sha256.Size]byte `json:"identity"`
	Attributes map[string]string `json:"attributes"`
}

const RMCUser = "user"
const RMCUserReply = "userreply"

const (
	RMUDescription    = "description"    // User description
	RMUAway           = "away"           // User away message
	RMUProfilePicture = "profilepicture" // User profile picture
)

type RMListPosts struct{}

const RMCListPosts = "listposts"

type PostListItem struct {
	ID    zkidentity.ShortID `json:"id"`
	Title string             `json:"title"`
}

type RMListPostsReply struct {
	Posts []PostListItem `json:"posts"`
}

const RMCListPostsReply = "listpostsreply"

type RMGetPost struct {
	ID            zkidentity.ShortID `json:"id"`
	IncludeStatus bool               `json:"include_status"`
}

const RMCGetPost = "getpost"

// RMPostStatusReply sets attributes such as status on a post. Attributes is a
// key value store that is used to describe the update attributes.
type RMPostStatus struct {
	Link       string            `json:"link"` // Link to post
	Attributes map[string]string `json:"attributes"`
}

type RMPostStatusReply struct {
	Error *string `json:"error,omitempty"`
}

const RMCPostStatus = "poststatus"
const RMCPostStatusReply = "poststatusreply"

const (
	RMPSHeart    = "heart"   // Heart a post
	RMPSComment  = "comment" // Comment on a post
	RMPSHeartYes = "1"       // +1 heart
	RMPSHeartNo  = "0"       // -1 heart
)

// RMPostSubscribe subscribes to new posts from a user.
type RMPostsSubscribe struct{}

const RMCPostsSubscribe = "postssubscribe"

type RMPostsSubscribeReply struct {
	Error *string `json:"error,omitempty"`
}

const RMCPostsSubscribeReply = "postssubscribereply"

// RMPostUnsubscribe unsubscribes to new posts from a user.
type RMPostsUnsubscribe struct{}

const RMCPostsUnsubscribe = "postsunsubscribe"

type RMPostsUnsubscribeReply struct {
	Error *string `json:"error,omitempty"`
}

const RMCPostsUnsubscribeReply = "postsunsubscribereply"

// RMPostShare creates a new post.
type RMPostShare struct {
	Version    uint64            `json:"version"`
	Attributes map[string]string `json:"attributes"`
}

const RMCPostShare = "postshare"

type RMPostGet struct {
	Link string `json:"link"`
}

const RMCPostGet = "postget"

type RMPostGetReply struct {
	Attributes map[string]string `json:"attributes"`
	Error      *string           `json:"error,omitempty"`
}

const RMCPostGetReply = "postgetreply"

const (
	RMPVersion     = "version"     // Post version
	RMPIdentifier  = "identifier"  // Post identifier
	RMPDescription = "description" // Post description
	RMPMain        = "main"        // Main post body
	RMPTitle       = "title"       // Title of the post
	RMPAttachment  = "attachment"  // Attached file to the post
	RMPStatusFrom  = "statusfrom"  // Status/post update from (author)
	RMPSignature   = "signature"   // Signature for the post/status
	RMPParent      = "parent"      // Parent status/post
	RMPStatusID    = "statusid"    // Status ID in status updates
	RMPNonce       = "nonce"       // Random nonce to avoid equal hashes
	RMPFromNick    = "from_nick"   // Nick of origin for post/status
	RMPTimestamp   = "timestamp"   // Timestamp of the status update
)

type PostMetadata struct {
	Version    uint64            `json:"version"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

func (pm *PostMetadata) Hash() [32]byte {
	h := blake256.New()
	var b [32]byte

	writeUint64 := func(i uint64) {
		binary.LittleEndian.PutUint64(b[:], i)
		h.Write(b[:])
	}
	wattr := func(key string) {
		h.Write([]byte(pm.Attributes[key]))
	}

	writeUint64(pm.Version)
	wattr(RMPDescription)
	wattr(RMPMain)
	wattr(RMPTitle)
	wattr(RMPStatusFrom)
	wattr(RMPParent)
	wattr(RMPFromNick)

	// Gate newer fields with a version check to ensure older copies of the
	// metadata still hash to the same value.
	copy(b[:], h.Sum(nil))
	return b
}

const PostMetadataVersion = 1

type PostMetadataStatus struct {
	Version    uint64            `json:"version"`
	From       string            `json:"from"` // Who sent update
	Link       string            `json:"link"` // Original post ID
	Attributes map[string]string `json:"attributes,omitempty"`
}

func (pm *PostMetadataStatus) Hash() [32]byte {
	h := blake256.New()
	var b [32]byte

	writeUint64 := func(i uint64) {
		binary.LittleEndian.PutUint64(b[:], i)
		h.Write(b[:])
	}
	wattr := func(key string) {
		h.Write([]byte(pm.Attributes[key]))
	}

	writeUint64(pm.Version)
	h.Write([]byte(pm.From))
	wattr(RMPIdentifier) // Identifier is the parent post.
	wattr(RMPDescription)
	wattr(RMPMain)
	wattr(RMPTitle)
	wattr(RMPParent)
	wattr(RMPSHeart)
	wattr(RMPSComment)
	wattr(RMPNonce)

	// RMPFromNick is not added because it's filled by post sharer.

	// RMPTimestamp is not added because it's undecided which timestamp
	// should be added (sender, relayer, server, etc).

	// Gate newer fields with a version check to ensure older copies of the
	// metadata still hash to the same value.
	copy(b[:], h.Sum(nil))
	return b
}

const PostMetadataStatusVersion = 1

// IsPostStatus returns true when the map of attributes (possibly) corresponds
// to a post status update.
func IsPostStatus(attrs map[string]string) bool {
	// The current version of post status does not have a differentiating
	// entry between status and post, so we infer based on the presence of
	// either a comment or heart entry, which are the currently supported
	// status updates.
	return attrs[RMPSComment] != "" || attrs[RMPSHeart] != ""
}
