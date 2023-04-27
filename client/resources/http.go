package resources

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/rpc"
)

// HttpProvider is a resources provider that can fulfill requests to an upstream
// provider via HTTP.
type HttpProvider struct {
	baseURL string
	c       *http.Client
}

func httpHeadersToMeta(h http.Header) map[string]string {
	if h == nil {
		return nil
	}
	res := make(map[string]string, len(h))
	for k, v := range h {
		if len(v) == 0 {
			continue
		}
		res[k] = v[len(v)-1]
	}
	return res
}

func (h *HttpProvider) Fulfill(ctx context.Context, uid clientintf.UserID, request *rpc.RMFetchResource) (*rpc.RMFetchResourceReply, error) {
	path := strings.Join(append([]string{h.baseURL}, request.Path...), "/")
	var body io.Reader
	if request.Data != nil {
		body = bytes.NewBuffer(request.Data)
	}
	upReq, err := http.NewRequestWithContext(ctx, "GET", path, body)
	if err != nil {
		return nil, err
	}
	for k, v := range request.Meta {
		upReq.Header.Add(k, v)
	}
	upReq.Header.Add("X-BisonRelay-UID", uid.String())

	upRes, err := h.c.Do(upReq)
	if err != nil {
		return nil, err
	}

	resBody, err := io.ReadAll(upRes.Body)
	if err != nil {
		return nil, err
	}
	upRes.Body.Close()

	res := &rpc.RMFetchResourceReply{
		Tag:    request.Tag,
		Meta:   httpHeadersToMeta(upRes.Header),
		Status: rpc.ResourceStatus(upRes.StatusCode),
		Index:  0,
		Count:  0,
		Data:   resBody,
	}
	return res, nil
}

// NewHttpProvider creates a new provider that responds to requests by
// forwarding them to an upstream server via HTTP.
func NewHttpProvider(baseURL string) *HttpProvider {
	hp := &HttpProvider{
		baseURL: baseURL,
		c:       &http.Client{},
	}
	return hp
}
