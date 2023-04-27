package resources

import (
	"context"
	"errors"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/rpc"
)

var ErrProviderNotFound = errors.New("provider not found for the request")

type routeMatcher func(req *rpc.RMFetchResource) bool

type matcherProvider struct {
	matcher  routeMatcher
	provider Provider
}

// Router is a Provider that matches requests to sub-providers using specific
// rules.
type Router struct {
	matchers []*matcherProvider
}

func (r *Router) bind(m routeMatcher, p Provider) {
	mp := &matcherProvider{
		matcher:  m,
		provider: p,
	}
	r.matchers = append(r.matchers, mp)
}

// BindExactPath binds the passed provider to be called whenever a request
// has an exact path.
func (r *Router) BindExactPath(path []string, p Provider) {
	r.bind(exactPathMatcher(path), p)
}

// BindPrefixPath binds the passed provider to be called whenever a request
// has a path with the passed prefix.
func (r *Router) BindPrefixPath(prefixPath []string, p Provider) {
	r.bind(prefixPathMatcher(prefixPath), p)
}

// FindProvider attempts to find a provider to match the request.
func (r *Router) FindProvider(req *rpc.RMFetchResource) Provider {
	for _, mp := range r.matchers {
		if mp.matcher(req) {
			return mp.provider
		}
	}

	return nil
}

// Fulfill attempts to find a sub-provider to match and fulfill the request.
func (r *Router) Fulfill(ctx context.Context, uid clientintf.UserID, req *rpc.RMFetchResource) (*rpc.RMFetchResourceReply, error) {
	p := r.FindProvider(req)
	if p == nil {
		return nil, ErrProviderNotFound
	}

	return p.Fulfill(ctx, uid, req)
}

// NewRouter initializes a new, empty router.
func NewRouter() *Router {
	return &Router{}
}
