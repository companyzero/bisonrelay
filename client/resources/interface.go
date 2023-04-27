package resources

import (
	"context"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/rpc"
)

// Provider is the interface for subsystems that provide resources.
type Provider interface {
	Fulfill(ctx context.Context, uid clientintf.UserID, request *rpc.RMFetchResource) (*rpc.RMFetchResourceReply, error)
}

type providerFunc struct {
	f func(ctx context.Context, uid clientintf.UserID, request *rpc.RMFetchResource) (*rpc.RMFetchResourceReply, error)
}

func (pf providerFunc) Fulfill(ctx context.Context, uid clientintf.UserID, request *rpc.RMFetchResource) (*rpc.RMFetchResourceReply, error) {
	return pf.f(ctx, uid, request)
}

// ProviderFunc wraps the passed function as a Provider.
func ProviderFunc(f func(ctx context.Context, uid clientintf.UserID, request *rpc.RMFetchResource) (*rpc.RMFetchResourceReply, error)) Provider {
	return providerFunc{f: f}
}
