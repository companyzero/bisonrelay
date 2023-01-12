package jsonrpc

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/companyzero/bisonrelay/clientrpc/types"
	"github.com/decred/slog"
	"golang.org/x/sync/errgroup"
)

// postPeer is a jsonrpc peer over a POST request. It currently only supports
// the server side.
type postPeer struct {
	p       *peer
	dec     *json.Decoder
	enc     *json.Encoder
	flusher func() error
}

func (p *postPeer) nextDecoder() (*json.Decoder, error) {
	return p.dec, nil
}

func (p *postPeer) nextEncoder() (*json.Encoder, error) {
	return p.enc, nil
}

func (p *postPeer) flushLastWrite() error {
	return p.flusher()
}

func (p *postPeer) run(ctx context.Context) error {
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return p.p.run(gctx) })
	return g.Wait()
}

// newServerPostPeer creates a new postPeer for a server request.
func newServerPostPeer(w http.ResponseWriter, r *http.Request, services *types.ServersMap, log slog.Logger) *postPeer {
	enc := json.NewEncoder(w)
	dec := json.NewDecoder(r.Body)
	flusher := func() error { return nil }
	if f, ok := w.(http.Flusher); ok {
		flusher = func() error {
			f.Flush()
			return nil
		}
	}
	p := &postPeer{
		enc:     enc,
		dec:     dec,
		flusher: flusher,
	}
	p.p = newPeer(services, log, p.nextDecoder, p.nextEncoder, p.flushLastWrite)
	return p
}
