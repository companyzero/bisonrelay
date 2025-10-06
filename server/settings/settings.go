package settings

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"strconv"
	"strings"
	"time"

	"github.com/companyzero/bisonrelay/rpc"
	brpgdb "github.com/companyzero/bisonrelay/server/internal/pgdb"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/decred/dcrlnd/lnrpc"
	"github.com/vaughan0/go-ini"
	strduration "github.com/xhit/go-str2duration/v2"
)

const (
	// The following constants define server-related files and dirs.

	ZKSIdentityFilename = "brserver.id"
	ZKSCertFilename     = "brserver.crt"
	ZKSKeyFilename      = "brserver.key"
	ZKSRoutedMessages   = "routedmessages"
	ZKSPaidRVs          = "paidrvs"
)

// Settings is the collection of all brserver settings.  This is separated out
// in order to be able to reuse in various tests.
type Settings struct {
	// default section
	Root            string        // root directory for brserver
	RoutedMessages  string        // routed messages
	PaidRVs         string        // paid for RVs
	Listen          []string      // listen addresses and port
	InitSessTimeout time.Duration // How long to wait for session on a new connection
	ClientVersions  string        // Suggested list of client versions (comma separated key=value)

	// policy section
	ExpirationDays    int // How many days after which to expire data
	MaxMsgSizeVersion rpc.MaxMsgSizeVersion
	PingLimit         time.Duration

	// payment section
	PayScheme               string
	LNRPCHost               string
	LNTLSCert               string
	LNMacaroonPath          string
	PushPayRateMAtoms       uint64
	PushPayRateBytes        uint64
	PushPayRateMinMAtoms    uint64
	MilliAtomsPerSub        uint64
	PushPaymentLifetime     int // how long a payment to a push is valid
	MaxPushInvoices         int
	MilliAtomsPerRTSess     uint64
	MilliAtomsPerUserRTSess uint64
	MilliAtomsGetCookie     uint64
	MilliAtomsPerUserCookie uint64
	MilliAtomsRTJoin        uint64
	MilliAtomsRTPushRate    uint64
	RTPushRateMBytes        uint64

	// log section
	LogFile    string // log filename
	DebugLevel string // debug level config string
	TimeFormat string // debug file time stamp format
	Profiler   string // go profiler link

	// Postgres config
	PGEnabled         bool
	PGHost            string
	PGPort            string
	PGDBName          string
	PGRoleName        string
	PGPassphrase      string
	PGServerCA        string
	PGIndexTableSpace string
	PGBulkTableSpace  string

	// RTDT config
	RTDTServerAddr       string
	RTDTServerPub        *zkidentity.FixedSizeSntrupPublicKey
	RTDTCookieKey        *zkidentity.FixedSizeSymmetricKey
	RTDTDecodeCookieKeys []*zkidentity.FixedSizeSymmetricKey

	// Versioner is a function that returns the current app version.
	Versioner func() string

	// LogStdOut is the stdout to write the log to. Defaults to os.Stdout.
	LogStdOut io.Writer

	// SeederAddr is the websocket host of the brseeder.
	SeederAddr string

	// SeederToken is the token from the brseeder.
	SeederToken string

	// SeederDisable disables the use of brseeder.
	SeederDisable bool

	// SeederDryRun pretends to follow seeder's commands.
	SeederDryRun bool

	// SeederInsecure may be set to true to use ws:// instead of wss:// as
	// protocol.
	SeederInsecure bool

	// LNRpcGetInfoMock may be set during tests to mock the lnRpc.GetInfo()
	// call.
	LNRpcGetInfoMock func() (*lnrpc.GetInfoResponse, error)
}

var (
	errIniNotFound = errors.New("not found")
)

// New returns a default settings structure.
func New() *Settings {
	return &Settings{
		// default
		Root:            "~/.brserver",
		RoutedMessages:  "~/.brserver/" + ZKSRoutedMessages,
		PaidRVs:         "~/.brserver/" + ZKSPaidRVs,
		Listen:          []string{"127.0.0.1:443"},
		InitSessTimeout: time.Second * 20,
		ClientVersions:  "",

		// Policy
		ExpirationDays:    rpc.PropExpirationDaysDefault,
		MaxMsgSizeVersion: rpc.PropMaxMsgSizeVersionDefault,
		PingLimit:         rpc.PropPingLimitDefault,

		// payment
		PayScheme:               "free",
		PushPayRateMAtoms:       rpc.PropPushPaymentRateDefault,
		PushPayRateBytes:        rpc.PropPushPaymentRateBytesDefault,
		PushPayRateMinMAtoms:    rpc.MinRMPushPayment, // Not currently exposed for config
		MilliAtomsPerSub:        rpc.PropSubPaymentRateDefault,
		PushPaymentLifetime:     rpc.PropPushPaymentLifetimeDefault,
		MaxPushInvoices:         rpc.PropMaxPushInvoicesDefault,
		MilliAtomsPerRTSess:     1000,
		MilliAtomsPerUserRTSess: 1000,
		MilliAtomsGetCookie:     1000,
		MilliAtomsPerUserCookie: 100,
		MilliAtomsRTJoin:        1000,
		MilliAtomsRTPushRate:    100,
		RTPushRateMBytes:        1,

		// log
		LogFile:    "~/.brserver/brserver.log",
		DebugLevel: "info",
		TimeFormat: "2006-01-02 15:04:05",
		Profiler:   "localhost:6060",

		PGEnabled:         false,
		PGHost:            brpgdb.DefaultHost,
		PGPort:            brpgdb.DefaultPort,
		PGDBName:          brpgdb.DefaultDBName,
		PGRoleName:        brpgdb.DefaultRoleName,
		PGPassphrase:      brpgdb.DefaultRoleName,
		PGIndexTableSpace: brpgdb.DefaultIndexTablespaceName,
		PGBulkTableSpace:  brpgdb.DefaultBulkDataTablespaceName,

		Versioner: func() string { return "" },
		LogStdOut: os.Stdout,

		SeederDisable: true,
		SeederDryRun:  true,
	}
}

func (s *Settings) SeederProto() string {
	if s.SeederInsecure {
		return "ws"
	} else {
		return "wss"
	}
}

// Load retrieves settings from an ini file.  Additionally it expands all ~ to
// the current user home directory.
func (s *Settings) Load(filename string) error {
	// parse file
	cfg, err := ini.LoadFile(filename)
	if err != nil {
		return err
	}

	get := func(s *string, section, field string) {
		v, ok := cfg.Get(section, field)
		if ok {
			*s = v
		}
	}

	// obtain current user for directory expansion
	usr, err := user.Current()
	if err != nil {
		return err
	}

	// root directory
	root, ok := cfg.Get("", "root")
	if ok {
		s.Root = root
	}
	s.Root = strings.Replace(s.Root, "~", usr.HomeDir, 1)

	// routedmessages directory
	routedmessages, ok := cfg.Get("", "routedmessages")
	if ok {
		s.RoutedMessages = routedmessages
	}
	s.RoutedMessages = strings.Replace(s.RoutedMessages, "~", usr.HomeDir, 1)

	// paidrvs directory
	paidrvs, ok := cfg.Get("", "paidrvs")
	if ok {
		s.PaidRVs = paidrvs
	}
	s.PaidRVs = strings.Replace(s.PaidRVs, "~", usr.HomeDir, 1)

	// listen address
	rawListen, ok := cfg.Get("", "listen")
	if ok {
		listenList := strings.Split(rawListen, ",")
		for i := range listenList {
			listenList[i] = strings.TrimSpace(listenList[i])
		}
		s.Listen = listenList
	}

	// Suggested client versions.
	s.ClientVersions, _ = cfg.Get("", "clientversions")

	// logging and debug
	logFile, ok := cfg.Get("log", "logfile")
	if ok {
		s.LogFile = logFile
	}
	s.LogFile = strings.Replace(s.LogFile, "~", usr.HomeDir, 1)

	debugLevel, ok := cfg.Get("log", "debuglevel")
	if ok {
		s.DebugLevel = debugLevel
	}

	timeFormat, ok := cfg.Get("log", "timeformat")
	if ok {
		s.TimeFormat = timeFormat
	}

	profiler, ok := cfg.Get("log", "profiler")
	if ok {
		s.Profiler = profiler
	}

	payScheme, ok := cfg.Get("payment", "scheme")
	if ok {
		s.PayScheme = payScheme
	}

	lnRPCHost, ok := cfg.Get("payment", "lnrpchost")
	if ok {
		s.LNRPCHost = strings.Replace(lnRPCHost, "~", usr.HomeDir, 1)
	}

	lnTLSCert, ok := cfg.Get("payment", "lntlscert")
	if ok {
		s.LNTLSCert = strings.Replace(lnTLSCert, "~", usr.HomeDir, 1)
	}

	lnMacaroonPath, ok := cfg.Get("payment", "lnmacaroonpath")
	if ok {
		s.LNMacaroonPath = strings.Replace(lnMacaroonPath, "~", usr.HomeDir, 1)
	}

	var pushRateAtoms float64 = rpc.PropPushPaymentRateDefault
	err = iniFloat(cfg, &pushRateAtoms, "payment", "pushrateatoms")
	if err != nil && !errors.Is(err, errIniNotFound) {
		return err
	}
	s.PushPayRateMAtoms = uint64(pushRateAtoms * 1000)
	iniUint64(cfg, &s.PushPayRateBytes, "payment", "pushratebytes")
	if s.PushPayRateBytes < 1 {
		return errors.New("pushratebytes cannot be < 1")
	}

	var atomsPerSub float64 = rpc.PropSubPaymentRateDefault / 1000
	err = iniFloat(cfg, &atomsPerSub, "payment", "atomspersub")
	if err != nil && !errors.Is(err, errIniNotFound) {
		return err
	}
	s.MilliAtomsPerSub = uint64(atomsPerSub * 1000)

	err = iniBool(cfg, &s.PGEnabled, "postgres", "enabled")
	if err != nil && !errors.Is(err, errIniNotFound) {
		return err
	}

	get(&s.PGHost, "postgres", "host")
	get(&s.PGPort, "postgres", "port")
	get(&s.PGDBName, "postgres", "dbname")
	get(&s.PGRoleName, "postgres", "role")
	get(&s.PGPassphrase, "postgres", "pass")
	get(&s.PGServerCA, "postgres", "serverca")
	get(&s.PGIndexTableSpace, "postgres", "indexts")
	get(&s.PGBulkTableSpace, "postgres", "bulkts")

	get(&s.RTDTServerAddr, "rtdt", "serveraddress")

	var rtCookieKeyStr string
	get(&rtCookieKeyStr, "rtdt", "cookiekey")
	if rtCookieKeyStr != "" {
		var cookieKey zkidentity.FixedSizeSymmetricKey
		if err := cookieKey.FromString(rtCookieKeyStr); err != nil {
			return err
		}
		s.RTDTCookieKey = &cookieKey
	}

	var rtDecodeCookieKeysStr string
	get(&rtDecodeCookieKeysStr, "rtdt", "decodecookiekeys")
	if rtDecodeCookieKeysStr != "" {
		keys := strings.Split(rtDecodeCookieKeysStr, ",")
		for _, key := range keys {
			cookieKey := new(zkidentity.FixedSizeSymmetricKey)
			key = strings.TrimSpace(key)
			if err := cookieKey.FromString(key); err != nil {
				return err
			}
			s.RTDTDecodeCookieKeys = append(s.RTDTDecodeCookieKeys, cookieKey)
		}
	}

	var rtdtServerPubFilename string
	get(&rtdtServerPubFilename, "rtdt", "serverpub")
	if rtdtServerPubFilename != "" {
		data, err := os.ReadFile(rtdtServerPubFilename)
		if err != nil {
			return err
		}
		s.RTDTServerPub = new(zkidentity.FixedSizeSntrupPublicKey)
		copy(s.RTDTServerPub[:], data)
	}

	expirationDays := rpc.PropExpirationDaysDefault
	err = iniInt(cfg, &expirationDays, "policy", "expirationdays")
	if err != nil && !errors.Is(err, errIniNotFound) {
		return err
	}
	s.ExpirationDays = expirationDays

	pushPaymentLifetime := rpc.PropPushPaymentLifetimeDefault
	err = iniInt(cfg, &pushPaymentLifetime, "policy", "pushpaymentlifetime")
	if err != nil && !errors.Is(err, errIniNotFound) {
		return err
	}
	s.PushPaymentLifetime = pushPaymentLifetime

	maxPushInvoices := rpc.PropMaxPushInvoicesDefault
	err = iniInt(cfg, &pushPaymentLifetime, "policy", "maxpushinvoices")
	if err != nil && !errors.Is(err, errIniNotFound) {
		return err
	}
	s.MaxPushInvoices = maxPushInvoices

	err = iniDuration(cfg, &s.PingLimit, "policy", "pinglimit")
	if err != nil && !errors.Is(err, errIniNotFound) {
		return err
	}
	if s.PingLimit < time.Second {
		return fmt.Errorf("pinglimit must be at least one second")
	}

	err = iniMaxMsgSize(cfg, &s.MaxMsgSizeVersion, "policy", "maxmsgsizeversion")
	if err != nil && !errors.Is(err, errIniNotFound) {
		return err
	}
	if size := rpc.MaxMsgSizeForVersion(s.MaxMsgSizeVersion); size == 0 {
		return fmt.Errorf("unsupported max msg size version")
	} else {
		// Double check a message of the max valid size is payable.
		_, err := rpc.CalcPushCostMAtoms(s.PushPayRateMinMAtoms, s.PushPayRateMAtoms, s.PushPayRateBytes, uint64(size))
		if err != nil {
			return fmt.Errorf("invalid combination of push pay rates and max msg size: %v", err)
		}
	}

	err = iniUint64(cfg, &s.MilliAtomsPerRTSess, "policy", "rtmatomspersess")
	if err != nil && !errors.Is(err, errIniNotFound) {
		return err
	}
	err = iniUint64(cfg, &s.MilliAtomsPerUserRTSess, "policy", "rtmatomsperusersess")
	if err != nil && !errors.Is(err, errIniNotFound) {
		return err
	}
	err = iniUint64(cfg, &s.MilliAtomsGetCookie, "policy", "rtmatomsgetcookie")
	if err != nil && !errors.Is(err, errIniNotFound) {
		return err
	}
	err = iniUint64(cfg, &s.MilliAtomsPerUserCookie, "policy", "rtmatomsperusercookie")
	if err != nil && !errors.Is(err, errIniNotFound) {
		return err
	}
	err = iniUint64(cfg, &s.MilliAtomsRTJoin, "policy", "rtmatomsjoinsess")
	if err != nil && !errors.Is(err, errIniNotFound) {
		return err
	}
	err = iniUint64(cfg, &s.MilliAtomsRTPushRate, "policy", "rtmatomspushrate")
	if err != nil && !errors.Is(err, errIniNotFound) {
		return err
	}
	err = iniUint64(cfg, &s.RTPushRateMBytes, "policy", "rtpushratembytes")
	if err != nil && !errors.Is(err, errIniNotFound) {
		return err
	}

	seederAddr, ok := cfg.Get("seeder", "host")
	if ok {
		s.SeederAddr = seederAddr
	}
	seederToken, ok := cfg.Get("seeder", "token")
	if ok {
		s.SeederToken = seederToken
	}

	err = iniBool(cfg, &s.SeederDisable, "seeder", "disable")
	if err != nil && !errors.Is(err, errIniNotFound) {
		return err
	}
	err = iniBool(cfg, &s.SeederDryRun, "seeder", "dryrun")
	if err != nil && !errors.Is(err, errIniNotFound) {
		return err
	}
	err = iniBool(cfg, &s.SeederInsecure, "seeder", "insecure")
	if err != nil && !errors.Is(err, errIniNotFound) {
		return err
	}

	if !s.SeederDisable {
		if s.SeederAddr == "" {
			return fmt.Errorf("no seeder address set")
		}
		if s.SeederToken == "" {
			return fmt.Errorf("no seeder token set")
		}
	}

	return nil
}

func iniBool(cfg ini.File, p *bool, section, key string) error {
	v, ok := cfg.Get(section, key)
	if ok {
		switch strings.ToLower(v) {
		case "yes":
			*p = true
			return nil
		case "no":
			*p = false
			return nil
		default:
			return fmt.Errorf("[%v]%v must be yes or no",
				section, key)
		}
	}
	return errIniNotFound
}

func iniFloat(cfg ini.File, p *float64, section, key string) error {
	v, ok := cfg.Get(section, key)
	if !ok {
		return errIniNotFound
	}

	var err error
	*p, err = strconv.ParseFloat(v, 64)
	return err
}

func iniInt(cfg ini.File, p *int, section, key string) error {
	v, ok := cfg.Get(section, key)
	if !ok {
		return errIniNotFound
	}

	i64, err := strconv.ParseInt(v, 10, 64)
	if err == nil {
		*p = int(i64)
	}
	return err
}

func iniUint64(cfg ini.File, p *uint64, section, key string) error {
	v, ok := cfg.Get(section, key)
	if !ok {
		return errIniNotFound
	}

	u64, err := strconv.ParseUint(v, 10, 64)
	if err == nil {
		*p = u64
	}
	return err
}

func iniDuration(cfg ini.File, p *time.Duration, section, key string) error {
	v, ok := cfg.Get(section, key)
	if !ok {
		return errIniNotFound
	}

	dur, err := strduration.ParseDuration(v)
	if err == nil {
		*p = dur
	}
	return err
}

func iniMaxMsgSize(cfg ini.File, p *rpc.MaxMsgSizeVersion, section, key string) error {
	v, ok := cfg.Get(section, key)
	if !ok {
		return errIniNotFound
	}

	u64, err := strconv.ParseUint(v, 10, 64)
	if err == nil {
		*p = rpc.MaxMsgSizeVersion(u64)
	}
	return err
}
