package server

// CommandStatus is sent by brserver to brseeder.
type CommandStatus struct {
	LastUpdated int64             `json:"lastUpdated"`
	Database    CommandStatusDB   `json:"db"`
	Node        CommandStatusNode `json:"node"`
}

type CommandStatusDB struct {
	Online bool `json:"db_online"`
	Master bool `json:"db_master"`
}

type CommandStatusNode struct {
	Alias         string `json:"alias"`
	Online        bool   `json:"online"`
	PublicKey     string `json:"publicKey"`
	NumPeers      uint32 `json:"numPeers"`
	BlockHeight   int64  `json:"blockHeight"`
	SyncedToChain bool   `json:"syncedToChain"`
	SyncedToGraph bool   `json:"syncedToGraph"`
}

// CommandStatusReply is the response from brseeder to brserver.
type CommandStatusReply struct {
	Master bool `json:"db_master"`
}
