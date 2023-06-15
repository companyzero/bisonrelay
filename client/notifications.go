package client

import (
	"fmt"
	"sync"
	"time"

	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
)

// Following are the notification types. Add new types at the bottom of this
// list, then add a notifyX() to NotificationManager and initialize a new
// container in NewNotificationManager().

const onPMNtfnType = "onPM"

// OnPMNtfn is the handler for received private messages.
type OnPMNtfn func(*RemoteUser, rpc.RMPrivateMessage, time.Time)

func (_ OnPMNtfn) typ() string { return onPMNtfnType }

const onGCMNtfnType = "onGCM"

// OnGCMNtfn is the handler for received gc messages.
type OnGCMNtfn func(*RemoteUser, rpc.RMGroupMessage, time.Time)

func (_ OnGCMNtfn) typ() string { return onGCMNtfnType }

const onPostRcvdNtfnType = "onPostRcvd"

// OnPostRcvdNtfn is the handler for received posts.
type OnPostRcvdNtfn func(*RemoteUser, clientdb.PostSummary, rpc.PostMetadata)

func (_ OnPostRcvdNtfn) typ() string { return onPostRcvdNtfnType }

const onPostStatusRcvdNtfnType = "onPostStatusRcvd"

// OnPostStatusRcvdNtfn is the handler for received post status updates.0
type OnPostStatusRcvdNtfn func(*RemoteUser, clientintf.PostID, UserID,
	rpc.PostMetadataStatus)

func (_ OnPostStatusRcvdNtfn) typ() string { return onPostStatusRcvdNtfnType }

const onRemoteSubscriptionChangedType = "onSubChanged"

// OnRemoteSubscriptionChanged is the handler for a remote user subscription
// changed event.
type OnRemoteSubscriptionChangedNtfn func(*RemoteUser, bool)

func (_ OnRemoteSubscriptionChangedNtfn) typ() string { return onRemoteSubscriptionChangedType }

const onRemoteSubscriptionErrorNtfnType = "onSubChangedErr"

// OnRemoteSubscriptionErrorNtfn is the handler for a remote user subscription
// change attempt that errored.
type OnRemoteSubscriptionErrorNtfn func(user *RemoteUser, wasSubscribing bool, errMsg string)

func (_ OnRemoteSubscriptionErrorNtfn) typ() string { return onRemoteSubscriptionErrorNtfnType }

const onLocalClientOfflineTooLong = "onLocalClientOfflineTooLong"

// OnLocalClientOfflineTooLong is called after the local client connects to the
// server, if it determines it has been offline for too long given the server's
// message retention policy.
type OnLocalClientOfflineTooLong func(time.Time)

func (_ OnLocalClientOfflineTooLong) typ() string { return onLocalClientOfflineTooLong }

const onKXCompleted = "onKXCompleted"

// OnKXCompleted is called after KX is completed with a remote user (either a
// new user or a reset KX).
type OnKXCompleted func(*clientintf.RawRVID, *RemoteUser, bool)

func (_ OnKXCompleted) typ() string { return onKXCompleted }

const onKXSuggested = "onKXSuggested"

// OnKXSuggested is called after a remote user suggests that this user should KX
// with another remote user.
type OnKXSuggested func(*RemoteUser, zkidentity.PublicIdentity)

func (_ OnKXSuggested) typ() string { return onKXSuggested }

const onInvoiceGenFailedNtfnType = "onInvoiceGenFailed"

type OnInvoiceGenFailedNtfn func(user *RemoteUser, dcrAmount float64, err error)

func (_ OnInvoiceGenFailedNtfn) typ() string { return onInvoiceGenFailedNtfnType }

const onGCVersionWarningType = "onGCVersionWarn"

// OnGCVersionWarning is a handler for warnings about a GC that has an
// unsupported version.
type OnGCVersionWarning func(user *RemoteUser, gc rpc.RMGroupList, minVersion, maxVersion uint8)

func (_ OnGCVersionWarning) typ() string { return onGCVersionWarningType }

const onJoinedGCNtfnType = "onJoinedGC"

// OnJoinedGCNtfn is a handler for when the local client joins a GC.
type OnJoinedGCNtfn func(gc rpc.RMGroupList)

func (_ OnJoinedGCNtfn) typ() string { return onJoinedGCNtfnType }

const onAddedGCMembersNtfnType = "onAddedGCMembers"

// OnAddedGCMembersNtfn is a handler for new members added to a GC.
type OnAddedGCMembersNtfn func(gc rpc.RMGroupList, uids []clientintf.UserID)

func (_ OnAddedGCMembersNtfn) typ() string { return onAddedGCMembersNtfnType }

const onRemovedGCMembersNtfnType = "onRemovedGCMembers"

// OnRemovedGCMembersNtfn is a handler for members removed from a GC.
type OnRemovedGCMembersNtfn func(gc rpc.RMGroupList, uids []clientintf.UserID)

func (_ OnRemovedGCMembersNtfn) typ() string { return onRemovedGCMembersNtfnType }

const onGCUpgradedNtfnType = "onGCUpgraded"

// OnGCUpgradedNtfn is a handler for a GC that had its version upgraded.
type OnGCUpgradedNtfn func(gc rpc.RMGroupList, oldVersion uint8)

func (_ OnGCUpgradedNtfn) typ() string { return onGCUpgradedNtfnType }

const onInvitedToGCNtfnType = "onInvitedToGC"

// OnInvitedToGCNtfn is a handler for invites received to join GCs.
type OnInvitedToGCNtfn func(user *RemoteUser, iid uint64, invite rpc.RMGroupInvite)

func (_ OnInvitedToGCNtfn) typ() string { return onInvitedToGCNtfnType }

const onGCInviteAcceptedNtfnType = "onGCInviteAccepted"

// OnGCInviteAcceptedNtfn is a handler for invites accepted by remote users to
// join a GC we invited them to.
type OnGCInviteAcceptedNtfn func(user *RemoteUser, gc rpc.RMGroupList)

func (_ OnGCInviteAcceptedNtfn) typ() string { return onGCInviteAcceptedNtfnType }

const onGCUserPartedNtfnType = "onGCUserParted"

// OnGCUserPartedNtfn is a handler when a user parted from a GC or an admin
// kicked a user.
type OnGCUserPartedNtfn func(gcid GCID, uid UserID, reason string, kicked bool)

func (_ OnGCUserPartedNtfn) typ() string { return onGCUserPartedNtfnType }

const onGCKilledNtfnType = "onGCKilled"

// OnGCKilledNtfn is a handler for a GC dissolved by its admin.
type OnGCKilledNtfn func(gcid GCID, reason string)

func (_ OnGCKilledNtfn) typ() string { return onGCKilledNtfnType }

const onGCAdminsChangedNtfnType = "onGCAdminsChanged"

type OnGCAdminsChangedNtfn func(ru *RemoteUser, gc rpc.RMGroupList, added, removed []zkidentity.ShortID)

func (_ OnGCAdminsChangedNtfn) typ() string { return onGCAdminsChangedNtfnType }

const onKXSearchCompletedNtfnType = "kxSearchCompleted"

// OnKXSearchCompleted is a handler for completed KX search procedures.
type OnKXSearchCompleted func(user *RemoteUser)

func (_ OnKXSearchCompleted) typ() string { return onKXSearchCompletedNtfnType }

const onTipAttemptProgressNtfnType = "onTipAttemptProgress"

type OnTipAttemptProgressNtfn func(ru *RemoteUser, amtMAtoms int64, completed bool, attempt int, attemptErr error, willRetry bool)

func (_ OnTipAttemptProgressNtfn) typ() string { return onTipAttemptProgressNtfnType }

const onBlockNtfnType = "onBlock"

// OnBlockNtfn is called when we blocked the specified user due to their
// request. Note that the passed user cannot be used for messaging anymore.
type OnBlockNtfn func(user *RemoteUser)

func (_ OnBlockNtfn) typ() string { return onBlockNtfnType }

const onServerSessionChangedNtfnType = "onServerSessionChanged"

// OnServerSessionChangedNtfn is called indicating that the connection to the
// server changed to the specified state (either connected or not).
//
// The push and subscription rates are specified in milliatoms/byte.
type OnServerSessionChangedNtfn func(connected bool, pushRate, subRate, expirationDays uint64)

func (_ OnServerSessionChangedNtfn) typ() string { return onServerSessionChangedNtfnType }

const onOnboardStateChangedNtfnType = "onOnboardStateChanged"

type OnOnboardStateChangedNtfn func(state clientintf.OnboardState, err error)

func (_ OnOnboardStateChangedNtfn) typ() string { return onOnboardStateChangedNtfnType }

const onResourceFetchedNtfnType = "onResourceFetched"

// OnResourceFetchedNtfn is called when a reply to a fetched resource is
// received.
//
// Note that the user may be nil if the resource was fetched locally, such as
// through the FetchLocalResource call.
type OnResourceFetchedNtfn func(ru *RemoteUser, fr clientdb.FetchedResource, sess clientdb.PageSessionOverview)

func (_ OnResourceFetchedNtfn) typ() string { return onResourceFetchedNtfnType }

const onTipUserInvoiceGeneratedNtfnType = "onTipUserInvoiceGenerated"

// OnTipUserInvoiceGeneratedNtfn is called when the local client generates an
// invoice to send to a remote user, for tipping purposes.
type OnTipUserInvoiceGeneratedNtfn func(ru *RemoteUser, tag uint32, invoice string)

func (_ OnTipUserInvoiceGeneratedNtfn) typ() string { return onTipUserInvoiceGeneratedNtfnType }

const onHandshakeStageNtfnType = "onHandshakeStage"

// OnHandshakeStageNtfn is called during a 3-way handshake with a remote client.
// mstype may be SYN, SYNACK or ACK. The SYNACK and ACK types allow the
// respective clients to infer that the ratchet operations are working.
type OnHandshakeStageNtfn func(ru *RemoteUser, msgtype string)

func (_ OnHandshakeStageNtfn) typ() string { return onHandshakeStageNtfnType }

// The following is used only in tests.

const onTestNtfnType = "testNtfnType"

type onTestNtfn func()

func (_ onTestNtfn) typ() string { return onTestNtfnType }

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

type handlersRegistry interface {
	Register(v interface{}, async bool) NotificationRegistration
}

type NotificationManager struct {
	handlers map[string]handlersRegistry
}

func (nmgr *NotificationManager) register(handler NotificationHandler, async bool) NotificationRegistration {
	handlers := nmgr.handlers[handler.typ()]
	if handlers == nil {
		panic(fmt.Sprintf("forgot to init the handler type %T "+
			"in NewNotificationManager", handler))
	}

	return handlers.Register(handler, async)
}

func (nmgr *NotificationManager) Register(handler NotificationHandler) NotificationRegistration {
	return nmgr.register(handler, true)
}

func (nmgr *NotificationManager) RegisterSync(handler NotificationHandler) NotificationRegistration {
	return nmgr.register(handler, false)
}

// Following are the notifyX() calls (one for each type of notification).

func (nmgr *NotificationManager) notifyTest() {
	nmgr.handlers[onTestNtfnType].(*handlersFor[onTestNtfn]).
		visit(func(h onTestNtfn) { h() })
}

func (nmgr *NotificationManager) notifyOnPM(user *RemoteUser, pm rpc.RMPrivateMessage, ts time.Time) {
	nmgr.handlers[onPMNtfnType].(*handlersFor[OnPMNtfn]).
		visit(func(h OnPMNtfn) { h(user, pm, ts) })
}

func (nmgr *NotificationManager) notifyOnGCM(user *RemoteUser, gcm rpc.RMGroupMessage, ts time.Time) {
	nmgr.handlers[onGCMNtfnType].(*handlersFor[OnGCMNtfn]).
		visit(func(h OnGCMNtfn) { h(user, gcm, ts) })
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

func (nmgr *NotificationManager) notifyOnGCKilled(gcid GCID, reason string) {
	nmgr.handlers[onGCKilledNtfnType].(*handlersFor[OnGCKilledNtfn]).
		visit(func(h OnGCKilledNtfn) { h(gcid, reason) })
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

func (nmgr *NotificationManager) notifyServerSessionChanged(connected bool, pushRate, subRate, expDays uint64) {
	nmgr.handlers[onServerSessionChangedNtfnType].(*handlersFor[OnServerSessionChangedNtfn]).
		visit(func(h OnServerSessionChangedNtfn) { h(connected, pushRate, subRate, expDays) })
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

func NewNotificationManager() *NotificationManager {
	return &NotificationManager{
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

			onKXSearchCompletedNtfnType:       &handlersFor[OnKXSearchCompleted]{},
			onInvoiceGenFailedNtfnType:        &handlersFor[OnInvoiceGenFailedNtfn]{},
			onRemoteSubscriptionChangedType:   &handlersFor[OnRemoteSubscriptionChangedNtfn]{},
			onRemoteSubscriptionErrorNtfnType: &handlersFor[OnRemoteSubscriptionErrorNtfn]{},
			onLocalClientOfflineTooLong:       &handlersFor[OnLocalClientOfflineTooLong]{},
			onTipAttemptProgressNtfnType:      &handlersFor[OnTipAttemptProgressNtfn]{},
			onTipUserInvoiceGeneratedNtfnType: &handlersFor[OnTipUserInvoiceGeneratedNtfn]{},
			onServerSessionChangedNtfnType:    &handlersFor[OnServerSessionChangedNtfn]{},
			onOnboardStateChangedNtfnType:     &handlersFor[OnOnboardStateChangedNtfn]{},
			onResourceFetchedNtfnType:         &handlersFor[OnResourceFetchedNtfn]{},
		},
	}
}
