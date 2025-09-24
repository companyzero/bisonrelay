package rpc

// SeederCommandStatus is sent by brserver to brseeder.
type SeederCommandStatus struct {
	LastUpdated int64                   `json:"lastUpdated"`
	Database    SeederCommandStatusDB   `json:"db"`
	Node        SeederCommandStatusNode `json:"node"`
}

type SeederCommandStatusDB struct {
	Online bool `json:"db_online"`
	Master bool `json:"db_master"`
}

type SeederCommandStatusNode struct {
	Alias         string `json:"alias"`
	Online        bool   `json:"online"`
	PublicKey     string `json:"publicKey"`
	NumPeers      uint32 `json:"numPeers"`
	BlockHeight   int64  `json:"blockHeight"`
	SyncedToChain bool   `json:"syncedToChain"`
	SyncedToGraph bool   `json:"syncedToGraph"`
}

// CommandStatusReply is the response from brseeder to brserver.
type SeederCommandStatusReply struct {
	Master bool `json:"db_master"`
}
