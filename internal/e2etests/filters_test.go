package e2etests

import (
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"golang.org/x/exp/maps"
)

// TestContentFilters assert that content filters work as expected.
func TestContentFilters(t *testing.T) {
	t.Parallel()

	// This is a complex test, due to the need to test various simultaneous
	// conditions. In particular, in order to assert that content filtering
	// is working, both filtered and unfiltered messages need to be tested,
	// in all contexts, in order to determine that ONLY the intended
	// messages are in fact filtered (and the other ones are not).
	//
	// Several combinations for filtering exist, and this test exercises
	// only some of them.
	//
	// Also, to make the test run faster, each separate filter setup needs
	// to be tested in parallel.
	//
	// The general strategy of each test case is to block some messages
	// from Bob (and sometimes from Charlie) and then send messages on all
	// contexts which have filtering, one message which has the target
	// filtering string and one that does not. Then the ones that should be
	// blocked (according to the test case) are verified to be blocked and
	// all the other ones are verified to be received.
	//
	// The following messaging contexts check for content filters:
	//
	//   - PMs
	//   - GCMs
	//   - Posts
	//   - Post Comments

	const (
		// Setup sample messages (one that contains the target string
		// to filter, one that does not).
		testMsg      = "someprefix TaRgEtStR somesuffix"
		filterRegexp = "(?i)targetStr"
		okMsg        = "someprefix not_the_target_str somesuffix"

		// Individual testing messages that are sent.

		pmBob              = "pm_bob"
		pmCharlie          = "pm_charlie"
		gcmBobGC1          = "gcm_gc01_bob"
		gcmCharlieGC1      = "gcm_gc01_charlie"
		gcmBobGC2          = "gcm_gc02_bob"
		gcmCharlieGC2      = "gcm_gc02_charlie"
		postBob            = "post_bob"
		postCharlie        = "post_charlie"
		pcAliceOnAlice     = "postcomment_alice_alice"     // Alice on Alice's post
		pcBobOnAlice       = "postcomment_alice_bob"       // Bob on Alice's post
		pcCharlieOnAlice   = "postcomment_alice_charlie"   // Charlie on Alice's post
		pcAliceOnBob       = "postcomment_bob_alice"       // Alice on Bob's post
		pcBobOnBob         = "postcomment_bob_bob"         // Bob on Bob's post
		pcCharlieOnBob     = "postcomment_bob_charlie"     // Charlie on Bob's post
		pcAliceOnCharlie   = "postcomment_charlie_alice"   // Alice on Charlie's post
		pcBobOnCharlie     = "postcomment_charlie_bob"     // Bob on Charlie's post
		pcCharlieOnCharlie = "postcomment_charlie_charlie" // Charlie on Charlie's post
	)

	// Helpers to indentify which messages should be filtered on each test
	// case and which rule should be used to filter it.
	filters := func(filters ...func(m map[string]int)) map[string]int {
		res := map[string]int{}
		for _, f := range filters {
			f(res)
		}
		return res
	}
	filtersMsg := func(id string, rule int) func(m map[string]int) {
		return func(m map[string]int) {
			m[id] = rule
		}
	}

	// Helpers to create the actual filters installed on the client on each
	// test case.
	mkfilter := func(funcs ...func(*clientdb.ContentFilter)) clientdb.ContentFilter {
		cf := clientdb.ContentFilter{Regexp: filterRegexp}
		for _, f := range funcs {
			f(&cf)
		}
		return cf
	}
	pseudoBobId := clientintf.UserID{31: 0x02}
	mkfFromBob := func() func(*clientdb.ContentFilter) {
		return func(cf *clientdb.ContentFilter) {
			cf.UID = &pseudoBobId
		}
	}
	mkfSkipAll := func() func(*clientdb.ContentFilter) {
		return func(cf *clientdb.ContentFilter) {
			cf.SkipPMs = true
			cf.SkipGCMs = true
			cf.SkipPosts = true
			cf.SkipPostComments = true
		}
	}
	pseudoGC1 := clientintf.UserID{30: 0x02}
	mkfGC1 := func() func(*clientdb.ContentFilter) {
		return func(cf *clientdb.ContentFilter) {
			cf.GC = &pseudoGC1
			cf.SkipGCMs = false
		}
	}
	mkfAnyGC := func() func(*clientdb.ContentFilter) {
		return func(cf *clientdb.ContentFilter) {
			cf.SkipGCMs = false
		}
	}
	mkfPM := func() func(*clientdb.ContentFilter) {
		return func(cf *clientdb.ContentFilter) {
			cf.SkipPMs = false
		}
	}
	mkfPosts := func() func(*clientdb.ContentFilter) {
		return func(cf *clientdb.ContentFilter) {
			cf.SkipPosts = false
		}
	}
	mkfPostComments := func() func(*clientdb.ContentFilter) {
		return func(cf *clientdb.ContentFilter) {
			cf.SkipPostComments = false
		}
	}

	// The actual test cases.
	tests := []struct {
		name     string
		filters  []clientdb.ContentFilter
		filtered map[string]int
	}{{
		// Test the null case (where nothing is blocked) to ensure
		// all test messages would be received in the cases they
		// were not blocked.
		name:     "null test",
		filtered: filters(),
	}, {
		// Simplest filter possible: block all messages in all contexts
		// that have the target filter string.
		name: "all test msgs filtered",
		filters: []clientdb.ContentFilter{
			mkfilter(),
		},
		filtered: filters(
			filtersMsg(pmBob, 1),
			filtersMsg(pmCharlie, 1),
			filtersMsg(gcmBobGC1, 1),
			filtersMsg(gcmBobGC2, 1),
			filtersMsg(gcmCharlieGC1, 1),
			filtersMsg(gcmCharlieGC2, 1),
			filtersMsg(postBob, 1),
			filtersMsg(postCharlie, 1),
			filtersMsg(pcBobOnAlice, 1),
			filtersMsg(pcBobOnBob, 1),
			filtersMsg(pcBobOnCharlie, 1),
			filtersMsg(pcCharlieOnAlice, 1),
			filtersMsg(pcCharlieOnBob, 1),
			filtersMsg(pcCharlieOnCharlie, 1),
		),
	}, {
		// Simplest user filter: block all messages in all contexts from
		// a specific user, which contain the target string.
		name: "all test messages from bob are filtered",
		filters: []clientdb.ContentFilter{
			mkfilter(mkfFromBob()),
		},
		filtered: filters(
			filtersMsg(pmBob, 1),
			filtersMsg(gcmBobGC1, 1),
			filtersMsg(gcmBobGC2, 1),
			filtersMsg(postBob, 1),
			filtersMsg(pcBobOnAlice, 1),
			filtersMsg(pcBobOnBob, 1),
			filtersMsg(pcBobOnCharlie, 1),
		),
	}, {
		// This shows that a filter can be applied to only block
		// certain PMs from a specific user.
		name: "only PMs from bob are blocked",
		filters: []clientdb.ContentFilter{
			mkfilter(mkfSkipAll(), mkfFromBob(), mkfPM()),
		},
		filtered: filters(
			filtersMsg(pmBob, 1),
		),
	}, {
		// This shows that a filter can be applied to only block
		// certain messages for a specific user in a specific GC.
		name: "only messages from bob in gc1 are blocked",
		filters: []clientdb.ContentFilter{
			mkfilter(mkfSkipAll(), mkfFromBob(), mkfGC1()),
		},
		filtered: filters(
			filtersMsg(gcmBobGC1, 1),
		),
	}, {
		name: "only messages from bob in any gc are blocked",
		filters: []clientdb.ContentFilter{
			mkfilter(mkfSkipAll(), mkfFromBob(), mkfAnyGC()),
		},
		filtered: filters(
			filtersMsg(gcmBobGC1, 1),
			filtersMsg(gcmBobGC2, 1),
		),
	}, {
		name: "only posts by bob are filtered",
		filters: []clientdb.ContentFilter{
			mkfilter(mkfSkipAll(), mkfFromBob(), mkfPosts()),
		},
		filtered: filters(
			filtersMsg(postBob, 1),
		),
	}, {
		name: "only post comments by bob are filtered",
		filters: []clientdb.ContentFilter{
			mkfilter(mkfSkipAll(), mkfFromBob(), mkfPostComments()),
		},
		filtered: filters(
			filtersMsg(pcBobOnAlice, 1),
			filtersMsg(pcBobOnBob, 1),
			filtersMsg(pcBobOnCharlie, 1),
		),
	}, {
		name: "only PMs are blocked",
		filters: []clientdb.ContentFilter{
			mkfilter(mkfSkipAll(), mkfPM()),
		},
		filtered: filters(
			filtersMsg(pmBob, 1),
			filtersMsg(pmCharlie, 1),
		),
	}, {
		name: "only GCMs are blocked",
		filters: []clientdb.ContentFilter{
			mkfilter(mkfSkipAll(), mkfAnyGC()),
		},
		filtered: filters(
			filtersMsg(gcmBobGC1, 1),
			filtersMsg(gcmBobGC2, 1),
			filtersMsg(gcmCharlieGC1, 1),
			filtersMsg(gcmCharlieGC2, 1),
		),
	}, {
		name: "only posts are blocked",
		filters: []clientdb.ContentFilter{
			mkfilter(mkfSkipAll(), mkfPosts()),
		},
		filtered: filters(
			filtersMsg(postBob, 1),
			filtersMsg(postCharlie, 1),
		),
	}, {
		name: "only post comments are blocked",
		filters: []clientdb.ContentFilter{
			mkfilter(mkfSkipAll(), mkfPostComments()),
		},
		filtered: filters(
			filtersMsg(pcBobOnAlice, 1),
			filtersMsg(pcBobOnBob, 1),
			filtersMsg(pcBobOnCharlie, 1),
			filtersMsg(pcCharlieOnAlice, 1),
			filtersMsg(pcCharlieOnBob, 1),
			filtersMsg(pcCharlieOnCharlie, 1),
		),
	}, {
		// This shows that a filter rule may affect more than one
		// context selectively (in this case, only PMs and GCMs).
		name: "both PMs and GCms are blocked",
		filters: []clientdb.ContentFilter{
			mkfilter(mkfSkipAll(), mkfPM(), mkfAnyGC()),
		},
		filtered: filters(
			filtersMsg(pmBob, 1),
			filtersMsg(pmCharlie, 1),
			filtersMsg(gcmBobGC1, 1),
			filtersMsg(gcmBobGC2, 1),
			filtersMsg(gcmCharlieGC1, 1),
			filtersMsg(gcmCharlieGC2, 1),
		),
	}, {
		name: "both PMs and GCMs from bob are blocked",
		filters: []clientdb.ContentFilter{
			mkfilter(mkfSkipAll(), mkfFromBob(), mkfPM(), mkfAnyGC()),
		},
		filtered: filters(
			filtersMsg(pmBob, 1),
			filtersMsg(gcmBobGC1, 1),
			filtersMsg(gcmBobGC2, 1),
		),
	}, {
		// This shows that a message that was not blocked by one rule
		// may still be blocked by another (i.e. rules are processed
		// in sequence).
		name: "different rules to block PMs and GCMs in gc1 from bob",
		filters: []clientdb.ContentFilter{
			mkfilter(mkfSkipAll(), mkfFromBob(), mkfPM()),
			mkfilter(mkfSkipAll(), mkfFromBob(), mkfGC1()),
		},
		filtered: filters(
			filtersMsg(pmBob, 1),
			filtersMsg(gcmBobGC1, 2),
		),
	}, {
		name: "different rules to block PMs from bob and post comments by anyone",
		filters: []clientdb.ContentFilter{
			mkfilter(mkfSkipAll(), mkfFromBob(), mkfPM()),
			mkfilter(mkfSkipAll(), mkfPostComments()),
		},
		filtered: filters(
			filtersMsg(pmBob, 1),
			filtersMsg(pcBobOnAlice, 2),
			filtersMsg(pcBobOnBob, 2),
			filtersMsg(pcBobOnCharlie, 2),
			filtersMsg(pcCharlieOnAlice, 2),
			filtersMsg(pcCharlieOnBob, 2),
			filtersMsg(pcCharlieOnCharlie, 2),
		),
	}}

	for i, tc := range tests {
		tc := tc
		i := i
		ok := t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Setup Alice, Bob and Charlie
			tcfg := testScaffoldCfg{}
			ts := newTestScaffold(t, tcfg)
			alice := ts.newClient("alice", withLogName(fmt.Sprintf("alice%02d", i)))
			bob := ts.newClient("bob", withLogName(fmt.Sprintf("bob%02d", i)))
			charlie := ts.newClient("charlie", withLogName(fmt.Sprintf("chrle%02d", i)))

			// KX users.
			ts.kxUsers(alice, bob)
			ts.kxUsers(alice, charlie)
			ts.kxUsers(charlie, bob)

			// Create and populate test GCs.
			gc1, err := alice.NewGroupChat("gc01")
			assert.NilErr(t, err)
			gc2, err := alice.NewGroupChat("gc02")
			assert.NilErr(t, err)
			assertJoinsGC(t, alice, bob, gc1)
			assertJoinsGC(t, alice, charlie, gc1)
			assertJoinsGC(t, alice, bob, gc2)
			assertJoinsGC(t, alice, charlie, gc2)
			gcNames := map[zkidentity.ShortID]string{gc1: "gc01", gc2: "gc02"}

			// Make post subscriptions.
			assertSubscribeToPosts(t, alice, bob)
			assertSubscribeToPosts(t, bob, alice)
			assertSubscribeToPosts(t, alice, charlie)
			assertSubscribeToPosts(t, bob, charlie)
			assertSubscribeToPosts(t, charlie, alice)
			assertSubscribeToPosts(t, charlie, bob)

			// Create test posts and assert everyone sees them.
			alicePost := assertReceivesNewPost(t, alice, nil, bob, charlie)
			bobPost := assertReceivesNewPost(t, bob, nil, alice, charlie)
			charliePost := assertReceivesNewPost(t, charlie, nil, alice, bob)

			_, _, _ = alicePost, bobPost, charliePost

			// Alice will create filters and will be the one used for checking
			// whether the filtering happens.
			aliceMsgChan := make(chan string, 10)
			alice.handle(client.OnPMNtfn(func(ru *client.RemoteUser, pm rpc.RMPrivateMessage, _ time.Time) {
				aliceMsgChan <- "pm_" + ru.Nick() + "_" + pm.Message
			}))
			alice.handle(client.OnGCMNtfn(func(ru *client.RemoteUser, gcm rpc.RMGroupMessage, _ time.Time) {
				gc, _ := alice.GetGCAlias(gcm.ID)
				aliceMsgChan <- "gcm_" + gc + "_" + ru.Nick() + "_" + gcm.Message
			}))
			alice.handle(client.OnPostRcvdNtfn(func(ru *client.RemoteUser, sum clientdb.PostSummary, _ rpc.PostMetadata) {
				aliceMsgChan <- "post_" + ru.Nick() + "_" + sum.Title
			}))
			alice.handle(client.OnPostStatusRcvdNtfn(func(ru *client.RemoteUser, _ clientintf.PostID, statusFrom client.UserID, status rpc.PostMetadataStatus) {
				postFromNick := alice.LocalNick()
				if ru != nil {
					postFromNick = ru.Nick()
				}
				nick := alice.LocalNick()
				if s, _ := alice.UserNick(statusFrom); s != "" {
					nick = s
				}
				comment := status.Attributes[rpc.RMPSComment]
				if idx := strings.LastIndexByte(comment, '#'); idx > -1 {
					// Drop the #<rand> suffix from comment
					comment = comment[:idx]
				}
				aliceMsgChan <- "postcomment_" + postFromNick + "_" + nick + "_" + comment
			}))
			aliceFilteredChan := make(chan string, 10)
			alice.handle(client.OnMsgContentFilteredNtfn(func(e client.MsgContentFilteredEvent) {
				isGCM := e.GC != nil
				isPost := e.PID != nil && !e.IsPostComment
				isPM := !isPost && !e.IsPostComment && !isGCM
				nick := alice.LocalNick()
				if s, _ := alice.UserNick(e.UID); s != "" {
					nick = s
				}
				var typ string
				switch {
				case isPM:
					typ = "pm"
				case isGCM:
					typ = "gcm"
					nick = gcNames[*e.GC] + "_" + nick
				case isPost:
					typ = "post"
				case e.IsPostComment:
					fromNick := alice.LocalNick()
					if s, _ := alice.UserNick(*e.PostFrom); s != "" {
						fromNick = s
					}
					typ = "postcomment"
					nick = fromNick + "_" + nick
				default:
					typ = "unknowntyp"
				}
				aliceFilteredChan <- typ + "_" + nick + "_" + strconv.FormatUint(e.Rule.ID, 10)
			}))

			// Setup helpers to generate messages that should reach Alice.
			sendPM := func(from *testClient, pm string) {
				assert.NilErr(t, from.PM(alice.PublicID(), pm))
			}
			sendGCM := func(from *testClient, gc zkidentity.ShortID, gcm string) {
				assert.NilErr(t, from.GCMessage(gc, gcm, 0, nil))
			}
			sendPost := func(from *testClient, post string) {
				// Send a random descr so that each post is unique.
				descr := fmt.Sprintf("%d", time.Now().UnixMicro())
				_, err := from.CreatePost(post, descr)
				assert.NilErr(t, err)
			}
			sendComment := func(commenter, postFrom *testClient, pid zkidentity.ShortID, comment string) {
				// Use a random parent id so that each comment is unique.
				var parent zkidentity.ShortID
				rand.Read(parent[:])
				comment += fmt.Sprintf("#%d", time.Now().UnixMicro())
				_, err := commenter.CommentPost(postFrom.PublicID(), pid, comment, &parent)
				assert.NilErr(t, err)
			}

			testMsgs := map[string]func(msg string){
				pmBob:              func(msg string) { sendPM(bob, msg) },
				pmCharlie:          func(msg string) { sendPM(charlie, msg) },
				gcmBobGC1:          func(msg string) { sendGCM(bob, gc1, msg) },
				gcmCharlieGC1:      func(msg string) { sendGCM(charlie, gc1, msg) },
				gcmBobGC2:          func(msg string) { sendGCM(bob, gc2, msg) },
				gcmCharlieGC2:      func(msg string) { sendGCM(charlie, gc2, msg) },
				postBob:            func(msg string) { sendPost(bob, msg) },
				postCharlie:        func(msg string) { sendPost(charlie, msg) },
				pcAliceOnAlice:     func(msg string) { sendComment(alice, alice, alicePost, msg) },
				pcBobOnAlice:       func(msg string) { sendComment(bob, alice, alicePost, msg) },
				pcCharlieOnAlice:   func(msg string) { sendComment(charlie, alice, alicePost, msg) },
				pcAliceOnBob:       func(msg string) { sendComment(alice, bob, bobPost, msg) },
				pcBobOnBob:         func(msg string) { sendComment(bob, bob, bobPost, msg) },
				pcCharlieOnBob:     func(msg string) { sendComment(charlie, bob, bobPost, msg) },
				pcAliceOnCharlie:   func(msg string) { sendComment(alice, charlie, charliePost, msg) },
				pcBobOnCharlie:     func(msg string) { sendComment(bob, charlie, charliePost, msg) },
				pcCharlieOnCharlie: func(msg string) { sendComment(charlie, charlie, charliePost, msg) },
			}

			// Extract and sort the keys in testMsgs so that tests are executed
			// always in the same order and in an order that roughly matches how
			// they are defined above (to ease debugging).
			testMsgsKeys := maps.Keys(testMsgs)
			sort.Slice(testMsgsKeys, func(i, j int) bool {
				si, sj := strings.Split(testMsgsKeys[i], "_"), strings.Split(testMsgsKeys[j], "_")
				if len(si) != len(sj) {
					return len(si) < len(sj)
				}
				if len(si[0]) != len(sj[0]) {
					return len(si[0]) < len(sj[0])
				}
				if si[1] != sj[1] {
					return si[1] < sj[1]
				}
				return si[2] < sj[2]
			})

			// Setup the filters.
			assert.NilErr(t, alice.RemoveAllContentFilters())
			for _, cf := range tc.filters {
				if cf.UID != nil && cf.UID.ConstantTimeEq(&pseudoBobId) {
					bobID := bob.PublicID()
					cf.UID = &bobID
				}
				if cf.GC != nil && cf.GC.ConstantTimeEq(&pseudoGC1) {
					cf.GC = &gc1
				}
				assert.NilErr(t, alice.StoreContentFilter(&cf))
			}

			// Send the test message and verify it is filtered as needed by
			// the test case.
			for _, id := range testMsgsKeys {
				f := testMsgs[id]
				filterRule := tc.filtered[id]
				f(testMsg)
				var wasFiltered bool
				var gotVal string
				select {
				case gotVal = <-aliceMsgChan:
				case gotVal = <-aliceFilteredChan:
					wasFiltered = true
				case <-time.After(5 * time.Second):
					t.Fatalf("Timeout waiting for a chan write")
				}
				var wantVal string
				if filterRule == 0 {
					wantVal = id + "_" + testMsg
				} else {
					wantVal = fmt.Sprintf("%s_%d", id, filterRule)
				}

				if wantVal != gotVal {
					t.Fatalf("Unexpected values: got %q, want %q",
						gotVal, wantVal)
				}

				// Uncomment to debug.
				// t.Logf("Message: %30s should/was filtered: %v/%v (%d)", id,
				//	filterRule > 0, wasFiltered, filterRule)
				_ = wasFiltered

				// Delete to track test mistakes.
				delete(tc.filtered, id)
			}
			if len(tc.filtered) > 0 {
				t.Fatalf("More filter assertions in test than performed")
			}

			// Send the unfiltered message and verify it is received.
			for id, f := range testMsgs {
				f(okMsg)
				select {
				case got := <-aliceMsgChan:
					assert.DeepEqual(t, got, id+"_"+okMsg)
				case <-time.After(5 * time.Second):
					t.Fatalf("timeout waiting for okMsg %s", id)
				}
			}
		})
		if !ok {
			return
		}
	}
}
