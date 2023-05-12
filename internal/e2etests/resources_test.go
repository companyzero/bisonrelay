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
	tag, err := bob.FetchResource(alice.PublicID(), resourcePath, nil, 0, 0, nil)
	assert.NilErr(t, err)

	// Bob receives the resource.
	res := assert.ChanWritten(t, chanResReply)
	assert.DeepEqual(t, res.Tag, tag)
	assert.DeepEqual(t, res.Data, staticData)

	// Have Bob ask for a resource that does not exist.
	bogusPath := []string{"does", "not", "exist"}
	_, err = bob.FetchResource(alice.PublicID(), bogusPath, nil, 0, 0, nil)
	assert.NilErr(t, err)

	// Bob does not receive a reply.
	assert.ChanNotWritten(t, chanResReply, time.Second)
}
