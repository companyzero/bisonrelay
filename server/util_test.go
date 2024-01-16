package server

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/server/settings"
	"github.com/companyzero/bisonrelay/session"
	"github.com/companyzero/bisonrelay/zkidentity"
)

func newTestServer(t testing.TB) *ZKS {
	t.Helper()

	cfg := settings.New()
	dir, err := os.MkdirTemp("", "br-server")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("Server data location: %s", dir)
		} else {
			os.RemoveAll(dir)
		}
	})

	cfg.Root = dir
	cfg.RoutedMessages = filepath.Join(dir, settings.ZKSRoutedMessages)
	cfg.LogFile = filepath.Join(dir, "brserver.log")
	cfg.Listen = []string{"127.0.0.1:0"}
	cfg.InitSessTimeout = time.Second
	cfg.DebugLevel = "off"

	s, err := NewServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func runTestServer(t testing.TB, svr *ZKS) chan error {
	c := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() {
		err := svr.Run(ctx)
		select {
		case c <- err:
		case <-time.After(30 * time.Second):
			// Avoid leaking goroutine for too long if c isn't read
			// from.
		}
	}()
	return c
}

func serverBoundAddr(t testing.TB, svr *ZKS) string {
	t.Helper()
	for i := 0; i < 100; i++ {
		addrs := svr.BoundAddrs()
		if len(addrs) == 0 {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		return addrs[0].String()
	}
	t.Fatal("Timeout waiting for server address")
	return ""
}

// kxServerConn returns a fully KXd session with the server at the other end
// of the passed conn.
func kxServerConn(t testing.TB, conn clientintf.Conn) *session.KX {
	t.Helper()
	var pid zkidentity.PublicIdentity

	// tell remote we want its public identity
	err := json.NewEncoder(conn).Encode(rpc.InitialCmdIdentify)
	if err != nil {
		t.Fatal(err)
	}

	// get server identity
	err = json.NewDecoder(conn).Decode(&pid)
	if err != nil {
		t.Fatal(err)
	}

	// Create the KX session w/ the server.
	err = json.NewEncoder(conn).Encode(rpc.InitialCmdSession)
	if err != nil {
		t.Fatal(err)
	}

	// Session with server and use a default msgSize.
	kx := &session.KX{
		Conn:           conn,
		MaxMessageSize: 1887437,
		TheirPublicKey: &pid.Key,
	}
	err = kx.Initiate()
	if err != nil {
		t.Fatal(err)
	}

	var (
		command rpc.Message
	)

	// Read welcome.
	b, err := kx.Read()
	if err != nil {
		t.Fatal(err)
	}

	// Unmarshal header.
	br := bytes.NewReader(b)
	dec := json.NewDecoder(br)
	err = dec.Decode(&command)
	if err != nil {
		t.Fatal(err)
	}

	switch command.Command {
	case rpc.SessionCmdWelcome:
		// fallthrough
	default:
		t.Fatal(err)
	}

	return kx
}

func writeServerMsg(t testing.TB, kx *session.KX, msg rpc.Message, payload interface{}) {
	var bb bytes.Buffer
	enc := json.NewEncoder(&bb)
	err := enc.Encode(msg)
	if err != nil {
		t.Fatalf("could not marshal message '%v': %v", msg.Command, err)
	}
	err = enc.Encode(payload)
	if err != nil {
		t.Fatalf("could not marshal payload '%v': %v", msg.Command, err)
	}

	b := bb.Bytes()
	err = kx.Write(b)
	if err != nil {
		t.Fatalf("could not write '%v': %v",
			msg.Command, err)
	}
}

func decodeServerMsg(t testing.TB, rawMsg []byte) (rpc.Message, interface{}) {
	t.Helper()
	var msg rpc.Message
	br := bytes.NewReader(rawMsg)
	dec := json.NewDecoder(br)
	err := dec.Decode(&msg)
	if err != nil {
		t.Fatalf("unable to unmarshal header: %v", err)
	}

	payload, err := decodeRPCPayload(&msg, dec)
	if err != nil {
		t.Fatalf("unable to decode payload: %v", err)
	}

	return msg, payload
}
