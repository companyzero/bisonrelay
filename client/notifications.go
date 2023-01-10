package client

import (
	"fmt"
	"sync"
	"time"

	"github.com/companyzero/bisonrelay/rpc"
)

// Following are the notification types. Add new types at the bottom of this
// list, then add a notifyX() to NotificationManager and initialize a new
// container in NewNotificationManager().

const onPMNtfnType = "onPM"

// OnPMNtfn is the handler for received private messages.
type OnPMNtfn func(*RemoteUser, rpc.RMPrivateMessage, time.Time)

func (_ OnPMNtfn) typ() string { return onPMNtfnType }

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

func NewNotificationManager() *NotificationManager {
	return &NotificationManager{
		handlers: map[string]handlersRegistry{
			onTestNtfnType: &handlersFor[onTestNtfn]{},
			onPMNtfnType:   &handlersFor[OnPMNtfn]{},
		},
	}
}
