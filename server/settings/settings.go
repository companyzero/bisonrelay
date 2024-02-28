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

	// policy section
	ExpirationDays    int // How many days after which to expire data
	MaxMsgSizeVersion rpc.MaxMsgSizeVersion
	PingLimit         time.Duration

	// payment section
	PayScheme           string
	LNRPCHost           string
	LNTLSCert           string
	LNMacaroonPath      string
	MilliAtomsPerByte   uint64
	MilliAtomsPerSub    uint64
	PushPaymentLifetime int // how long a payment to a push is valid
	MaxPushInvoices     int

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

	// Versioner is a function that returns the current app version.
	Versioner func() string

	// LogStdOut is the stdout to write the log to. Defaults to os.Stdout.
	LogStdOut io.Writer
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
		Listen:          []string{"127.0.0.1:12345"},
		InitSessTimeout: time.Second * 20,

		// Policy
		ExpirationDays:    rpc.PropExpirationDaysDefault,
		MaxMsgSizeVersion: rpc.PropMaxMsgSizeVersionDefault,
		PingLimit:         rpc.PropPingLimitDefault,

		// payment
		PayScheme:           "free",
		MilliAtomsPerByte:   rpc.PropPushPaymentRateDefault,
		MilliAtomsPerSub:    rpc.PropSubPaymentRateDefault,
		PushPaymentLifetime: rpc.PropPushPaymentLifetimeDefault,
		MaxPushInvoices:     rpc.PropMaxPushInvoicesDefault,

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

	var atomsPerByte float64 = float64(rpc.PropPushPaymentRateDefault) / 1000
	err = iniFloat(cfg, &atomsPerByte, "payment", "atomsperbyte")
	if err != nil && !errors.Is(err, errIniNotFound) {
		return err
	}
	s.MilliAtomsPerByte = uint64(atomsPerByte * 1000)

	var atomsPerSub float64 = float64(rpc.PropSubPaymentRateDefault) / 1000
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
