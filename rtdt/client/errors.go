package rtdtclient

import "errors"

var (
	errLeftSession               = errors.New("already left session")
	errLeaveSessNoReply          = errors.New("no reply to leaving session")
	errKickFromSessNoReply       = errors.New("no reply to kicking target peer")
	errAdminRotateCookiesNoReply = errors.New("no reply to rotating session cookies")
)
