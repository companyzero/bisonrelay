package golib

import (
	"encoding/json"

	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/client/resources/simplestore"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrlnd/lnrpc"
	lpclient "github.com/decred/dcrlnlpd/client"
)

type InitClient struct {
	ServerAddr        string `json:"server_addr"`
	DBRoot            string `json:"dbroot"`
	DownloadsDir      string `json:"downloads_dir"`
	LNRPCHost         string `json:"ln_rpc_host"`
	LNTLSCertPath     string `json:"ln_tls_cert_path"`
	LNMacaroonPath    string `json:"ln_macaroon_path"`
	LogFile           string `json:"log_file"`
	MsgsRoot          string `json:"msgs_root"`
	DebugLevel        string `json:"debug_level"`
	WantsLogNtfns     bool   `json:"wants_log_ntfns"`
	ResourcesUpstream string `json:"resources_upstream"`

	SimpleStorePayType    string  `json:"simplestore_pay_type"`
	SimpleStoreAccount    string  `json:"simplestore_account"`
	SimpleStoreShipCharge float64 `json:"simplestore_ship_charge"`

	ProxyAddr     string `json:"proxyaddr"`
	ProxyUsername string `json:"proxy_username"`
	ProxyPassword string `json:"proxy_password"`
	TorIsolation  bool   `json:"torisolation"`
	CircuitLimit  uint32 `json:"circuit_limit"`
}

type IDInit struct {
	Nick string `json:"nick"`
	Name string `json:"name"`
}

type LocalInfo struct {
	ID   clientintf.UserID `json:"id"`
	Nick string            `json:"nick"`
}

type ServerCert struct {
	InnerFingerprint string `json:"inner_fingerprint"`
	OuterFingerprint string `json:"outer_fingerprint"`
}

const (
	ConnStateOffline        = 0
	ConnStateCheckingWallet = 1
	ConnStateOnline         = 2
)

type ServerSessState struct {
	State          int     `json:"state"`
	CheckWalletErr *string `json:"check_wallet_err"`
}

type PM struct {
	UID       clientintf.UserID `json:"sid"` // sid == source id
	Msg       string            `json:"msg"`
	Mine      bool              `json:"mine"`
	TimeStamp int64             `json:"timestamp"`
}

type RemoteUser struct {
	UID  string `json:"uid"`
	Name string `json:"name"`
	Nick string `json:"nick"`
}

func remoteUserFromPII(pii *zkidentity.PublicIdentity) RemoteUser {
	return RemoteUser{
		UID:  pii.Identity.String(),
		Name: pii.Name,
		Nick: pii.Nick,
	}
}

type InviteToGC struct {
	GC  zkidentity.ShortID `json:"gc"`
	UID clientdb.UserID    `json:"uid"`
}

type GCAddressBookEntry struct {
	ID      zkidentity.ShortID  `json:"id"`
	Name    string              `json:"name"`
	Members []clientintf.UserID `json:"members"`
}

type GCInvitation struct {
	Inviter RemoteUser `json:"inviter"`
	IID     uint64     `json:"iid"`
	Name    string     `json:"name"`
}

type GCMessage struct {
	SenderUID clientdb.UserID `json:"sender_uid"`
	ID        string          `json:"sid"` // sid == source id == gc name
	Msg       string          `json:"msg"`
	TimeStamp int64           `json:"timestamp"`
}

type GCMessageToSend struct {
	GC  zkidentity.ShortID `json:"gc"`
	Msg string             `json:"msg"`
}

type GCRemoveUserArgs struct {
	GC  zkidentity.ShortID `json:"gc"`
	UID clientintf.UserID  `json:"uid"`
}

type ShareFileArgs struct {
	Filename    string `json:"filename"`
	UID         string `json:"uid"`
	Cost        uint64 `json:"cost"`
	Description string `json:"description"`
}

type UnshareFileArgs struct {
	FID zkidentity.ShortID `json:"fid"`
	UID *clientintf.UserID `json:"uid"`
}

type GetRemoteFileArgs struct {
	UID clientintf.UserID  `json:"uid"`
	FID zkidentity.ShortID `json:"fid"`
}

type PayTipArgs struct {
	UID    clientintf.UserID `json:"uid"`
	Amount float64           `json:"amount"`
}

type PostReceived struct {
	UID      clientintf.UserID `json:"uid"`
	PostMeta rpc.PostMetadata  `json:"post_meta"`
}

type ReadPostArgs struct {
	From clientintf.UserID `json:"from"`
	PID  clientintf.PostID `json:"pid"`
}

type CommentPostArgs struct {
	From    clientintf.UserID  `json:"from"`
	PID     clientintf.PostID  `json:"pid"`
	Comment string             `json:"comment"`
	Parent  *clientintf.PostID `json:"parent,omitempty"`
}

type PostStatusReceived struct {
	PostFrom   clientintf.UserID      `json:"post_from"`
	PID        clientintf.PostID      `json:"pid"`
	StatusFrom clientintf.UserID      `json:"status_from"`
	Status     rpc.PostMetadataStatus `json:"status"`
	Mine       bool                   `json:"mine"`
}

type ChatLogEntry struct {
	Message   string `json:"message"`
	From      string `json:"from"`
	Timestamp int64  `json:"timestamp"`
	Internal  bool   `json:"internal"`
}
type MediateIDArgs struct {
	Mediator clientintf.UserID `json:"mediator"`
	Target   clientintf.UserID `json:"target"`
}

type PostActionArgs struct {
	From clientintf.UserID `json:"from"`
	PID  clientintf.PostID `json:"pid"`
}

type FileDownloadProgress struct {
	UID             clientintf.UserID `json:"uid"`
	FID             clientdb.FileID   `json:"fid"`
	Metadata        rpc.FileMetadata  `json:"metadata"`
	NbMissingChunks int               `json:"nb_missing_chunks"`
}

type LNBalances struct {
	Channel *lnrpc.ChannelBalanceResponse `json:"channel"`
	Wallet  *lnrpc.WalletBalanceResponse  `json:"wallet"`
}

type LNChannelPoint struct {
	Txid        string `json:"txid"`
	OutputIndex int    `json:"output_index"`
}

type LNCloseChannelRequest struct {
	ChannelPoint LNChannelPoint `json:"channel_point"`
	Force        bool           `json:"force"`
}

type LNPayInvoiceRequest struct {
	PaymentRequest string `json:"payment_request"`
	Amount         int64  `json:"amount"`
}

type LNTryExternalDcrlnd struct {
	RPCHost      string `json:"rpc_host"`
	TLSCertPath  string `json:"tls_cert_path"`
	MacaroonPath string `json:"macaroon_path"`
}

type LNInitDcrlnd struct {
	RootDir         string   `json:"root_dir"`
	Network         string   `json:"network"`
	Password        string   `json:"password"`
	ExistingSeed    []string `json:"existingseed"`
	MultiChanBackup []byte   `json:"multichanbackup"`
	ProxyAddr       string   `json:"proxyaddr"`
	TorIsolation    bool     `json:"torisolation"`
}

type LNNewWalletSeed struct {
	Seed    string `json:"seed"`
	RPCHost string `json:"rpc_host"`
}

type LNReqChannelArgs struct {
	Server       string `json:"server"`
	Key          string `json:"key"`
	ChanSize     uint64 `json:"chan_size"`
	Certificates string `json:"certificates"`
}

type LNReqChannelEstValue struct {
	Amount       uint64                `json:"amount"`
	ServerPolicy lpclient.ServerPolicy `json:"server_policy"`
}

type ConfirmFileDownload struct {
	UID      clientintf.UserID  `json:"uid"`
	FID      zkidentity.ShortID `json:"fid"`
	Metadata rpc.FileMetadata   `json:"metadata"`
}

type ConfirmFileDownloadReply struct {
	FID   zkidentity.ShortID `json:"fid"`
	Reply bool               `json:"reply"`
}

type SendFileArgs struct {
	UID      clientintf.UserID `json:"uid"`
	Filepath string            `json:"filepath"`
}

type UserPostList struct {
	UID   clientintf.UserID  `json:"uid"`
	Posts []rpc.PostListItem `json:"posts"`
}

type UserContentList struct {
	UID   clientintf.UserID     `json:"uid"`
	Files []clientdb.RemoteFile `json:"files"`
}

type LocalRenameArgs struct {
	ID      zkidentity.ShortID `json:"id"`
	NewName string             `json:"new_name"`
	IsGC    bool               `json:"is_gc"`
}

type PostSubscriptionResult struct {
	ID            zkidentity.ShortID `json:"id"`
	WasSubRequest bool               `json:"was_sub_request"`
	Error         string             `json:"error"`
}

type LastUserReceivedTime struct {
	UID           clientintf.UserID `json:"uid"`
	LastDecrypted int64             `json:"last_decrypted"`
}

type InvoiceGenFailed struct {
	UID       clientintf.UserID `json:"uid"`
	Nick      string            `json:"nick"`
	DcrAmount float64           `json:"dcr_amount"`
	Err       string            `json:"err"`
}

type GCVersionWarn struct {
	ID         zkidentity.ShortID `json:"id"`
	Alias      string             `json:"alias"`
	Version    uint8              `json:"version"`
	MinVersion uint8              `json:"min_version"`
	MaxVersion uint8              `json:"max_version"`
}

type GCAddedMembers struct {
	ID   zkidentity.ShortID   `json:"id"`
	UIDs []zkidentity.ShortID `json:"uids"`
}

type GCUpgradedVersion struct {
	ID         zkidentity.ShortID `json:"id"`
	OldVersion uint8              `json:"old_version"`
	NewVersion uint8              `json:"new_version"`
}

type GCMemberParted struct {
	GCID   zkidentity.ShortID `json:"gcid"`
	UID    zkidentity.ShortID `json:"uid"`
	Reason string             `json:"reason"`
	Kicked bool               `json:"kicked"`
}

type GCModifyAdmins struct {
	GCID      zkidentity.ShortID   `json:"gcid"`
	NewAdmins []zkidentity.ShortID `json:"new_admins"`
}

type GCAdminsChanged struct {
	GCID    zkidentity.ShortID   `json:"gcid"`
	Source  zkidentity.ShortID   `json:"source"`
	Added   []zkidentity.ShortID `json:"added"`
	Removed []zkidentity.ShortID `json:"removed"`
}

type SubscribeToPosts struct {
	Target    clientintf.UserID  `json:"target"`
	FetchPost *clientintf.PostID `json:"fetch_post"`
}

type SuggestKX struct {
	AlreadyKnown bool               `json:"alreadyknown"`
	InviteeNick  string             `json:"inviteenick"`
	Invitee      zkidentity.ShortID `json:"inviteeid"`
	TargetNick   string             `json:"targetnick"`
	Target       zkidentity.ShortID `json:"targetid"`
}

type Account struct {
	Name               string         `json:"name"`
	UnconfirmedBalance dcrutil.Amount `json:"unconfirmed_balance"`
	ConfirmedBalance   dcrutil.Amount `json:"confirmed_balance"`
	InternalKeyCount   uint32         `json:"internal_key_count"`
	ExternalKeyCount   uint32         `json:"external_key_count"`
}

type SendOnChain struct {
	Addr        string         `json:"addr"`
	Amount      dcrutil.Amount `json:"amount"`
	FromAccount string         `json:"from_account"`
}

type WriteInvite struct {
	FundAmount  dcrutil.Amount      `json:"fund_amount"`
	FundAccount string              `json:"fund_account"`
	GCID        *zkidentity.ShortID `json:"gc_id"`
}

type GeneratedKXInvite struct {
	Blob  []byte                   `json:"blob"`
	Funds *rpc.InviteFunds         `json:"funds"`
	Key   clientintf.PaidInviteKey `json:"key"`
}

type RedeemedInviteFunds struct {
	Txid  rpc.TxHash     `json:"txid"`
	Total dcrutil.Amount `json:"total"`
}

type Invitation struct {
	Blob   []byte                      `json:"blob"`
	Invite rpc.OOBPublicIdentityInvite `json:"invite"`
}

type FetchResourceArgs struct {
	UID        clientintf.UserID         `json:"uid"`
	Path       []string                  `json:"path"`
	Metadata   map[string]string         `json:"metadata,omitempty"`
	SessionID  clientintf.PagesSessionID `json:"session_id"`
	ParentPage clientintf.PagesSessionID `json:"parent_page"`
	Data       json.RawMessage           `json:"data"`
}

type SimpleStoreOrder struct {
	Order simplestore.Order `json:"order"`
	Msg   string            `json:"msg"`
}

type HandshakeStage struct {
	UID   clientintf.UserID `json:"uid"`
	Stage string            `json:"stage"`
}

type LoadUserHistory struct {
	UID    clientintf.UserID `json:"uid"`
	GcName string            `json:"gc_name"`
}
