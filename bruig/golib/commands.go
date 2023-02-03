package golib

import (
	"encoding/json"
	"fmt"
	"time"
)

type CmdType uint32

const (
	CTUnknown                 CmdType = 0x00
	CTHello                           = 0x01
	CTInitClient                      = 0x02
	CTInvite                          = 0x03
	CTDecodeInvite                    = 0x04
	CTAcceptInvite                    = 0x05
	CTPM                              = 0x06
	CTAddressBook                     = 0x07
	CTLocalID                         = 0x08
	CTAcceptServerCert                = 0x09
	CTRejectServerCert                = 0x0a
	CTNewGroupChat                    = 0x0b
	CTInviteToGroupChat               = 0x0c
	CTAcceptGCInvite                  = 0x0d
	CTGetGC                           = 0x0e
	CTGCMsg                           = 0x0f
	CTListGCs                         = 0x10
	CTShareFile                       = 0x11
	CTUnshareFile                     = 0x12
	CTListSharedFiles                 = 0x13
	CTListUserContent                 = 0x14
	CTGetUserContent                  = 0x15
	CTPayTip                          = 0x16
	CTSubscribeToPosts                = 0x17
	CTUnsubscribeToPosts              = 0x18
	CTGCRemoveUser                    = 0x19
	CTKXReset                         = 0x20
	CTListPosts                       = 0x21
	CTReadPost                        = 0x22
	CTReadPostUpdates                 = 0x23
	CTGetUserNick                     = 0x24
	CTCommentPost                     = 0x25
	CTGetLocalInfo                    = 0x26
	CTRequestMediateID                = 0x27
	CTKXSearchPostAuthor              = 0x28
	CTRelayPostToAll                  = 0x29
	CTCreatePost                      = 0x30
	CTGCGetBlockList                  = 0x31
	CTGCAddToBlockList                = 0x32
	CTGCRemoveFromBlockList           = 0x33
	CTGCPart                          = 0x34
	CTGCKill                          = 0x35
	CTBlockUser                       = 0x36
	CTIgnoreUser                      = 0x37
	CTUnignoreUser                    = 0x38
	CTIsIgnored                       = 0x39
	CTListSubscribers                 = 0x3a
	CTListSubscriptions               = 0x3b
	CTListDownloads                   = 0x3c
	CTLNGetInfo                       = 0x3d
	CTLNListChannels                  = 0x3e
	CTLNListPendingChannels           = 0x3f
	CTLNGenInvoice                    = 0x40
	CTLNPayInvoice                    = 0x41
	CTLNGetServerNode                 = 0x42
	CTLNQueryRoute                    = 0x43
	CTLNGetBalances                   = 0x44
	CTLNDecodeInvoice                 = 0x45
	CTLNListPeers                     = 0x46
	CTLNConnectToPeer                 = 0x47
	CTLNDisconnectFromPeer            = 0x48
	CTLNOpenChannel                   = 0x49
	CTLNCloseChannel                  = 0x4a
	CTLNTryConnect                    = 0x4b
	CTLNInitDcrlnd                    = 0x4c
	CTLNRunDcrlnd                     = 0x4d
	CTCaptureDcrlndLog                = 0x4e
	CTLNGetDepositAddr                = 0x4f
	CTLNRequestRecvCapacity           = 0x50
	CTLNConfirmPayReqRecvChan         = 0x51
	CTConfirmFileDownload             = 0x52
	CTFTSendFile                      = 0x53
	CTEstimatePostSize                = 0x54
	CTLNStopDcrlnd                    = 0x55
	CTStopClient                      = 0x56
	CTListPayStats                    = 0x57
	CTSummUserPayStats                = 0x58
	CTClearPayStats                   = 0x59
	CTListUserPosts                   = 0x5a
	CTGetUserPost                     = 0x5b
	CTLocalRename                     = 0x5c
	CTGoOnline                        = 0x5d
	CTRemainOffline                   = 0x5e
	CTLNGetNodeInfo                   = 0x5f
	CTCreateLockFile                  = 0x60
	CTCloseLockFile                   = 0x61
	CTSkipWalletCheck                 = 0x62
	CTLNRestoreMultiSCB               = 0x63
	CTLNSaveMultiSCB                  = 0x64
	CTListUsersLastMsgTimes           = 0x65
	CTUserRatchetDebugInfo            = 0x66

	NTInviteReceived         = 0x1001
	NTInviteAccepted         = 0x1002
	NTInviteErrored          = 0x1003
	NTPM                     = 0x1004
	NTLocalIDNeeded          = 0x1005
	NTConfServerCert         = 0x1006
	NTServerSessChanged      = 0x1007
	NTNOP                    = 0x1008
	NTInvitedToGC            = 0x1009
	NTUserAcceptedGCInvite   = 0x100a
	NTGCListUpdated          = 0x100b
	NTGCMessage              = 0x100c
	NTKXCompleted            = 0x100d
	NTTipReceived            = 0x100e
	NTPostReceived           = 0x100f
	NTFileDownloadConfirm    = 0x1010
	NTFileDownloadCompleted  = 0x1011
	NTFileDownloadProgress   = 0x1012
	NTPostStatusReceived     = 0x1013
	NTLogLine                = 0x1014
	NTLNInitialChainSyncUpdt = 0x1015
	NTLNConfPayReqRecvChan   = 0x1016
	NTConfFileDownload       = 0x1017
	NTLNDcrlndStopped        = 0x1018
	NTClientStopped          = 0x1019
	NTUserPostsList          = 0x101a
	NTUserContentList        = 0x101b
	NTRemoteSubChanged       = 0x101c
)

type cmd struct {
	Type         CmdType
	ID           uint32
	ClientHandle uint32
	Payload      []byte
}

func (cmd *cmd) decode(to interface{}) error {
	return json.Unmarshal(cmd.Payload, to)
}

type CmdResult struct {
	ID      uint32
	Type    CmdType
	Err     error
	Payload []byte
}

type CmdResultLoopCB interface {
	F(*CmdResult)
}

var cmdResultChan = make(chan *CmdResult)

func call(cmd *cmd) *CmdResult {
	var v interface{}
	var err error

	decode := func(to interface{}) bool {
		err = cmd.decode(to)
		return err == nil
	}

	// Handle calls that do not need a client.
	switch cmd.Type {
	case CTHello:
		var name string
		if decode(&name) {
			v, err = handleHello(name)
		}
	case CTInitClient:
		var initClient InitClient
		if decode(&initClient) {
			err = handleInitClient(cmd.ClientHandle, initClient)
		}

	case CTLNTryConnect:
		var args LNTryExternalDcrlnd
		if decode(&args) {
			v, err = handleLNTryExternalDcrlnd(args)
		}

	case CTLNInitDcrlnd:
		var args LNInitDcrlnd
		if decode(&args) {
			v, err = handleLNInitDcrlnd(args)
		}

	case CTLNRunDcrlnd:
		var args LNInitDcrlnd
		if decode(&args) {
			v, err = handleLNRunDcrlnd(args)
		}

	case CTCaptureDcrlndLog:
		go handleCaptureDcrlndLog()

	case CTLNStopDcrlnd:
		err = handleLNStopDcrlnd()

	case CTCreateLockFile:
		var args string
		decode(&args)
		err = handleCreateLockFile(args)

	case CTCloseLockFile:
		var args string
		decode(&args)
		err = handleCloseLockFile(args)

	default:
		// Calls that need a client. Figure out the client.
		cmtx.Lock()
		var client *clientCtx
		if cs != nil {
			client = cs[cmd.ClientHandle]
		}
		cmtx.Unlock()

		if client == nil {
			err = fmt.Errorf("unknown client handle %d", cmd.ClientHandle)
		} else {
			v, err = handleClientCmd(client, cmd)
		}
	}

	var resPayload []byte
	if err == nil {
		resPayload, err = json.Marshal(v)
	}

	return &CmdResult{ID: cmd.ID, Err: err, Payload: resPayload}
}

func AsyncCall(typ CmdType, id, clientHandle uint32, payload []byte) {
	cmd := &cmd{
		Type:         typ,
		ID:           id,
		ClientHandle: clientHandle,
		Payload:      payload,
	}
	go func() { cmdResultChan <- call(cmd) }()
}

func notify(typ CmdType, payload interface{}, err error) {
	var resPayload []byte
	if err == nil {
		resPayload, err = json.Marshal(payload)
	}

	r := &CmdResult{Type: typ, Err: err, Payload: resPayload}
	cmdResultChan <- r
}

func NextCmdResult() *CmdResult {
	select {
	case r := <-cmdResultChan:
		return r
	case <-time.After(time.Second): // Timeout.
		return &CmdResult{Type: NTNOP, Payload: []byte{}}
	}
}

func CmdResultLoop(cb CmdResultLoopCB) {
	go func() {
		for {
			cb.F(<-cmdResultChan)
		}
	}()
}
