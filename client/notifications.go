package client

import (
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/mdembeds"
	"github.com/companyzero/bisonrelay/internal/strescape"
	"github.com/companyzero/bisonrelay/ratchet"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
)

// Following are the notification types. Add new types at the bottom of this
// list, then add a notifyX() to NotificationManager and initialize a new
// container in NewNotificationManager().

const onPMNtfnType = "onPM"

// OnPMNtfn is the handler for received private messages.
type OnPMNtfn func(*RemoteUser, rpc.RMPrivateMessage, time.Time)

func (OnPMNtfn) typ() string { return onPMNtfnType }

const onGCMNtfnType = "onGCM"

// OnGCMNtfn is the handler for received gc messages.
type OnGCMNtfn func(*RemoteUser, rpc.RMGroupMessage, time.Time)

func (OnGCMNtfn) typ() string { return onGCMNtfnType }

const onPostRcvdNtfnType = "onPostRcvd"

// OnPostRcvdNtfn is the handler for received posts.
type OnPostRcvdNtfn func(*RemoteUser, clientdb.PostSummary, rpc.PostMetadata)

func (OnPostRcvdNtfn) typ() string { return onPostRcvdNtfnType }

const onPostStatusRcvdNtfnType = "onPostStatusRcvd"

// OnPostStatusRcvdNtfn is the handler for received post status updates.
type OnPostStatusRcvdNtfn func(*RemoteUser, clientintf.PostID, UserID,
	rpc.PostMetadataStatus)

func (OnPostStatusRcvdNtfn) typ() string { return onPostStatusRcvdNtfnType }

const onRemoteSubscriptionChangedType = "onSubChanged"

// OnRemoteSubscriptionChanged is the handler for a remote user subscription
// changed event.
type OnRemoteSubscriptionChangedNtfn func(*RemoteUser, bool)

func (OnRemoteSubscriptionChangedNtfn) typ() string { return onRemoteSubscriptionChangedType }

const onRemoteSubscriptionErrorNtfnType = "onSubChangedErr"

// OnRemoteSubscriptionErrorNtfn is the handler for a remote user subscription
// change attempt that errored.
type OnRemoteSubscriptionErrorNtfn func(user *RemoteUser, wasSubscribing bool, errMsg string)

func (OnRemoteSubscriptionErrorNtfn) typ() string { return onRemoteSubscriptionErrorNtfnType }

const onLocalClientOfflineTooLong = "onLocalClientOfflineTooLong"

// OnLocalClientOfflineTooLong is called after the local client connects to the
// server, if it determines it has been offline for too long given the server's
// message retention policy.
type OnLocalClientOfflineTooLong func(time.Time)

func (OnLocalClientOfflineTooLong) typ() string { return onLocalClientOfflineTooLong }

const onKXCompleted = "onKXCompleted"

// OnKXCompleted is called after KX is completed with a remote user (either a
// new user or a reset KX).
type OnKXCompleted func(*clientintf.RawRVID, *RemoteUser, bool)

func (OnKXCompleted) typ() string { return onKXCompleted }

const onKXSuggested = "onKXSuggested"

// OnKXSuggested is called after a remote user suggests that this user should KX
// with another remote user.
type OnKXSuggested func(*RemoteUser, zkidentity.PublicIdentity)

func (OnKXSuggested) typ() string { return onKXSuggested }

const onInvoiceGenFailedNtfnType = "onInvoiceGenFailed"

type OnInvoiceGenFailedNtfn func(user *RemoteUser, dcrAmount float64, err error)

func (OnInvoiceGenFailedNtfn) typ() string { return onInvoiceGenFailedNtfnType }

const onGCVersionWarningType = "onGCVersionWarn"

// OnGCVersionWarning is a handler for warnings about a GC that has an
// unsupported version.
type OnGCVersionWarning func(user *RemoteUser, gc rpc.RMGroupList, minVersion, maxVersion uint8)

func (OnGCVersionWarning) typ() string { return onGCVersionWarningType }

const onJoinedGCNtfnType = "onJoinedGC"

// OnJoinedGCNtfn is a handler for when the local client joins a GC.
type OnJoinedGCNtfn func(gc rpc.RMGroupList)

func (OnJoinedGCNtfn) typ() string { return onJoinedGCNtfnType }

const onAddedGCMembersNtfnType = "onAddedGCMembers"

// OnAddedGCMembersNtfn is a handler for new members added to a GC.
type OnAddedGCMembersNtfn func(gc rpc.RMGroupList, uids []clientintf.UserID)

func (OnAddedGCMembersNtfn) typ() string { return onAddedGCMembersNtfnType }

const onRemovedGCMembersNtfnType = "onRemovedGCMembers"

// OnRemovedGCMembersNtfn is a handler for members removed from a GC.
type OnRemovedGCMembersNtfn func(gc rpc.RMGroupList, uids []clientintf.UserID)

func (OnRemovedGCMembersNtfn) typ() string { return onRemovedGCMembersNtfnType }

const onGCUpgradedNtfnType = "onGCUpgraded"

// OnGCUpgradedNtfn is a handler for a GC that had its version upgraded.
type OnGCUpgradedNtfn func(gc rpc.RMGroupList, oldVersion uint8)

func (OnGCUpgradedNtfn) typ() string { return onGCUpgradedNtfnType }

const onInvitedToGCNtfnType = "onInvitedToGC"

// OnInvitedToGCNtfn is a handler for invites received to join GCs.
type OnInvitedToGCNtfn func(user *RemoteUser, iid uint64, invite rpc.RMGroupInvite)

func (OnInvitedToGCNtfn) typ() string { return onInvitedToGCNtfnType }

const onGCInviteAcceptedNtfnType = "onGCInviteAccepted"

// OnGCInviteAcceptedNtfn is a handler for invites accepted by remote users to
// join a GC we invited them to.
type OnGCInviteAcceptedNtfn func(user *RemoteUser, gc rpc.RMGroupList)

func (OnGCInviteAcceptedNtfn) typ() string { return onGCInviteAcceptedNtfnType }

const onGCUserPartedNtfnType = "onGCUserParted"

// OnGCUserPartedNtfn is a handler when a user parted from a GC or an admin
// kicked a user.
type OnGCUserPartedNtfn func(gcid GCID, uid UserID, reason string, kicked bool)

func (OnGCUserPartedNtfn) typ() string { return onGCUserPartedNtfnType }

const onGCKilledNtfnType = "onGCKilled"

// OnGCKilledNtfn is a handler for a GC dissolved by its admin.
type OnGCKilledNtfn func(ru *RemoteUser, gcid GCID, reason string)

func (OnGCKilledNtfn) typ() string { return onGCKilledNtfnType }

const onGCAdminsChangedNtfnType = "onGCAdminsChanged"

type OnGCAdminsChangedNtfn func(ru *RemoteUser, gc rpc.RMGroupList, added, removed []zkidentity.ShortID)

func (OnGCAdminsChangedNtfn) typ() string { return onGCAdminsChangedNtfnType }

const onKXSearchCompletedNtfnType = "kxSearchCompleted"

// OnKXSearchCompleted is a handler for completed KX search procedures.
type OnKXSearchCompleted func(user *RemoteUser)

func (OnKXSearchCompleted) typ() string { return onKXSearchCompletedNtfnType }

const onTipAttemptProgressNtfnType = "onTipAttemptProgress"

type OnTipAttemptProgressNtfn func(ru *RemoteUser, amtMAtoms int64, completed bool, attempt int, attemptErr error, willRetry bool)

func (OnTipAttemptProgressNtfn) typ() string { return onTipAttemptProgressNtfnType }

const onBlockNtfnType = "onBlock"

// OnBlockNtfn is called when we blocked the specified user due to their
// request. Note that the passed user cannot be used for messaging anymore.
type OnBlockNtfn func(user *RemoteUser)

func (OnBlockNtfn) typ() string { return onBlockNtfnType }

const onServerSessionChangedNtfnType = "onServerSessionChanged"

// OnServerSessionChangedNtfn is called indicating that the connection to the
// server changed to the specified state (either connected or not).
//
// The push and subscription rates are specified in milliatoms/byte.
type OnServerSessionChangedNtfn func(connected bool, policy clientintf.ServerPolicy)

func (OnServerSessionChangedNtfn) typ() string { return onServerSessionChangedNtfnType }

const onOnboardStateChangedNtfnType = "onOnboardStateChanged"

type OnOnboardStateChangedNtfn func(state clientintf.OnboardState, err error)

func (OnOnboardStateChangedNtfn) typ() string { return onOnboardStateChangedNtfnType }

const onResourceFetchedNtfnType = "onResourceFetched"

// OnResourceFetchedNtfn is called when a reply to a fetched resource is
// received.
//
// Note that the user may be nil if the resource was fetched locally, such as
// through the FetchLocalResource call.
type OnResourceFetchedNtfn func(ru *RemoteUser, fr clientdb.FetchedResource, sess clientdb.PageSessionOverview)

func (OnResourceFetchedNtfn) typ() string { return onResourceFetchedNtfnType }

const onTipUserInvoiceGeneratedNtfnType = "onTipUserInvoiceGenerated"

// OnTipUserInvoiceGeneratedNtfn is called when the local client generates an
// invoice to send to a remote user, for tipping purposes.
type OnTipUserInvoiceGeneratedNtfn func(ru *RemoteUser, tag uint32, invoice string)

func (OnTipUserInvoiceGeneratedNtfn) typ() string { return onTipUserInvoiceGeneratedNtfnType }

const onHandshakeStageNtfnType = "onHandshakeStage"

// OnHandshakeStageNtfn is called during a 3-way handshake with a remote client.
// mstype may be SYN, SYNACK or ACK. The SYNACK and ACK types allow the
// respective clients to infer that the ratchet operations are working.
type OnHandshakeStageNtfn func(ru *RemoteUser, msgtype string)

func (OnHandshakeStageNtfn) typ() string { return onHandshakeStageNtfnType }

const onGCWithUnkxdMemberNtfnType = "onGCWithUnkxdMember"

// OnGCWithUnkxdMemberNtfn is called when attempting to send a message to a
// GC in which there are members that the local client hasn't KX'd with.
type OnGCWithUnkxdMemberNtfn func(gc zkidentity.ShortID, uid clientintf.UserID,
	hasKX, hasMI bool, miCount uint32, startedMIMediator *clientintf.UserID)

func (OnGCWithUnkxdMemberNtfn) typ() string { return onGCWithUnkxdMemberNtfnType }

const onTipReceivedNtfnType = "onTipReceived"

// OnTipReceivedNtfn is called when a tip is received from a remote user.
type OnTipReceivedNtfn func(ru *RemoteUser, amountMAtoms int64)

func (OnTipReceivedNtfn) typ() string { return onTipReceivedNtfnType }

const onMessageContentFilteredNtfType = "onMsgContentFiltered"

// MsgContentFilteredEvent is the data for a message content filter event.
type MsgContentFilteredEvent struct {
	UID           UserID
	GC            *zkidentity.ShortID
	PID           *clientintf.PostID
	PostFrom      *clientintf.UserID
	IsPostComment bool
	Msg           string
	Rule          clientdb.ContentFilter
}

// OnMsgContentFilteredNtfn is called when a message was filtered due to its
// contents.
type OnMsgContentFilteredNtfn func(MsgContentFilteredEvent)

func (OnMsgContentFilteredNtfn) typ() string { return onMessageContentFilteredNtfType }

const onPostSubscriberUpdated = "onPostSubscriberUpdated"

// OnPostSubscriberUpdated is called when a remote user changes its subscription
// status to the local client's posts (i.e. the remote user subscribed or
// unsubscribed to the local client's posts).
type OnPostSubscriberUpdated func(user *RemoteUser, subscribed bool)

func (OnPostSubscriberUpdated) typ() string { return onPostSubscriberUpdated }

const onPostsListReceived = "onPostsListReceived"

// PostsListReceived is called when the local client receives the list of posts
// from a remote user.
type OnPostsListReceived func(user *RemoteUser, postList rpc.RMListPostsReply)

func (OnPostsListReceived) typ() string { return onPostsListReceived }

const onUnsubscribingIdleRemoteClient = "onUnsubscribingIdleRemoteClient"

// OnUnsubscribingIdleRemoteClient is a notification sent when a remote client
// is detected as idle and being unsubscribed from GCs and posts.
type OnUnsubscribingIdleRemoteClient func(user *RemoteUser, lastDecTime time.Time)

func (OnUnsubscribingIdleRemoteClient) typ() string { return onUnsubscribingIdleRemoteClient }

const onReceiveReceipt = "onReceiveReceipt"

// OnReceiveReceipt is a notification sent when a remote client sends a
// receive receipt.
type OnReceiveReceipt func(user *RemoteUser, rr rpc.RMReceiveReceipt, serverTime time.Time)

func (OnReceiveReceipt) typ() string { return onReceiveReceipt }

const onContentListReceived = "onContentListReceived"

// ContentListReceived is called when the list of content of the user is
// received.
type OnContentListReceived func(user *RemoteUser, files []clientdb.RemoteFile, listErr error)

func (OnContentListReceived) typ() string { return onContentListReceived }

const onFileDownloadCompleted = "onFileDownloadCompleted"

// FileDownloadCompleted is called whenever a download of a file has
// completed.
type OnFileDownloadCompleted func(user *RemoteUser, fm rpc.FileMetadata, diskPath string)

func (OnFileDownloadCompleted) typ() string { return onFileDownloadCompleted }

const onFileDownloadProgress = "onFileDownloadProgress"

// FileDownloadProgress is called reporting the progress of a file
// download process.
type OnFileDownloadProgress func(user *RemoteUser, fm rpc.FileMetadata, nbMissingChunks int)

func (OnFileDownloadProgress) typ() string { return onFileDownloadProgress }

const onRMReceived = "onRMReceived"

// OnRMReceived is a notification sent whenever a remote user receives an RM.
// Note: this is called _before_ the RM has been processed, therefore care must
// be taken when hooking and handling this notification.
type OnRMReceived func(ru *RemoteUser, h *rpc.RMHeader, p interface{}, ts time.Time)

func (OnRMReceived) typ() string { return onRMReceived }

const onRMQueuedNtfnType = "onRMQueued"

// OnRMQueued is a notification sent whenever a user RM has been queued in the
// RMQ for outbound sending to the server. This is NOT when the message has been
// sent to the server, but rather when the RMQ has acknowledged that the message
// has been added to the outbound queue to be sent.
type OnRMQueued func(ru *RemoteUser, p interface{})

func (OnRMQueued) typ() string { return onRMQueuedNtfnType }

const onRMSent = "onRMSent"

// OnRMSent is a notification sent whenever a message has been delivered to
// the server directed to a remote user. Note: this is called _after_ the RM
// has been acknowledged by the server.
type OnRMSent func(ru *RemoteUser, rv ratchet.RVPoint, p interface{})

func (OnRMSent) typ() string { return onRMSent }

const onServerUnwelcomeError = "onServerUnwelcomeError"

const onUnackedRMSentNtfnType = "onUnackedRMSent"

// OnUnackedRMSent is a notification sent when a previously unacked RM was
// resent to the server.
type OnUnackedRMSent func(uid clientintf.UserID, rv ratchet.RVPoint)

func (OnUnackedRMSent) typ() string { return onUnackedRMSentNtfnType }

// OnServerUnwelcomeError is a notification sent, when attempting to connect
// to a server, the client receives an error that hints that it should
// upgrade.
type OnServerUnwelcomeError func(err error)

func (OnServerUnwelcomeError) typ() string { return onServerUnwelcomeError }

// ProfileUpdateField tracks profile fields which may be updated.
type ProfileUpdateField string

const (
	// ProfileUpdateAvatar is the profile field that corresponds to the
	// user's avatar.
	ProfileUpdateAvatar ProfileUpdateField = "avatar"
)

const onProfileUpdatedType = "onProfileChanged"

// OnProfileChanged is a notification sent whenever a remote client has updated
// its profile.
type OnProfileUpdated func(ru *RemoteUser, ab *clientdb.AddressBookEntry, fields []ProfileUpdateField)

func (OnProfileUpdated) typ() string { return onProfileUpdatedType }

const onTransitiveEventType = "onTransitiveEvent"

// OnTransitiveEvent is called whenever a request is made by source for the
// local client to forward a message to dst.
type OnTransitiveEvent func(src, dst UserID, event TransitiveEvent)

func (OnTransitiveEvent) typ() string { return onTransitiveEventType }

const onRequestingMediateIDType = "onReqMediateID"

// OnRequestingMediateID is called whenever an autokx attempt is requesting a
// mediator to mediate id between the local client and a target.
type OnRequestingMediateID func(mediator, target UserID)

func (OnRequestingMediateID) typ() string { return onRequestingMediateIDType }

// UINotificationsConfig is the configuration for how UI notifications are
// emitted.
type UINotificationsConfig struct {
	// PMs flags whether to emit notification for PMs.
	PMs bool

	// GCMs flags whether to emit notifications for GCMs.
	GCMs bool

	// GCMMentions flags whether to emit notification for mentions.
	GCMMentions bool

	// MaxLength is the max length of messages emitted.
	MaxLength int

	// MentionRegexp is the regexp to detect mentions.
	MentionRegexp *regexp.Regexp

	// EmitInterval is the interval to wait for additional messages before
	// emitting a notification. Multiple messages received within this
	// interval will only generate a single UI notification.
	EmitInterval time.Duration

	// CancelEmissionChannel may be set to a Context.Done() channel to
	// cancel emission of notifications.
	CancelEmissionChannel <-chan struct{}
}

func (cfg *UINotificationsConfig) clip(msg string) string {
	if len(msg) < cfg.MaxLength {
		return msg
	}
	return msg[:cfg.MaxLength]
}

// UINotificationType is the type of notification.
type UINotificationType string

const (
	UINtfnPM         UINotificationType = "pm"
	UINtfnGCM        UINotificationType = "gcm"
	UINtfnGCMMention UINotificationType = "gcmmention"
	UINtfnMultiple   UINotificationType = "multiple"
)

// UINotification is a notification that should be shown as an UI alert.
type UINotification struct {
	// Type of notification.
	Type UINotificationType `json:"type"`

	// Text of the notification.
	Text string `json:"text"`

	// Count will be greater than one when multiple notifications were
	// batched.
	Count int `json:"count"`

	// From is the original sender or GC of the notification.
	From zkidentity.ShortID `json:"from"`

	// FromNick is the nick of the sender.
	FromNick string `json:"from_nick"`

	// Timestamp is the unix timestamp in seconds of the first message.
	Timestamp int64 `json:"timestamp"`
}

// fromSame returns true if the notification is from the same ID.
func (n *UINotification) fromSame(id *zkidentity.ShortID) bool {
	if id == nil || n.From.IsEmpty() {
		return false
	}

	return *id == n.From
}

const onUINtfnType = "uintfn"

// OnUINotification is called when a notification should be shown by the UI to
// the user. This should usually take the form of an alert dialog about a
// received message.
type OnUINotification func(ntfn UINotification)

func (OnUINotification) typ() string { return onUINtfnType }

const onInvitedToRTDTSessionNtfnType = "invitedRTDTSess"

// OnInvitedToRTDTSession is a notification sent when the local client is
// invited to an RTDT session.
type OnInvitedToRTDTSession func(ru *RemoteUser, sess *rpc.RMRTDTSessionInvite, ts time.Time)

func (OnInvitedToRTDTSession) typ() string { return onInvitedToRTDTSessionNtfnType }

const onRTDTSessionInviteAcceptedNtfnType = "rtdtSessionInviteAccepted"

// OnRTDTSessionInviteAccepted is a notification sent when a remote client has
// accepted our invitation to join an RTDT session.
type OnRTDTSessionInviteAccepted func(ru *RemoteUser, sessID zkidentity.ShortID, acceptedAsPublisher bool)

func (OnRTDTSessionInviteAccepted) typ() string { return onRTDTSessionInviteAcceptedNtfnType }

const onRTDTSessionInviteCanceledNtfnType = "rtdtSessionInviteCanceled"

// OnRTDTSessionInviteCanceled is a notification sent when a remote client has
// canceled/denied our invitation to join an RTDT session.
type OnRTDTSessionInviteCanceled func(ru *RemoteUser, sessID zkidentity.ShortID)

func (OnRTDTSessionInviteCanceled) typ() string { return onRTDTSessionInviteCanceledNtfnType }

// RTDTSessionUpdateNtfn is the data of one update to an RTDT session.
type RTDTSessionUpdateNtfn struct {
	SessionRV         zkidentity.ShortID            `json:"session_rv"`
	NewPublishers     []*rpc.RMRTDTSessionPublisher `json:"new_publishers"`
	NewMetadata       *rpc.RMRTDTSession            `json:"new_metadata"`
	InitialJoin       bool                          `json:"initial_join"`
	RemovedPublishers []*rpc.RMRTDTSessionPublisher `json:"removed_publishers"`
}

const onRTDTSesssionUpdatedNtfnType = "rtdtSessionUpdated"

// OnRTDTSesssionUpdated is a notification sent when the owner or admin of an
// RTDT session updated its metadata.
type OnRTDTSesssionUpdated func(ru *RemoteUser, update *RTDTSessionUpdateNtfn)

func (OnRTDTSesssionUpdated) typ() string { return onRTDTSesssionUpdatedNtfnType }

const onRTDTLiveSessionJoinedNtfnType = "rtdtlivesessjoined"

// OnRTDTLiveSessionJoined is a notification sent when the local client joins a
// live RTDT session.
type OnRTDTLiveSessionJoined func(sessRV zkidentity.ShortID)

func (OnRTDTLiveSessionJoined) typ() string { return onRTDTLiveSessionJoinedNtfnType }

const onRTDTRefreshedSessionAllowance = "rtdtrefreshsessallowance"

// OnRTDTRefreshedSessionAllowance is a notification sent when the local client
// has refreshed its allowance to publish data in a live RTDT session.
type OnRTDTRefreshedSessionAllowance func(sessRV zkidentity.ShortID, addAllowance uint64)

func (OnRTDTRefreshedSessionAllowance) typ() string { return onRTDTRefreshedSessionAllowance }

const onRTDTLivePeerJoinedNtfnType = "rtdtlivepeerjoined"

// OnRTDTLivePeerJoined is a notification sent when a remote peer has joined a
// live RTDT session.
type OnRTDTLivePeerJoined func(sessRV zkidentity.ShortID, peerID rpc.RTDTPeerID)

func (OnRTDTLivePeerJoined) typ() string { return onRTDTLivePeerJoinedNtfnType }

const onRTDTLivePeerStalledNtfnType = "rtdtlivepeerstalled"

// OnRTDTLivePeerStalled is a notification sent when a remote peer has stopped sending
// live RTDT session data.
type OnRTDTLivePeerStalled func(sessRV zkidentity.ShortID, peerID rpc.RTDTPeerID)

func (OnRTDTLivePeerStalled) typ() string { return onRTDTLivePeerStalledNtfnType }

const onRTDTLiveSendErroredNtfnType = "rtdtlivesenderrored"

// OnRTDTLiveSessionSendErrored is called when sending data to a live RTDT session
// errored. This moves hot audio status from the session.
type OnRTDTLiveSessionSendErrored func(sessRV zkidentity.ShortID, err error)

func (OnRTDTLiveSessionSendErrored) typ() string { return onRTDTLiveSendErroredNtfnType }

const onRTDTRemadeLiveSessionHot = "rtdtremadehotsesss"

// OnRTDTRemadeLiveSessionHotAudio is called when a session that had previously
// errored was made hot again.
//
// This usually happens on transient network errors or when changing networks.
type OnRTDTRemadeLiveSessionHotAudio func(sessRV zkidentity.ShortID)

func (OnRTDTRemadeLiveSessionHotAudio) typ() string { return onRTDTRemadeLiveSessionHot }

const onRTDTPeerSoundChangedNtfnType = "rtdtpeersoundchanged"

// OnRTDTPeerSoundChanged is a notification sent when sound state changes
// (starts/ends) for a given peer in a live RTDT session.
type OnRTDTPeerSoundChanged func(sessRV zkidentity.ShortID, peerID rpc.RTDTPeerID, hasSoundStream, hasSound bool)

func (OnRTDTPeerSoundChanged) typ() string { return onRTDTPeerSoundChangedNtfnType }

const onRTDTPeerExitedSessionNtfnType = "rtdtpeerexitsess"

// OnRTDTPeerExitedSession is a notification sent when the local client admins
// a RTDT session and a remote peer asked to permanently exit it.
type OnRTDTPeerExitedSession func(ru *RemoteUser, sessRV zkidentity.ShortID, peerID rpc.RTDTPeerID)

func (OnRTDTPeerExitedSession) typ() string { return onRTDTPeerExitedSessionNtfnType }

const onRTDTKickedFromLiveSessNtfnType = "rtdtkickedfromlive"

// OnRTDTKickedFromLiveSession is called when the local client was kicked
// from a live RTDT session.
type OnRTDTKickedFromLiveSession func(sessRV zkidentity.ShortID, peerID rpc.RTDTPeerID, banDuration time.Duration)

func (OnRTDTKickedFromLiveSession) typ() string { return onRTDTKickedFromLiveSessNtfnType }

const onRTDTSessionDissolvedNtfnType = "rtdtsessiondissolved"

// OnRTDTSessionDissolved is a notification sent when an RTDT session is dissolved.
type OnRTDTSessionDissolved func(ru *RemoteUser, sessRV zkidentity.ShortID, peerID rpc.RTDTPeerID)

func (OnRTDTSessionDissolved) typ() string { return onRTDTSessionDissolvedNtfnType }

const onRTDTRemovedFromSessionNtfnType = "rtdtremovedfromsess"

// OnRTDTRemovedFromSession is called when the local client was removed from
// the list of members of an RTDT session.
type OnRTDTRemovedFromSession func(ru *RemoteUser, sessRV zkidentity.ShortID, reason string)

func (OnRTDTRemovedFromSession) typ() string { return onRTDTRemovedFromSessionNtfnType }

const onRTDTRotatedCookieNtfnType = "rtdtrotcookie"

type OnRTDTRotatedCookie func(ru *RemoteUser, sessRV zkidentity.ShortID)

func (OnRTDTRotatedCookie) typ() string { return onRTDTRotatedCookieNtfnType }

const onRTDTChatMsgRcvdNtfnType = "rtdtchatmsgreceived"

// OnRTDTChatMessageReceived is called when the local client receives a chat
// message in an RTDT session.
type OnRTDTChatMessageReceived func(sessionRV zkidentity.ShortID, pub rpc.RMRTDTSessionPublisher, msg string, ts uint32)

func (OnRTDTChatMessageReceived) typ() string { return onRTDTChatMsgRcvdNtfnType }

const onRTDTAdminCookiesRcvdNtfnType = "rtdtadmincookiesrcv"

// OnRTDTAdminCookiesReceived is called when the local client receives admin
// cookies from another admin in an RTDT session.
type OnRTDTAdminCookiesReceived func(ru *RemoteUser, sessionRV zkidentity.ShortID)

func (OnRTDTAdminCookiesReceived) typ() string { return onRTDTAdminCookiesRcvdNtfnType }

const onRTDTRTTCalculatedNtfnType = "rtdtrttcalc"

type OnRTDTRTTCalculated func(addr net.UDPAddr, rtt time.Duration)

func (OnRTDTRTTCalculated) typ() string { return onRTDTRTTCalculatedNtfnType }

const onRTDTJoinedInstantCallNtfnType = "rtdtjoinedinstantcall"

// OnRTDTJoinedInstantCall is called when an instant RTDT session is joined/made
// live/hot.
type OnRTDTJoinedInstantCall func(sessionRV zkidentity.ShortID)

func (OnRTDTJoinedInstantCall) typ() string { return onRTDTJoinedInstantCallNtfnType }

// The following is used only in tests.

const onTestNtfnType = "testNtfnType"

type onTestNtfn func()

func (onTestNtfn) typ() string { return onTestNtfnType }

// Following is the generic notification code.

type NotificationRegistration struct {
	unreg func() bool
}

func (reg NotificationRegistration) Unregister() bool {
	return reg.unreg()
}

type NotificationHandler interface {
	typ() string
}

type handler[T any] struct {
	handler T
	async   bool
}

type handlersFor[T any] struct {
	mtx      sync.Mutex
	next     uint
	handlers map[uint]handler[T]
}

func (hn *handlersFor[T]) register(h T, async bool) NotificationRegistration {
	var id uint

	hn.mtx.Lock()
	id, hn.next = hn.next, hn.next+1
	if hn.handlers == nil {
		hn.handlers = make(map[uint]handler[T])
	}
	hn.handlers[id] = handler[T]{handler: h, async: async}
	registered := true
	hn.mtx.Unlock()

	return NotificationRegistration{
		unreg: func() bool {
			hn.mtx.Lock()
			res := registered
			if registered {
				delete(hn.handlers, id)
				registered = false
			}
			hn.mtx.Unlock()
			return res
		},
	}
}

func (hn *handlersFor[T]) visit(f func(T)) {
	hn.mtx.Lock()
	for _, h := range hn.handlers {
		if h.async {
			go f(h.handler)
		} else {
			f(h.handler)
		}
	}
	hn.mtx.Unlock()
}

func (hn *handlersFor[T]) Register(v interface{}, async bool) NotificationRegistration {
	if h, ok := v.(T); !ok {
		panic("wrong type")
	} else {
		return hn.register(h, async)
	}
}

func (hn *handlersFor[T]) AnyRegistered() bool {
	hn.mtx.Lock()
	res := len(hn.handlers) > 0
	hn.mtx.Unlock()
	return res
}

type handlersRegistry interface {
	Register(v interface{}, async bool) NotificationRegistration
	AnyRegistered() bool
}

type NotificationManager struct {
	handlers map[string]handlersRegistry

	uiMtx      sync.Mutex
	uiConfig   UINotificationsConfig
	uiNextNtfn UINotification
	uiTimer    *time.Timer
}

// UpdateUIConfig updates the config used to generate UI notifications about
// PMs, GCMs, etc.
func (nmgr *NotificationManager) UpdateUIConfig(cfg UINotificationsConfig) {
	nmgr.uiMtx.Lock()
	nmgr.uiConfig = cfg
	nmgr.uiMtx.Unlock()
}

func (nmgr *NotificationManager) register(handler NotificationHandler, async bool) NotificationRegistration {
	handlers := nmgr.handlers[handler.typ()]
	if handlers == nil {
		panic(fmt.Sprintf("forgot to init the handler type %T "+
			"in NewNotificationManager", handler))
	}

	return handlers.Register(handler, async)
}

// Register registers a callback notification function that is called
// asynchronously to the event (i.e. in a separate goroutine).
func (nmgr *NotificationManager) Register(handler NotificationHandler) NotificationRegistration {
	return nmgr.register(handler, true)
}

// RegisterSync registers a callback notification function that is called
// synchronously to the event. This callback SHOULD return as soon as possible,
// otherwise the client might hang.
//
// Synchronous callbacks are mostly intended for tests and when external
// callers need to ensure proper order of multiple sequential events. In
// general it is preferable to use callbacks registered with the Register call,
// to ensure the client will not deadlock or hang.
func (nmgr *NotificationManager) RegisterSync(handler NotificationHandler) NotificationRegistration {
	return nmgr.register(handler, false)
}

// AnyRegistered returns true if there are any handlers registered for the given
// handler type.
func (ngmr *NotificationManager) AnyRegistered(handler NotificationHandler) bool {
	return ngmr.handlers[handler.typ()].AnyRegistered()
}

func (nmgr *NotificationManager) waitAndEmitUINtfn(c <-chan time.Time, cancel <-chan struct{}) {
	select {
	case <-c:
	case <-cancel:
		return
	}

	nmgr.uiMtx.Lock()
	n := nmgr.uiNextNtfn
	nmgr.uiNextNtfn = UINotification{}
	nmgr.uiMtx.Unlock()

	nmgr.handlers[onUINtfnType].(*handlersFor[OnUINotification]).
		visit(func(h OnUINotification) { h(n) })
}

func (nmgr *NotificationManager) addUINtfn(from zkidentity.ShortID, fromNick string, typ UINotificationType, msg string, ts time.Time) {
	nmgr.uiMtx.Lock()

	n := &nmgr.uiNextNtfn
	cfg := &nmgr.uiConfig

	// Remove embeds.
	msg = mdembeds.ReplaceEmbeds(msg, func(args mdembeds.EmbeddedArgs) string {
		if strings.HasPrefix(args.Typ, "image/") {
			return "[image]"
		}
		return ""
	})

	// Check if it has mention.
	if typ == UINtfnGCM && cfg.MentionRegexp != nil && cfg.MentionRegexp.MatchString(msg) {
		typ = UINtfnGCMMention
	}

	switch {
	case typ == UINtfnPM && !cfg.PMs,
		typ == UINtfnGCM && !cfg.GCMs,
		typ == UINtfnGCMMention && !cfg.GCMMentions:

		// Ignore
		nmgr.uiMtx.Unlock()
		return

	case typ == UINtfnPM && n.Type == "":
		// First PM.
		n.Type = typ
		n.Count = 1
		n.From = from
		n.FromNick = fromNick
		n.Timestamp = ts.Unix()
		n.Text = fmt.Sprintf("PM from %s: %s", strescape.Nick(fromNick),
			cfg.clip(msg))

	case typ == UINtfnPM && n.Type == UINtfnPM && n.fromSame(&from):
		// Additional PM from same user.
		n.Count += 1
		n.Text = fmt.Sprintf("%d PMs from %s", n.Count, strescape.Nick(fromNick))

	case typ == UINtfnPM && n.Type == UINtfnPM:
		// PMs from multiple users.
		n.Count += 1
		n.FromNick = "multiple"
		n.Text = fmt.Sprintf("%d PMs from multiple users", n.Count)

	case typ == UINtfnGCM && n.Type == "":
		// First GCM.
		n.Type = typ
		n.Count = 1
		n.From = from
		n.FromNick = fromNick
		n.Timestamp = ts.Unix()
		n.Text = fmt.Sprintf("GCM on %s: %s", strescape.Nick(fromNick),
			cfg.clip(msg))

	case typ == UINtfnGCMMention && n.Type == "":
		// First mention.
		n.Type = typ
		n.Count = 1
		n.From = from
		n.FromNick = fromNick
		n.Timestamp = ts.Unix()
		n.Text = fmt.Sprintf("Mention on GC %s: %s", strescape.Nick(fromNick),
			cfg.clip(msg))

	case (typ == UINtfnGCM || typ == UINtfnGCMMention) &&
		(n.Type == UINtfnGCM || n.Type == UINtfnGCMMention) &&
		n.fromSame(&from):

		// Additional GCM on same GC.
		n.Count += 1
		n.Text = fmt.Sprintf("%d GCMs on %s", n.Count, strescape.Nick(fromNick))

	case (typ == UINtfnGCM || typ == UINtfnGCMMention) &&
		(n.Type == UINtfnGCM || n.Type == UINtfnGCMMention):

		// GCMs on multiple GCs.
		n.FromNick = "multiple"
		n.Count += 1
		n.Text = fmt.Sprintf("%d GCMs on multiple GCs", n.Count)

	default:
		// Multiple types.
		n.Type = UINtfnMultiple
		n.FromNick = "multiple"
		n.Count += 1
		n.Text = fmt.Sprintf("%d messages received", n.Count)
	}

	// The first notification starts the timer to emit the actual UI
	// notification. Other notifications will get batched.
	if n.Count == 1 {
		nmgr.uiTimer.Reset(cfg.EmitInterval)
		c, cancel := nmgr.uiTimer.C, cfg.CancelEmissionChannel
		go nmgr.waitAndEmitUINtfn(c, cancel)
	}

	nmgr.uiMtx.Unlock()
}

// Following are the notifyX() calls (one for each type of notification).

func (nmgr *NotificationManager) notifyTest() {
	nmgr.handlers[onTestNtfnType].(*handlersFor[onTestNtfn]).
		visit(func(h onTestNtfn) { h() })
}

func (nmgr *NotificationManager) notifyOnPM(user *RemoteUser, pm rpc.RMPrivateMessage, ts time.Time) {
	nmgr.handlers[onPMNtfnType].(*handlersFor[OnPMNtfn]).
		visit(func(h OnPMNtfn) { h(user, pm, ts) })

	nmgr.addUINtfn(user.ID(), user.Nick(), UINtfnPM, pm.Message, ts)
}

func (nmgr *NotificationManager) notifyOnGCM(user *RemoteUser, gcm rpc.RMGroupMessage, gcAlias string, ts time.Time) {
	nmgr.handlers[onGCMNtfnType].(*handlersFor[OnGCMNtfn]).
		visit(func(h OnGCMNtfn) { h(user, gcm, ts) })

	nmgr.addUINtfn(gcm.ID, gcAlias, UINtfnGCM, gcm.Message, ts)
}

func (nmgr *NotificationManager) notifyOnPostRcvd(user *RemoteUser, summary clientdb.PostSummary, post rpc.PostMetadata) {
	nmgr.handlers[onPostRcvdNtfnType].(*handlersFor[OnPostRcvdNtfn]).
		visit(func(h OnPostRcvdNtfn) { h(user, summary, post) })
}

func (nmgr *NotificationManager) notifyOnPostStatusRcvd(user *RemoteUser, pid clientintf.PostID,
	statusFrom UserID, status rpc.PostMetadataStatus) {
	nmgr.handlers[onPostStatusRcvdNtfnType].(*handlersFor[OnPostStatusRcvdNtfn]).
		visit(func(h OnPostStatusRcvdNtfn) { h(user, pid, statusFrom, status) })
}

func (nmgr *NotificationManager) notifyOnRemoteSubChanged(user *RemoteUser, subscribed bool) {
	nmgr.handlers[onRemoteSubscriptionChangedType].(*handlersFor[OnRemoteSubscriptionChangedNtfn]).
		visit(func(h OnRemoteSubscriptionChangedNtfn) { h(user, subscribed) })
}

func (nmgr *NotificationManager) notifyOnRemoteSubErrored(user *RemoteUser, wasSubscribing bool, errMsg string) {
	nmgr.handlers[onRemoteSubscriptionErrorNtfnType].(*handlersFor[OnRemoteSubscriptionErrorNtfn]).
		visit(func(h OnRemoteSubscriptionErrorNtfn) { h(user, wasSubscribing, errMsg) })
}

func (nmgr *NotificationManager) notifyOnLocalClientOfflineTooLong(date time.Time) {
	nmgr.handlers[onLocalClientOfflineTooLong].(*handlersFor[OnLocalClientOfflineTooLong]).
		visit(func(h OnLocalClientOfflineTooLong) { h(date) })
}

func (nmgr *NotificationManager) notifyOnGCVersionWarning(user *RemoteUser, gc rpc.RMGroupList, minVersion, maxVersion uint8) {
	nmgr.handlers[onGCVersionWarningType].(*handlersFor[OnGCVersionWarning]).
		visit(func(h OnGCVersionWarning) { h(user, gc, minVersion, maxVersion) })
}

func (nmgr *NotificationManager) notifyOnKXCompleted(ir *clientintf.RawRVID, user *RemoteUser, isNew bool) {
	nmgr.handlers[onKXCompleted].(*handlersFor[OnKXCompleted]).
		visit(func(h OnKXCompleted) { h(ir, user, isNew) })
}

func (nmgr *NotificationManager) notifyOnKXSearchCompleted(user *RemoteUser) {
	nmgr.handlers[onKXSearchCompletedNtfnType].(*handlersFor[OnKXSearchCompleted]).
		visit(func(h OnKXSearchCompleted) { h(user) })
}

func (nmgr *NotificationManager) notifyOnKXSuggested(invitee *RemoteUser, target zkidentity.PublicIdentity) {
	nmgr.handlers[onKXSuggested].(*handlersFor[OnKXSuggested]).
		visit(func(h OnKXSuggested) { h(invitee, target) })
}

func (nmgr *NotificationManager) notifyInvoiceGenFailed(user *RemoteUser, dcrAmount float64, err error) {
	nmgr.handlers[onInvoiceGenFailedNtfnType].(*handlersFor[OnInvoiceGenFailedNtfn]).
		visit(func(h OnInvoiceGenFailedNtfn) { h(user, dcrAmount, err) })
}

func (nmgr *NotificationManager) notifyOnJoinedGC(gc rpc.RMGroupList) {
	nmgr.handlers[onJoinedGCNtfnType].(*handlersFor[OnJoinedGCNtfn]).
		visit(func(h OnJoinedGCNtfn) { h(gc) })
}

func (nmgr *NotificationManager) notifyOnAddedGCMembers(gc rpc.RMGroupList, uids []clientintf.UserID) {
	nmgr.handlers[onAddedGCMembersNtfnType].(*handlersFor[OnAddedGCMembersNtfn]).
		visit(func(h OnAddedGCMembersNtfn) { h(gc, uids) })
}

func (nmgr *NotificationManager) notifyOnRemovedGCMembers(gc rpc.RMGroupList, uids []clientintf.UserID) {
	nmgr.handlers[onRemovedGCMembersNtfnType].(*handlersFor[OnRemovedGCMembersNtfn]).
		visit(func(h OnRemovedGCMembersNtfn) { h(gc, uids) })
}

func (nmgr *NotificationManager) notifyOnGCUpgraded(gc rpc.RMGroupList, oldVersion uint8) {
	nmgr.handlers[onGCUpgradedNtfnType].(*handlersFor[OnGCUpgradedNtfn]).
		visit(func(h OnGCUpgradedNtfn) { h(gc, oldVersion) })
}

func (nmgr *NotificationManager) notifyInvitedToGC(user *RemoteUser, iid uint64, invite rpc.RMGroupInvite) {
	nmgr.handlers[onInvitedToGCNtfnType].(*handlersFor[OnInvitedToGCNtfn]).
		visit(func(h OnInvitedToGCNtfn) { h(user, iid, invite) })
}

func (nmgr *NotificationManager) notifyGCInviteAccepted(user *RemoteUser, gc rpc.RMGroupList) {
	nmgr.handlers[onGCInviteAcceptedNtfnType].(*handlersFor[OnGCInviteAcceptedNtfn]).
		visit(func(h OnGCInviteAcceptedNtfn) { h(user, gc) })
}

func (nmgr *NotificationManager) notifyGCUserParted(gcid GCID, uid UserID, reason string, kicked bool) {
	nmgr.handlers[onGCUserPartedNtfnType].(*handlersFor[OnGCUserPartedNtfn]).
		visit(func(h OnGCUserPartedNtfn) { h(gcid, uid, reason, kicked) })
}

func (nmgr *NotificationManager) notifyOnGCKilled(ru *RemoteUser, gcid GCID, reason string) {
	nmgr.handlers[onGCKilledNtfnType].(*handlersFor[OnGCKilledNtfn]).
		visit(func(h OnGCKilledNtfn) { h(ru, gcid, reason) })
}

func (nmgr *NotificationManager) notifyGCAdminsChanged(ru *RemoteUser, gc rpc.RMGroupList,
	added, removed []zkidentity.ShortID) {
	nmgr.handlers[onGCAdminsChangedNtfnType].(*handlersFor[OnGCAdminsChangedNtfn]).
		visit(func(h OnGCAdminsChangedNtfn) { h(ru, gc, added, removed) })
}

func (nmgr *NotificationManager) notifyTipAttemptProgress(ru *RemoteUser, amtMAtoms int64, completed bool, attempt int, attemptErr error, willRetry bool) {
	nmgr.handlers[onTipAttemptProgressNtfnType].(*handlersFor[OnTipAttemptProgressNtfn]).
		visit(func(h OnTipAttemptProgressNtfn) { h(ru, amtMAtoms, completed, attempt, attemptErr, willRetry) })
}

func (nmgr *NotificationManager) notifyTipUserInvoiceGenerated(ru *RemoteUser, tag uint32, invoice string) {
	nmgr.handlers[onTipUserInvoiceGeneratedNtfnType].(*handlersFor[OnTipUserInvoiceGeneratedNtfn]).
		visit(func(h OnTipUserInvoiceGeneratedNtfn) { h(ru, tag, invoice) })
}

func (nmgr *NotificationManager) notifyOnBlock(ru *RemoteUser) {
	nmgr.handlers[onBlockNtfnType].(*handlersFor[OnBlockNtfn]).
		visit(func(h OnBlockNtfn) { h(ru) })
}

func (nmgr *NotificationManager) notifyServerSessionChanged(connected bool, policy clientintf.ServerPolicy) {
	nmgr.handlers[onServerSessionChangedNtfnType].(*handlersFor[OnServerSessionChangedNtfn]).
		visit(func(h OnServerSessionChangedNtfn) { h(connected, policy) })
}

func (nmgr *NotificationManager) notifyOnOnboardStateChanged(state clientintf.OnboardState, err error) {
	nmgr.handlers[onOnboardStateChangedNtfnType].(*handlersFor[OnOnboardStateChangedNtfn]).
		visit(func(h OnOnboardStateChangedNtfn) { h(state, err) })
}

func (nmgr *NotificationManager) notifyResourceFetched(ru *RemoteUser,
	fr clientdb.FetchedResource, sess clientdb.PageSessionOverview) {
	nmgr.handlers[onResourceFetchedNtfnType].(*handlersFor[OnResourceFetchedNtfn]).
		visit(func(h OnResourceFetchedNtfn) { h(ru, fr, sess) })
}

func (nmgr *NotificationManager) notifyHandshakeStage(ru *RemoteUser, msgtype string) {
	nmgr.handlers[onHandshakeStageNtfnType].(*handlersFor[OnHandshakeStageNtfn]).
		visit(func(h OnHandshakeStageNtfn) { h(ru, msgtype) })
}

func (nmgr *NotificationManager) notifyGCWithUnxkdMember(gc zkidentity.ShortID, uid clientintf.UserID,
	hasKX, hasMI bool, miCount uint32, startedMIMediator *clientintf.UserID) {
	nmgr.handlers[onGCWithUnkxdMemberNtfnType].(*handlersFor[OnGCWithUnkxdMemberNtfn]).
		visit(func(h OnGCWithUnkxdMemberNtfn) {
			h(gc, uid, hasKX, hasMI, miCount, startedMIMediator)
		})
}

func (nmgr *NotificationManager) notifyTipReceived(ru *RemoteUser, amountMAtoms int64) {
	nmgr.handlers[onTipReceivedNtfnType].(*handlersFor[OnTipReceivedNtfn]).
		visit(func(h OnTipReceivedNtfn) { h(ru, amountMAtoms) })
}

func (nmgr *NotificationManager) notifyMsgContentFiltered(e MsgContentFilteredEvent) {
	nmgr.handlers[onMessageContentFilteredNtfType].(*handlersFor[OnMsgContentFilteredNtfn]).
		visit(func(h OnMsgContentFilteredNtfn) {
			h(e)
		})
}

func (nmgr *NotificationManager) notifyPostsSubscriberUpdated(ru *RemoteUser, subscribed bool) {
	nmgr.handlers[onPostSubscriberUpdated].(*handlersFor[OnPostSubscriberUpdated]).
		visit(func(h OnPostSubscriberUpdated) { h(ru, subscribed) })
}

func (nmgr *NotificationManager) notifyPostsListReceived(ru *RemoteUser, postList rpc.RMListPostsReply) {
	nmgr.handlers[onPostsListReceived].(*handlersFor[OnPostsListReceived]).
		visit(func(h OnPostsListReceived) { h(ru, postList) })
}

func (nmgr *NotificationManager) notifyUnsubscribingIdleRemote(ru *RemoteUser, lastDecTime time.Time) {
	nmgr.handlers[onUnsubscribingIdleRemoteClient].(*handlersFor[OnUnsubscribingIdleRemoteClient]).
		visit(func(h OnUnsubscribingIdleRemoteClient) { h(ru, lastDecTime) })
}

func (nmgr *NotificationManager) notifyReceiveReceipt(ru *RemoteUser, rr rpc.RMReceiveReceipt, serverTime time.Time) {
	nmgr.handlers[onReceiveReceipt].(*handlersFor[OnReceiveReceipt]).
		visit(func(h OnReceiveReceipt) { h(ru, rr, serverTime) })
}

func (nmgr *NotificationManager) notifyContentListReceived(user *RemoteUser, files []clientdb.RemoteFile, listErr error) {
	nmgr.handlers[onContentListReceived].(*handlersFor[OnContentListReceived]).
		visit(func(h OnContentListReceived) { h(user, files, listErr) })

}

func (nmgr *NotificationManager) notifyFileDownloadCompleted(user *RemoteUser, fm rpc.FileMetadata, diskPath string) {
	nmgr.handlers[onFileDownloadCompleted].(*handlersFor[OnFileDownloadCompleted]).
		visit(func(h OnFileDownloadCompleted) { h(user, fm, diskPath) })
}

func (nmgr *NotificationManager) notifyFileDownloadProgress(user *RemoteUser, fm rpc.FileMetadata, nbMissingChunks int) {
	nmgr.handlers[onFileDownloadProgress].(*handlersFor[OnFileDownloadProgress]).
		visit(func(h OnFileDownloadProgress) { h(user, fm, nbMissingChunks) })
}

func (nmgr *NotificationManager) notifyRMReceived(ru *RemoteUser, rmh *rpc.RMHeader, p interface{}, ts time.Time) {
	nmgr.handlers[onRMReceived].(*handlersFor[OnRMReceived]).
		visit(func(h OnRMReceived) { h(ru, rmh, p, ts) })
}

func (nmgr *NotificationManager) notifyRMQueued(ru *RemoteUser, p interface{}) {
	nmgr.handlers[onRMQueuedNtfnType].(*handlersFor[OnRMQueued]).
		visit(func(h OnRMQueued) { h(ru, p) })
}

func (nmgr *NotificationManager) notifyRMSent(ru *RemoteUser, rv ratchet.RVPoint, p interface{}) {
	nmgr.handlers[onRMSent].(*handlersFor[OnRMSent]).
		visit(func(h OnRMSent) { h(ru, rv, p) })
}

func (nmgr *NotificationManager) notifyUnackedRMSent(uid clientintf.UserID, rv ratchet.RVPoint) {
	nmgr.handlers[onUnackedRMSentNtfnType].(*handlersFor[OnUnackedRMSent]).
		visit(func(h OnUnackedRMSent) { h(uid, rv) })
}

func (nmgr *NotificationManager) notifyServerUnwelcomeError(err error) {
	nmgr.handlers[onServerUnwelcomeError].(*handlersFor[OnServerUnwelcomeError]).
		visit(func(h OnServerUnwelcomeError) { h(err) })
}

func (nmgr *NotificationManager) notifyProfileUpdated(ru *RemoteUser, ab *clientdb.AddressBookEntry,
	fields []ProfileUpdateField) {
	nmgr.handlers[onProfileUpdatedType].(*handlersFor[OnProfileUpdated]).
		visit(func(h OnProfileUpdated) { h(ru, ab, fields) })
}

func (nmgr *NotificationManager) notifyTransitiveEvent(src, tgt clientintf.UserID,
	event TransitiveEvent) {
	nmgr.handlers[onTransitiveEventType].(*handlersFor[OnTransitiveEvent]).
		visit(func(h OnTransitiveEvent) { h(src, tgt, event) })
}

func (nmgr *NotificationManager) notifyRequestingMediateID(mediator, target clientintf.UserID) {
	nmgr.handlers[onRequestingMediateIDType].(*handlersFor[OnRequestingMediateID]).
		visit(func(h OnRequestingMediateID) { h(mediator, target) })
}

func (nmgr *NotificationManager) notifyInvitedToRTDTSession(ru *RemoteUser, invite *rpc.RMRTDTSessionInvite, ts time.Time) {
	nmgr.handlers[onInvitedToRTDTSessionNtfnType].(*handlersFor[OnInvitedToRTDTSession]).
		visit(func(h OnInvitedToRTDTSession) { h(ru, invite, ts) })
}

func (nmgr *NotificationManager) notifyRTDTSessionInviteAccepted(ru *RemoteUser, sessID zkidentity.ShortID,
	asPublisher bool) {
	nmgr.handlers[onRTDTSessionInviteAcceptedNtfnType].(*handlersFor[OnRTDTSessionInviteAccepted]).
		visit(func(h OnRTDTSessionInviteAccepted) { h(ru, sessID, asPublisher) })
}

func (nmgr *NotificationManager) notifyRTDTSessionInviteCanceled(ru *RemoteUser, sessID zkidentity.ShortID) {
	nmgr.handlers[onRTDTSessionInviteCanceledNtfnType].(*handlersFor[OnRTDTSessionInviteCanceled]).
		visit(func(h OnRTDTSessionInviteCanceled) { h(ru, sessID) })
}

func (nmgr *NotificationManager) buildRTDTSessionUpdateNtfn(oldMeta, newMeta *rpc.RMRTDTSession) RTDTSessionUpdateNtfn {
	var res RTDTSessionUpdateNtfn
	res.SessionRV = oldMeta.RV

	// Find out new publishers.
	oldPublishers := make(map[UserID]*rpc.RMRTDTSessionPublisher, len(oldMeta.Publishers))
	for i := range oldMeta.Publishers {
		oldPublishers[oldMeta.Publishers[i].PublisherID] = &oldMeta.Publishers[i]
	}
	for i := range newMeta.Publishers {
		if _, ok := oldPublishers[newMeta.Publishers[i].PublisherID]; !ok {
			res.NewPublishers = append(res.NewPublishers, &newMeta.Publishers[i])
		}
		delete(oldPublishers, newMeta.Publishers[i].PublisherID)
	}
	res.NewMetadata = newMeta

	// Anything left in the map are old publishers that are not in new
	// (i.e. removed).
	for _, oldPub := range oldPublishers {
		res.RemovedPublishers = append(res.RemovedPublishers, oldPub)
	}

	return res
}

func (nmgr *NotificationManager) notifyRTDTSessionUpdated(ru *RemoteUser, update *RTDTSessionUpdateNtfn) {
	nmgr.handlers[onRTDTSesssionUpdatedNtfnType].(*handlersFor[OnRTDTSesssionUpdated]).
		visit(func(h OnRTDTSesssionUpdated) { h(ru, update) })
}

func (nmgr *NotificationManager) notifyRTDTLiveSessionJoined(sessRV zkidentity.ShortID) {
	nmgr.handlers[onRTDTLiveSessionJoinedNtfnType].(*handlersFor[OnRTDTLiveSessionJoined]).
		visit(func(h OnRTDTLiveSessionJoined) { h(sessRV) })
}

func (nmgr *NotificationManager) notifyRTDTRefreshedSessionAllowance(sessRV zkidentity.ShortID, addAllowance uint64) {
	nmgr.handlers[onRTDTRefreshedSessionAllowance].(*handlersFor[OnRTDTRefreshedSessionAllowance]).
		visit(func(h OnRTDTRefreshedSessionAllowance) { h(sessRV, addAllowance) })
}

func (nmgr *NotificationManager) notifyRTDTLivePeerJoined(sessRV zkidentity.ShortID, peerID rpc.RTDTPeerID) {
	nmgr.handlers[onRTDTLivePeerJoinedNtfnType].(*handlersFor[OnRTDTLivePeerJoined]).
		visit(func(h OnRTDTLivePeerJoined) { h(sessRV, peerID) })
}

func (nmgr *NotificationManager) notifyRTDTLivePeerStalled(sessRV zkidentity.ShortID, peerID rpc.RTDTPeerID) {
	nmgr.handlers[onRTDTLivePeerStalledNtfnType].(*handlersFor[OnRTDTLivePeerStalled]).
		visit(func(h OnRTDTLivePeerStalled) { h(sessRV, peerID) })
}

func (nmgr *NotificationManager) notifyRTDTLiveSendErrored(sessRV zkidentity.ShortID, err error) {
	nmgr.handlers[onRTDTLiveSendErroredNtfnType].(*handlersFor[OnRTDTLiveSessionSendErrored]).
		visit(func(h OnRTDTLiveSessionSendErrored) { h(sessRV, err) })
}

func (nmgr *NotificationManager) notifyRTDTRemadeLiveSessionHot(sessRV zkidentity.ShortID) {
	nmgr.handlers[onRTDTRemadeLiveSessionHot].(*handlersFor[OnRTDTRemadeLiveSessionHotAudio]).
		visit(func(h OnRTDTRemadeLiveSessionHotAudio) { h(sessRV) })
}

func (nmgr *NotificationManager) notifyRTDTPeerSoundChanged(sessRV zkidentity.ShortID, peerID rpc.RTDTPeerID, hasSoundStream, hasSound bool) {
	nmgr.handlers[onRTDTPeerSoundChangedNtfnType].(*handlersFor[OnRTDTPeerSoundChanged]).
		visit(func(h OnRTDTPeerSoundChanged) { h(sessRV, peerID, hasSoundStream, hasSound) })
}

func (nmgr *NotificationManager) notifyRTDTPeerExitedSession(ru *RemoteUser, sessRV zkidentity.ShortID, peerID rpc.RTDTPeerID) {
	nmgr.handlers[onRTDTPeerExitedSessionNtfnType].(*handlersFor[OnRTDTPeerExitedSession]).
		visit(func(h OnRTDTPeerExitedSession) { h(ru, sessRV, peerID) })
}

func (nmgr *NotificationManager) notifyRTDTKickedFromLiveSession(sessRV zkidentity.ShortID, peerID rpc.RTDTPeerID, banDuration time.Duration) {
	nmgr.handlers[onRTDTKickedFromLiveSessNtfnType].(*handlersFor[OnRTDTKickedFromLiveSession]).
		visit(func(h OnRTDTKickedFromLiveSession) { h(sessRV, peerID, banDuration) })
}

func (nmgr *NotificationManager) notifyRTDTSessionDissolved(ru *RemoteUser, sessRV zkidentity.ShortID, peerID rpc.RTDTPeerID) {
	nmgr.handlers[onRTDTSessionDissolvedNtfnType].(*handlersFor[OnRTDTSessionDissolved]).
		visit(func(h OnRTDTSessionDissolved) { h(ru, sessRV, peerID) })
}

func (nmgr *NotificationManager) notifyRTDTRemovedFromSession(ru *RemoteUser, sessRV zkidentity.ShortID, reason string) {
	nmgr.handlers[onRTDTRemovedFromSessionNtfnType].(*handlersFor[OnRTDTRemovedFromSession]).
		visit(func(h OnRTDTRemovedFromSession) { h(ru, sessRV, reason) })
}

func (nmgr *NotificationManager) notifyRTDTRotatedCookie(ru *RemoteUser, sessRV zkidentity.ShortID) {
	nmgr.handlers[onRTDTRotatedCookieNtfnType].(*handlersFor[OnRTDTRotatedCookie]).
		visit(func(h OnRTDTRotatedCookie) { h(ru, sessRV) })
}

func (nmgr *NotificationManager) notifyRTDTChatMsgReceived(sessionRV zkidentity.ShortID, pub rpc.RMRTDTSessionPublisher, msg string, ts uint32) {
	nmgr.handlers[onRTDTChatMsgRcvdNtfnType].(*handlersFor[OnRTDTChatMessageReceived]).
		visit(func(h OnRTDTChatMessageReceived) { h(sessionRV, pub, msg, ts) })
}

func (nmgr *NotificationManager) notifyRTDTAdminCookiesReceived(ru *RemoteUser, sessionRV zkidentity.ShortID) {
	nmgr.handlers[onRTDTAdminCookiesRcvdNtfnType].(*handlersFor[OnRTDTAdminCookiesReceived]).
		visit(func(h OnRTDTAdminCookiesReceived) { h(ru, sessionRV) })
}

func (nmgr *NotificationManager) notifyRTDTRTTCalculated(addr net.UDPAddr, rtt time.Duration) {
	nmgr.handlers[onRTDTRTTCalculatedNtfnType].(*handlersFor[OnRTDTRTTCalculated]).
		visit(func(h OnRTDTRTTCalculated) { h(addr, rtt) })
}

func (nmgr *NotificationManager) notifyRTDTJoinedInstantCall(sessionRV zkidentity.ShortID) {
	nmgr.handlers[onRTDTJoinedInstantCallNtfnType].(*handlersFor[OnRTDTJoinedInstantCall]).
		visit(func(h OnRTDTJoinedInstantCall) { h(sessionRV) })
}

func NewNotificationManager() *NotificationManager {
	nmgr := &NotificationManager{
		uiConfig: UINotificationsConfig{
			MaxLength:    255,
			EmitInterval: 30 * time.Second,
		},
		uiTimer: time.NewTimer(time.Hour * 24),
		handlers: map[string]handlersRegistry{
			onTestNtfnType:           &handlersFor[onTestNtfn]{},
			onPMNtfnType:             &handlersFor[OnPMNtfn]{},
			onGCMNtfnType:            &handlersFor[OnGCMNtfn]{},
			onKXCompleted:            &handlersFor[OnKXCompleted]{},
			onKXSuggested:            &handlersFor[OnKXSuggested]{},
			onBlockNtfnType:          &handlersFor[OnBlockNtfn]{},
			onPostRcvdNtfnType:       &handlersFor[OnPostRcvdNtfn]{},
			onPostStatusRcvdNtfnType: &handlersFor[OnPostStatusRcvdNtfn]{},
			onHandshakeStageNtfnType: &handlersFor[OnHandshakeStageNtfn]{},
			onTipReceivedNtfnType:    &handlersFor[OnTipReceivedNtfn]{},
			onReceiveReceipt:         &handlersFor[OnReceiveReceipt]{},
			onRMReceived:             &handlersFor[OnRMReceived]{},
			onRMQueuedNtfnType:       &handlersFor[OnRMQueued]{},
			onRMSent:                 &handlersFor[OnRMSent]{},
			onUnackedRMSentNtfnType:  &handlersFor[OnUnackedRMSent]{},
			onProfileUpdatedType:     &handlersFor[OnProfileUpdated]{},
			onTransitiveEventType:    &handlersFor[OnTransitiveEvent]{},
			onUINtfnType:             &handlersFor[OnUINotification]{},

			onPostSubscriberUpdated:    &handlersFor[OnPostSubscriberUpdated]{},
			onPostsListReceived:        &handlersFor[OnPostsListReceived]{},
			onGCVersionWarningType:     &handlersFor[OnGCVersionWarning]{},
			onJoinedGCNtfnType:         &handlersFor[OnJoinedGCNtfn]{},
			onAddedGCMembersNtfnType:   &handlersFor[OnAddedGCMembersNtfn]{},
			onRemovedGCMembersNtfnType: &handlersFor[OnRemovedGCMembersNtfn]{},
			onGCUpgradedNtfnType:       &handlersFor[OnGCUpgradedNtfn]{},
			onInvitedToGCNtfnType:      &handlersFor[OnInvitedToGCNtfn]{},
			onGCInviteAcceptedNtfnType: &handlersFor[OnGCInviteAcceptedNtfn]{},
			onGCUserPartedNtfnType:     &handlersFor[OnGCUserPartedNtfn]{},
			onGCKilledNtfnType:         &handlersFor[OnGCKilledNtfn]{},
			onGCAdminsChangedNtfnType:  &handlersFor[OnGCAdminsChangedNtfn]{},
			onContentListReceived:      &handlersFor[OnContentListReceived]{},
			onFileDownloadCompleted:    &handlersFor[OnFileDownloadCompleted]{},
			onFileDownloadProgress:     &handlersFor[OnFileDownloadProgress]{},
			onServerUnwelcomeError:     &handlersFor[OnServerUnwelcomeError]{},
			onRequestingMediateIDType:  &handlersFor[OnRequestingMediateID]{},

			onKXSearchCompletedNtfnType:         &handlersFor[OnKXSearchCompleted]{},
			onInvoiceGenFailedNtfnType:          &handlersFor[OnInvoiceGenFailedNtfn]{},
			onRemoteSubscriptionChangedType:     &handlersFor[OnRemoteSubscriptionChangedNtfn]{},
			onRemoteSubscriptionErrorNtfnType:   &handlersFor[OnRemoteSubscriptionErrorNtfn]{},
			onLocalClientOfflineTooLong:         &handlersFor[OnLocalClientOfflineTooLong]{},
			onTipAttemptProgressNtfnType:        &handlersFor[OnTipAttemptProgressNtfn]{},
			onTipUserInvoiceGeneratedNtfnType:   &handlersFor[OnTipUserInvoiceGeneratedNtfn]{},
			onServerSessionChangedNtfnType:      &handlersFor[OnServerSessionChangedNtfn]{},
			onOnboardStateChangedNtfnType:       &handlersFor[OnOnboardStateChangedNtfn]{},
			onResourceFetchedNtfnType:           &handlersFor[OnResourceFetchedNtfn]{},
			onGCWithUnkxdMemberNtfnType:         &handlersFor[OnGCWithUnkxdMemberNtfn]{},
			onMessageContentFilteredNtfType:     &handlersFor[OnMsgContentFilteredNtfn]{},
			onUnsubscribingIdleRemoteClient:     &handlersFor[OnUnsubscribingIdleRemoteClient]{},
			onInvitedToRTDTSessionNtfnType:      &handlersFor[OnInvitedToRTDTSession]{},
			onRTDTSessionInviteAcceptedNtfnType: &handlersFor[OnRTDTSessionInviteAccepted]{},
			onRTDTSessionInviteCanceledNtfnType: &handlersFor[OnRTDTSessionInviteCanceled]{},
			onRTDTSesssionUpdatedNtfnType:       &handlersFor[OnRTDTSesssionUpdated]{},
			onRTDTLiveSessionJoinedNtfnType:     &handlersFor[OnRTDTLiveSessionJoined]{},
			onRTDTRefreshedSessionAllowance:     &handlersFor[OnRTDTRefreshedSessionAllowance]{},
			onRTDTLivePeerJoinedNtfnType:        &handlersFor[OnRTDTLivePeerJoined]{},
			onRTDTLivePeerStalledNtfnType:       &handlersFor[OnRTDTLivePeerStalled]{},
			onRTDTLiveSendErroredNtfnType:       &handlersFor[OnRTDTLiveSessionSendErrored]{},
			onRTDTRemadeLiveSessionHot:          &handlersFor[OnRTDTRemadeLiveSessionHotAudio]{},
			onRTDTPeerSoundChangedNtfnType:      &handlersFor[OnRTDTPeerSoundChanged]{},
			onRTDTPeerExitedSessionNtfnType:     &handlersFor[OnRTDTPeerExitedSession]{},
			onRTDTKickedFromLiveSessNtfnType:    &handlersFor[OnRTDTKickedFromLiveSession]{},
			onRTDTSessionDissolvedNtfnType:      &handlersFor[OnRTDTSessionDissolved]{},
			onRTDTRemovedFromSessionNtfnType:    &handlersFor[OnRTDTRemovedFromSession]{},
			onRTDTRotatedCookieNtfnType:         &handlersFor[OnRTDTRotatedCookie]{},
			onRTDTChatMsgRcvdNtfnType:           &handlersFor[OnRTDTChatMessageReceived]{},
			onRTDTAdminCookiesRcvdNtfnType:      &handlersFor[OnRTDTAdminCookiesReceived]{},
			onRTDTRTTCalculatedNtfnType:         &handlersFor[OnRTDTRTTCalculated]{},
			onRTDTJoinedInstantCallNtfnType:     &handlersFor[OnRTDTJoinedInstantCall]{},
		},
	}
	if !nmgr.uiTimer.Stop() {
		<-nmgr.uiTimer.C
	}

	return nmgr
}
