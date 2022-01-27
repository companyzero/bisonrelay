// Copyright (c) 2016 Company 0, LLC.
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package disk

type RatchetState struct {
	RootKey            []byte                   `json:"rootKey"`
	SendHeaderKey      []byte                   `json:"sendHeaderKey"`
	RecvHeaderKey      []byte                   `json:"recvHeaderKey"`
	NextSendHeaderKey  []byte                   `json:"nextSendHeaderKey"`
	NextRecvHeaderKey  []byte                   `json:"nextRecvHeaderKey"`
	PrevRecvHeaderKey  []byte                   `json:"prevRecvHeaderKey"`
	SendChainKey       []byte                   `json:"sendChainKey"`
	RecvChainKey       []byte                   `json:"recvChainKey"`
	SendRatchetPrivate []byte                   `json:"sendPrivate"`
	RecvRatchetPublic  []byte                   `json:"recvPublic"`
	SendCount          uint32                   `json:"sendCount"`
	RecvCount          uint32                   `json:"recvCount"`
	PrevSendCount      uint32                   `json:"prevSendCount"`
	PrevRecvCount      uint32                   `json:"prevRecvCount"`
	Ratchet            bool                     `json:"ratchet"`
	KXPrivate          []byte                   `json:"kxprivate"`
	MyHalf             []byte                   `json:"myHalf"`
	TheirHalf          []byte                   `json:"theirHalf"`
	SavedKeys          []RatchetState_SavedKeys `json:"savedKeys"`
	LastEncryptTime    int64                    `json:"lastEncryptTime"`
	LastDecryptTime    int64                    `json:"lastDencryptTime"`
}

type RatchetState_SavedKeys struct {
	HeaderKey   []byte                              `json:"headerKey"`
	MessageKeys []RatchetState_SavedKeys_MessageKey `json:"messageKeys"`
}

type RatchetState_SavedKeys_MessageKey struct {
	Num          uint32 `json:"num"`
	Key          []byte `json:"key"`
	CreationTime int64  `json:"creationTime"`
}
