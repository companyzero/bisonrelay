package e2etests

import (
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/resources"
	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/rpc"
)

// TestFetchesFixedResource tests fetching a fixed resource.
func TestFetchesFixedResource(t *testing.T) {
	// Setup Alice and Bob and have them KX.
	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")
	ts.kxUsers(alice, bob)

	// Setup Alice's resources handler.
	resourcePath := resources.SplitPath("/path/to/resource")
	staticData := []byte("some static data")
	alice.modifyHandlers(func() {
		r := resources.NewRouter()
		p := &resources.StaticResource{
			Data: staticData,
		}
		r.BindExactPath(resourcePath, p)
		alice.resourcesProvider = r
	})

	// Setup Bob's fetched resource handler.
	chanResReply := make(chan rpc.RMFetchResourceReply, 1)
	bob.handle(client.OnResourceFetchedNtfn(func(user *client.RemoteUser,
		fr clientdb.FetchedResource, sess clientdb.PageSessionOverview) {
		chanResReply <- fr.Response
	}))

	// Have Bob ask for the resource.
	tag, err := bob.FetchResource(alice.PublicID(), resourcePath, nil, 0,
		0, nil, "")
	assert.NilErr(t, err)

	// Bob receives the resource.
	res := assert.ChanWritten(t, chanResReply)
	assert.DeepEqual(t, res.Tag, tag)
	assert.DeepEqual(t, res.Data, staticData)

	// Have Bob ask for a resource that does not exist.
	bogusPath := []string{"does", "not", "exist"}
	_, err = bob.FetchResource(alice.PublicID(), bogusPath, nil, 0, 0,
		nil, "")
	assert.NilErr(t, err)

	// Bob does not receive a reply.
	assert.ChanNotWritten(t, chanResReply, time.Second)
}

// TestFetchesMultipleAsyncTargets tests fetching multiple async targets
// starting from a base page (simulates performing multiple actions in a single
// page).
func TestFetchesMultipleAsyncTargets(t *testing.T) {
	// Setup Alice and Bob and have them KX.
	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")
	ts.kxUsers(alice, bob)

	// Static paths and resources data.
	var (
		rootPath       = resources.SplitPath("/path/to/root")
		rootData       = []byte("root static data")
		asyncPath1     = resources.SplitPath("/async/target/one")
		asyncData1     = []byte("async data one")
		asyncPath2     = resources.SplitPath("/async/target/two")
		asyncData2     = []byte("async data two")
		asyncPath1Req2 = resources.SplitPath("/async/target/one second request")
		asyncData1Req2 = []byte("async data one second request")
		asyncTarget1   = "async target 1"
		asyncTarget2   = "async target 2"
	)

	// Setup Alice's resources handler.
	alice.modifyHandlers(func() {
		r := resources.NewRouter()
		r.BindExactPath(rootPath, &resources.StaticResource{Data: rootData})
		r.BindExactPath(asyncPath1, &resources.StaticResource{Data: asyncData1})
		r.BindExactPath(asyncPath2, &resources.StaticResource{Data: asyncData2})
		r.BindExactPath(asyncPath1Req2, &resources.StaticResource{Data: asyncData1Req2})
		alice.resourcesProvider = r
	})

	// Setup Bob's fetched resource handler.
	chanResReply := make(chan clientdb.FetchedResource, 1)
	bob.handle(client.OnResourceFetchedNtfn(func(user *client.RemoteUser,
		fr clientdb.FetchedResource, sess clientdb.PageSessionOverview) {
		chanResReply <- fr
	}))

	sessID, err := bob.NewPagesSession()
	assert.NilErr(t, err)

	// Bob asks for the root resource.
	tagRoot, err := bob.FetchResource(alice.PublicID(), rootPath, nil, sessID, 0, nil, "")
	assert.NilErr(t, err)

	// Bob receives the resource.
	resRoot := assert.ChanWritten(t, chanResReply)
	assert.DeepEqual(t, resRoot.Response.Tag, tagRoot)
	assert.DeepEqual(t, resRoot.Response.Data, rootData)

	// Alice goes offline.
	assertGoesOffline(t, alice)

	// Bob asks for two async resources. These simulate asking for two
	// async/background actions that were available in the root page.
	tagAsync1, err := bob.FetchResource(alice.PublicID(), asyncPath1, nil,
		sessID, resRoot.PageID, nil, asyncTarget1)
	assert.NilErr(t, err)
	tagAsync2, err := bob.FetchResource(alice.PublicID(), asyncPath2, nil,
		sessID, resRoot.PageID, nil, asyncTarget2)
	assert.NilErr(t, err)

	// Add a hook in Alice to assert the replies are going out.
	aliceSentResReply := make(chan struct{}, 5)
	alice.handle(client.OnRMSent(func(ru *client.RemoteUser, rm interface{}) {
		if _, ok := rm.(rpc.RMFetchResourceReply); ok {
			aliceSentResReply <- struct{}{}
		}
	}))

	// Bob goes offline. Alice comes back online and send their replies.
	assertEmptyRMQ(t, bob)
	assertGoesOffline(t, bob)

	time.Sleep(time.Second)
	assertGoesOnline(t, alice)
	assert.ChanWritten(t, aliceSentResReply)
	assert.ChanWritten(t, aliceSentResReply)
	assertEmptyRMQ(t, alice)

	// Bob comes back online and receives the replies.
	assertGoesOnline(t, bob)
	resAsync1 := assert.ChanWritten(t, chanResReply)
	resAsync2 := assert.ChanWritten(t, chanResReply)
	if resAsync1.Request.Tag == tagAsync2 {
		// Handle case if replies are received out of order.
		resAsync1, resAsync2 = resAsync2, resAsync1
	}

	// Verifiy replies.
	assert.DeepEqual(t, resAsync1.Response.Tag, tagAsync1)
	assert.DeepEqual(t, resAsync1.Response.Data, asyncData1)
	assert.DeepEqual(t, resAsync1.AsyncTargetID, asyncTarget1)
	assert.DeepEqual(t, resAsync1.ParentPage, resRoot.PageID)
	assert.NotDeepEqual(t, resAsync1.PageID, resRoot.PageID)
	assert.DeepEqual(t, resAsync2.Response.Tag, tagAsync2)
	assert.DeepEqual(t, resAsync2.Response.Data, asyncData2)
	assert.DeepEqual(t, resAsync2.AsyncTargetID, asyncTarget2)
	assert.DeepEqual(t, resAsync2.ParentPage, resRoot.PageID)
	assert.NotDeepEqual(t, resAsync2.PageID, resRoot.PageID)
	assert.NotDeepEqual(t, resAsync1.PageID, resAsync2.PageID)

	// Double check Bob can load the original page plus all async requests.
	loaded, err := bob.LoadFetchedResource(alice.PublicID(), sessID, resRoot.PageID)
	assert.NilErr(t, err)
	assert.DeepEqual(t, len(loaded), 3)
	assert.DeepEqual(t, loaded[0].PageID, resRoot.PageID)
	assert.DeepEqual(t, loaded[1].PageID, resAsync1.PageID)
	assert.DeepEqual(t, loaded[2].PageID, resAsync2.PageID)

	// Perform a second async request to the same async target (but a
	// different action, which returns different data).
	tagAsync1Req2, err := bob.FetchResource(alice.PublicID(), asyncPath1Req2, nil,
		sessID, resRoot.PageID, nil, asyncTarget1)
	assert.NilErr(t, err)
	assert.ChanWritten(t, aliceSentResReply)
	resAsync1Req2 := assert.ChanWritten(t, chanResReply)
	assert.DeepEqual(t, resAsync1Req2.Response.Tag, tagAsync1Req2)
	assert.DeepEqual(t, resAsync1Req2.Response.Data, asyncData1Req2)
	assert.DeepEqual(t, resAsync1Req2.AsyncTargetID, asyncTarget1)
	assert.DeepEqual(t, resAsync1Req2.ParentPage, resRoot.PageID)
	assert.NotDeepEqual(t, resAsync1Req2.PageID, resRoot.PageID)
	assert.NotDeepEqual(t, resAsync1Req2.PageID, resAsync1.PageID)

	// Double check Bob can load the original page plus all async requests.
	// The second request replaces the first request target async.
	loaded, err = bob.LoadFetchedResource(alice.PublicID(), sessID, resRoot.PageID)
	assert.NilErr(t, err)
	assert.DeepEqual(t, len(loaded), 3)
	assert.DeepEqual(t, loaded[0].PageID, resRoot.PageID)
	assert.DeepEqual(t, loaded[1].PageID, resAsync1Req2.PageID)
	assert.DeepEqual(t, loaded[2].PageID, resAsync2.PageID)
}
