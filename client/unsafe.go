package client

import (
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/testutils"
)

// FillTestInterface prepares the unsafe test interface. This should not be used
// except in internal tests.
func (c *Client) FillTestInterface(i *testutils.UnsafeTestInterface) {
	i.SendUserRM = func(uid clientintf.UserID, msg interface{}) error {
		ru, err := c.UserByID(uid)
		if err != nil {
			return err
		}
		return ru.sendRM(msg, "testinterface")
	}

	i.QueueUserRM = func(uid clientintf.UserID, msg interface{}) error {
		ru, err := c.UserByID(uid)
		if err != nil {
			return err
		}
		return ru.queueRMPriority(msg, priorityDefault, nil, "testinterface", nil)
	}
}
