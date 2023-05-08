package testutils

import "github.com/companyzero/bisonrelay/client/clientintf"

// UnsafeTestInterface is an interface used for exposing internal methods to
// integration tests.
//
// This struct is subject to change without notice and is should not be used
// outside internal/* packages.
type UnsafeTestInterface struct {
	// SendUserRM send a custom RM to a remote user.
	SendUserRM func(uid clientintf.UserID, msg interface{}) error
}
