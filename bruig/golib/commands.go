package golib

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/companyzero/bisonrelay/client"
	"github.com/davecgh/go-spew/spew"
)

type CmdType = int32

const (
	CTUnknown                      CmdType = 0x00
	CTHello                        CmdType = 0x01
	CTInitClient                   CmdType = 0x02
	CTInvite                       CmdType = 0x03
	CTDecodeInvite                 CmdType = 0x04
	CTAcceptInvite                 CmdType = 0x05
	CTPM                           CmdType = 0x06
	CTAddressBook                  CmdType = 0x07
	CTLocalID                      CmdType = 0x08
	CTAcceptServerCert             CmdType = 0x09
	CTRejectServerCert             CmdType = 0x0a
	CTNewGroupChat                 CmdType = 0x0b
	CTInviteToGroupChat            CmdType = 0x0c
	CTAcceptGCInvite               CmdType = 0x0d
	CTGetGC                        CmdType = 0x0e
	CTGCMsg                        CmdType = 0x0f
	CTListGCs                      CmdType = 0x10
	CTShareFile                    CmdType = 0x11
	CTUnshareFile                  CmdType = 0x12
	CTListSharedFiles              CmdType = 0x13
	CTListUserContent              CmdType = 0x14
	CTGetUserContent               CmdType = 0x15
	CTPayTip                       CmdType = 0x16
	CTSubscribeToPosts             CmdType = 0x17
	CTUnsubscribeToPosts           CmdType = 0x18
	CTGCRemoveUser                 CmdType = 0x19
	CTKXReset                      CmdType = 0x20
	CTListPosts                    CmdType = 0x21
	CTReadPost                     CmdType = 0x22
	CTReadPostUpdates              CmdType = 0x23
	CTGetUserNick                  CmdType = 0x24
	CTCommentPost                  CmdType = 0x25
	CTGetLocalInfo                 CmdType = 0x26
	CTRequestMediateID             CmdType = 0x27
	CTKXSearchPostAuthor           CmdType = 0x28
	CTRelayPostToAll               CmdType = 0x29
	CTCreatePost                   CmdType = 0x30
	CTGCGetBlockList               CmdType = 0x31
	CTGCAddToBlockList             CmdType = 0x32
	CTGCRemoveFromBlockList        CmdType = 0x33
	CTGCPart                       CmdType = 0x34
	CTGCKill                       CmdType = 0x35
	CTBlockUser                    CmdType = 0x36
	CTIgnoreUser                   CmdType = 0x37
	CTUnignoreUser                 CmdType = 0x38
	CTIsIgnored                    CmdType = 0x39
	CTListSubscribers              CmdType = 0x3a
	CTListSubscriptions            CmdType = 0x3b
	CTListDownloads                CmdType = 0x3c
	CTLNGetInfo                    CmdType = 0x3d
	CTLNListChannels               CmdType = 0x3e
	CTLNListPendingChannels        CmdType = 0x3f
	CTLNGenInvoice                 CmdType = 0x40
	CTLNPayInvoice                 CmdType = 0x41
	CTLNGetServerNode              CmdType = 0x42
	CTLNQueryRoute                 CmdType = 0x43
	CTLNGetBalances                CmdType = 0x44
	CTLNDecodeInvoice              CmdType = 0x45
	CTLNListPeers                  CmdType = 0x46
	CTLNConnectToPeer              CmdType = 0x47
	CTLNDisconnectFromPeer         CmdType = 0x48
	CTLNOpenChannel                CmdType = 0x49
	CTLNCloseChannel               CmdType = 0x4a
	CTLNTryConnect                 CmdType = 0x4b
	CTLNInitDcrlnd                 CmdType = 0x4c
	CTLNRunDcrlnd                  CmdType = 0x4d
	CTCaptureDcrlndLog             CmdType = 0x4e
	CTLNGetDepositAddr             CmdType = 0x4f
	CTLNRequestRecvCapacity        CmdType = 0x50
	CTLNConfirmPayReqRecvChan      CmdType = 0x51
	CTConfirmFileDownload          CmdType = 0x52
	CTFTSendFile                   CmdType = 0x53
	CTEstimatePostSize             CmdType = 0x54
	CTLNStopDcrlnd                 CmdType = 0x55
	CTStopClient                   CmdType = 0x56
	CTListPayStats                 CmdType = 0x57
	CTSummUserPayStats             CmdType = 0x58
	CTClearPayStats                CmdType = 0x59
	CTListUserPosts                CmdType = 0x5a
	CTGetUserPost                  CmdType = 0x5b
	CTLocalRename                  CmdType = 0x5c
	CTGoOnline                     CmdType = 0x5d
	CTRemainOffline                CmdType = 0x5e
	CTLNGetNodeInfo                CmdType = 0x5f
	CTCreateLockFile               CmdType = 0x60
	CTCloseLockFile                CmdType = 0x61
	CTSkipWalletCheck              CmdType = 0x62
	CTLNRestoreMultiSCB            CmdType = 0x63
	CTLNSaveMultiSCB               CmdType = 0x64
	CTListUsersLastMsgTimes        CmdType = 0x65
	CTUserRatchetDebugInfo         CmdType = 0x66
	CTResendGCList                 CmdType = 0x67
	CTGCUpgradeVersion             CmdType = 0x68
	CTGCModifyAdmins               CmdType = 0x69
	CTGetKXSearch                  CmdType = 0x6a
	CTSuggestKX                    CmdType = 0x6b
	CTListAccounts                 CmdType = 0x6c
	CTCreateAccount                CmdType = 0x6d
	CTSendOnchain                  CmdType = 0x6e
	CTRedeeemInviteFunds           CmdType = 0x6f
	CTFetchInvite                  CmdType = 0x70
	CTReadOnboard                  CmdType = 0x71
	CTRetryOnboard                 CmdType = 0x72
	CTSkipOnboardStage             CmdType = 0x73
	CTStartOnboard                 CmdType = 0x74
	CTCancelOnboard                CmdType = 0x75
	CTFetchResource                CmdType = 0x76
	CTHandshake                    CmdType = 0x77
	CTLoadUserHistory              CmdType = 0x78
	CTAddressBookEntry             CmdType = 0x79
	CTResetAllOldKX                CmdType = 0x7a
	CTTransReset                   CmdType = 0x7b
	CTGCModifyOwner                CmdType = 0x7c
	CTRescanWallet                 CmdType = 0x7d
	CTListTransactions             CmdType = 0x7e
	CTListPostRecvReceipts         CmdType = 0x7f
	CTListPostCommentRecvReceipts  CmdType = 0x80
	CTMyAvatarSet                  CmdType = 0x81
	CTMyAvatarGet                  CmdType = 0x82
	CTGetRunState                  CmdType = 0x83
	CTEnableBackgroundNtfs         CmdType = 0x84
	CTDisableBackgroundNtfs        CmdType = 0x85
	CTZipLogs                      CmdType = 0x86
	CTEnableProfiler               CmdType = 0x87
	CTNotifyServerSessionState     CmdType = 0x88
	CTEnableTimedProfiling         CmdType = 0x89
	CTZipTimedProfilingLogs        CmdType = 0x8a
	CTListGCInvites                CmdType = 0x8b
	CTCancelDownload               CmdType = 0x8c
	CTSubAllPosts                  CmdType = 0x8d
	CTUpdateUINotificationsCfg     CmdType = 0x8e
	CTGCListUnkxdMembers           CmdType = 0x8f
	CTListKXs                      CmdType = 0x90
	CTListMIRequests               CmdType = 0x91
	CTListAudioDevices             CmdType = 0x92
	CTAudioStartRecordNode         CmdType = 0x93
	CTAudioStartPlaybackNote       CmdType = 0x94
	CTAudioStopNote                CmdType = 0x95
	CTAudioNoteEmbed               CmdType = 0x96
	CTLoadFetchedResource          CmdType = 0x97
	CTRTDTJoinSession              CmdType = 0x98
	CTRTDTListLiveSessions         CmdType = 0x99
	CTRTDTListSessions             CmdType = 0x9a
	CTRTDTGetSession               CmdType = 0x9b
	CTRTDTSwitchHotAudio           CmdType = 0x9c
	CTRTDTCreateSession            CmdType = 0x9d
	CTRTDTInviteToSession          CmdType = 0x9e
	CTRTDTAcceptInvite             CmdType = 0x9f
	CTRTDTLeaveLiveSession         CmdType = 0xa0
	CTAudioSetDevices              CmdType = 0xa1
	CTRTDTModifyLivePeerVolumeGain CmdType = 0xa2
	CTAudioSetCaptureGain          CmdType = 0xa3
	CTAudioGetCaptureGain          CmdType = 0xa4
	CTRTDTKickFromLiveSession      CmdType = 0xa5
	CTRTDTRemoveFromSession        CmdType = 0xa6
	CTRTDTExitSession              CmdType = 0xa7
	CTRTDTRotateCookies            CmdType = 0xa8
	CTRTDTDissolveSession          CmdType = 0xa9
	CTRTDTGetLiveSession           CmdType = 0xaa
	CTRTDTSendChatMsg              CmdType = 0xab
	CTRTDTGetChatMessages          CmdType = 0xac
	CTAudioSetPlaybackGain         CmdType = 0xad
	CTAudioGetPlaybackGain         CmdType = 0xae
	CTCancelKX                     CmdType = 0xaf
	CTCancelMediateID              CmdType = 0xb0

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
	NTGCJoined               = 0x100b
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
	NTInvoiceGenFailed       = 0x101d
	NTGCVersionWarn          = 0x101e
	NTGCAddedMembers         = 0x101f
	NTGCUpgradedVersion      = 0x1020
	NTGCMemberParted         = 0x1021
	NTGCAdminsChanged        = 0x1022
	NTKXSuggested            = 0x1023
	NTTipUserProgress        = 0x1024
	NTOnboardStateChanged    = 0x1025
	NTResourceFetched        = 0x1026
	NTSimpleStoreOrderPlaced = 0x1027
	NTHandshakeStage         = 0x1028
	NTRescanWalletProgress   = 0x1029
	NTServerUnwelcomeError   = 0x102a
	NTProfileUpdated         = 0x102b
	NTAddressBookLoaded      = 0x102c
	NTPostsSubscriberUpdated = 0x102d
	NTUINotification         = 0x102e
	NTGCKilled               = 0x102f
	NTRTDTInvitedToSession   = 0x1030
	NTRTDTSessionUpdated     = 0x1031
	NTRTDTJoinedLiveSession  = 0x1032
	NTRTDTLivePeerJoined     = 0x1033
	NTRTDTLivePeerStalled    = 0x1034
	NTRTDTPeerSoundChanged   = 0x1035
	NTRTDTSessionSendError   = 0x1036
	NTRTDTKickedFromLive     = 0x1037
	NTRTDTRemovedFromSess    = 0x1038
	NTRTDTRotatedCookie      = 0x1039
	NTRTDTSessDissolved      = 0x103a
	NTRTDTPeerExited         = 0x103b
	NTRTDTRemadeSessHot      = 0x103c
	NTRTDTChatMsgReceived    = 0x103d
	NTRTDTRTTCalculated      = 0x103e
)

type cmd struct {
	Type         CmdType
	ID           int32
	ClientHandle int32
	Payload      []byte
}

func (cmd *cmd) decode(to interface{}) error {
	return json.Unmarshal(cmd.Payload, to)
}

type CmdResult struct {
	ID      int32
	Type    CmdType
	Err     error
	Payload []byte
}

type CmdResultLoopCB interface {
	F(id int32, typ int32, payload string, err string)
	UINtfn(text string, nick string, ts int64)
}

var cmdResultChan = make(chan *CmdResult)

func call(cmd *cmd) *CmdResult {
	var v interface{}
	var err error

	decode := func(to interface{}) bool {
		err = cmd.decode(to)
		if err != nil {
			err = fmt.Errorf("unable to decode input payload: %v; full payload: %s", err, spew.Sdump(cmd.Payload))
		}
		return err == nil
	}

	ctx := context.Background()
	// Handle calls that do not need a client.
	switch cmd.Type {
	case CTHello:
		var name string
		if decode(&name) {
			v, err = handleHello(name)
		}
	case CTInitClient:
		var initClient initClient
		if decode(&initClient) {
			err = handleInitClient(uint32(cmd.ClientHandle), initClient)
		}

	case CTLNTryConnect:
		var args lnTryExternalDcrlnd
		if decode(&args) {
			v, err = handleLNTryExternalDcrlnd(args)
		}

	case CTLNInitDcrlnd:
		var args lnInitDcrlnd
		if decode(&args) {
			v, err = handleLNInitDcrlnd(ctx, args)
		}

	case CTLNRunDcrlnd:
		var args lnInitDcrlnd
		if decode(&args) {
			v, err = handleLNRunDcrlnd(ctx, args)
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

	case CTGetRunState:
		v = runState{
			DcrlndRunning: isDcrlndRunning(),
			ClientRunning: isClientRunning(uint32(cmd.ClientHandle)),
		}
		err = nil

	case CTEnableProfiler:
		var args string
		decode(&args)
		if args == "" {
			args = "0.0.0.0:8118"
		}
		fmt.Printf("Enabling profiler on %s\n", args)
		go func() {
			err := http.ListenAndServe(args, nil)
			if err != nil {
				fmt.Printf("Unable to listen on profiler %s: %v\n",
					args, err)
			}
		}()

	case CTEnableTimedProfiling:
		var args string
		decode(&args)
		go globalProfiler.Run(args)

	case CTZipTimedProfilingLogs:
		var args string
		decode(&args)
		err = globalProfiler.zipLogs(args)

	default:
		// Calls that need a client. Figure out the client.
		cmtx.Lock()
		var client *clientCtx
		if cs != nil {
			client = cs[uint32(cmd.ClientHandle)]
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

	return &CmdResult{ID: cmd.ID, Type: cmd.Type, Err: err, Payload: resPayload}
}

func AsyncCall(typ CmdType, id, clientHandle int32, payload []byte) {
	cmd := &cmd{
		Type:         typ,
		ID:           id,
		ClientHandle: clientHandle,
		Payload:      payload,
	}
	go func() { cmdResultChan <- call(cmd) }()
}

func AsyncCallStr(typ int64, id, clientHandle int64, payload string) {
	cmd := &cmd{
		Type:         CmdType(typ),
		ID:           int32(id),
		ClientHandle: int32(clientHandle),
		Payload:      []byte(payload),
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

var (
	cmdResultLoopsMtx   sync.Mutex
	cmdResultLoops      = map[int32]chan struct{}{}
	cmdResultLoopsLive  atomic.Int32
	cmdResultLoopsCount int32
)

// emitBackgroundNtfns emits background notifications to the callback object.
func emitBackgroundNtfns(r *CmdResult, cb CmdResultLoopCB) {
	switch r.Type {
	case NTUINotification:
		var n client.UINotification
		err := json.Unmarshal(r.Payload, &n)
		if err != nil {
			return
		}

		cb.UINtfn(n.Text, n.FromNick, n.Timestamp)

	default:
		// Ignore every other notification.
	}
}

// CmdResultLoop runs the loop that fetches async results in a goroutine and
// calls cb.F() with the results. Returns an ID that may be passed to
// StopCmdResultLoop to stop this goroutine.
//
// If onlyBgNtfns is specified, only background notifications are sent.
func CmdResultLoop(cb CmdResultLoopCB, onlyBgNtfns bool) int32 {
	cmdResultLoopsMtx.Lock()
	id := cmdResultLoopsCount + 1
	cmdResultLoopsCount += 1
	ch := make(chan struct{})
	cmdResultLoops[id] = ch
	cmdResultLoopsLive.Add(1)
	cmdResultLoopsMtx.Unlock()

	// onlyBgNtfns == true when this is called from the native plugin
	// code while the flutter engine is _not_ attached to it.
	deliverBackgroundNtfns := onlyBgNtfns

	cmtx.Lock()
	if cs != nil && cs[0x12131400] != nil {
		cc := cs[0x12131400]
		cc.log.Infof("CmdResultLoop: starting new run for pid %d id %d",
			os.Getpid(), id)
	}
	cmtx.Unlock()

	go func() {
		minuteTicker := time.NewTicker(time.Minute)
		defer minuteTicker.Stop()
		startTime := time.Now()
		wallStartTime := startTime.Round(0)
		lastTime := startTime
		lastCPUTimes := make([]cpuTime, 6)

		defer func() {
			cmtx.Lock()
			if cs != nil && cs[0x12131400] != nil {
				elapsed := time.Since(startTime).Truncate(time.Millisecond)
				elapsedWall := time.Now().Round(0).Sub(wallStartTime).Truncate(time.Millisecond)
				cc := cs[0x12131400]
				cc.log.Infof("CmdResultLoop: finishing "+
					"goroutine for pid %d id %d after %s (wall %s)",
					os.Getpid(), id, elapsed, elapsedWall)
			}
			cmtx.Unlock()
		}()

		for {
			var r *CmdResult
			select {
			case r = <-cmdResultChan:
			case <-minuteTicker.C:
				// This is being used to debug background issues
				// on mobile. It may be removed in the future.
				go reportCmdResultLoop(startTime, lastTime, id, lastCPUTimes)
				lastTime = time.Now()
				continue

			case <-ch:
				return
			}

			// Process the special commands that toggle calling
			// native code with background ntfn events.
			switch r.Type {
			case CTEnableBackgroundNtfs:
				deliverBackgroundNtfns = true
				continue
			case CTDisableBackgroundNtfs:
				deliverBackgroundNtfns = false
				continue
			}

			// If the flutter engine is attached to the process,
			// deliver the event so that it can be processed.
			if !onlyBgNtfns {
				var errMsg, payload string
				if r.Err != nil {
					errMsg = r.Err.Error()
				}
				if len(r.Payload) > 0 {
					payload = string(r.Payload)
				}
				cb.F(r.ID, r.Type, payload, errMsg)
			}

			// Emit a background ntfn if the flutter engine is
			// deatched or if it is attached but paused/on
			// background.
			if deliverBackgroundNtfns {
				emitBackgroundNtfns(r, cb)
			}
		}
	}()

	return id
}

// StopCmdResultLoop stops an async goroutine created with CmdResultLoop. Does
// nothing if this goroutine is already stopped.
func StopCmdResultLoop(id int32) {
	cmdResultLoopsMtx.Lock()
	ch := cmdResultLoops[id]
	delete(cmdResultLoops, id)
	cmdResultLoopsLive.Add(-1)
	cmdResultLoopsMtx.Unlock()
	if ch != nil {
		close(ch)
	}
}

// StopAllCmdResultLoops stops all async goroutines created by CmdResultLoop.
func StopAllCmdResultLoops() {
	cmdResultLoopsMtx.Lock()
	chans := cmdResultLoops
	cmdResultLoops = map[int32]chan struct{}{}
	cmdResultLoopsLive.Store(0)
	cmdResultLoopsMtx.Unlock()
	for _, ch := range chans {
		close(ch)
	}
}

// ClientExists returns true if the client with the specified handle is running.
func ClientExists(handle int32) bool {
	cmtx.Lock()
	exists := cs != nil && cs[uint32(handle)] != nil
	cmtx.Unlock()
	return exists
}

func LogInfo(id int32, s string) {
	cmtx.Lock()
	if cs != nil && cs[uint32(id)] != nil {
		cs[uint32(id)].log.Info(s)
	} else {
		fmt.Println(s)
	}
	cmtx.Unlock()
}
