package e2etests

import (
	"sync/atomic"

	"github.com/decred/dcrlnd/lnrpc"
)

type mockGetInfoResponse struct {
	res *lnrpc.GetInfoResponse
	err error
}

type mockLnrpc struct {
	info atomic.Pointer[mockGetInfoResponse]
}

func (ln *mockLnrpc) setGetInfoResponse(res *lnrpc.GetInfoResponse, err error) {
	ln.info.Store(&mockGetInfoResponse{
		res: res,
		err: err,
	})
}

func (ln *mockLnrpc) GetInfo() (*lnrpc.GetInfoResponse, error) {
	res := ln.info.Load()
	return res.res, res.err
}
