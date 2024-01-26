package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/companyzero/bisonrelay/brclient/internal/version"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/go-socks/socks"
	"github.com/jrick/flagfile"
	strduration "github.com/xhit/go-str2duration/v2"
	"golang.org/x/exp/slices"
)

const (
	appName = "brclient"
)

var (
	// defaultAutoRemoveIgnoreList is the list of users that should not be
	// removed during the auto unsubscribe idle check. By default, these are
	// some well-known bots.
	defaultAutoRemoveIgnoreList = strings.Join([]string{
		"86abd31f2141b274196d481edd061a00ab7a56b61a31656775c8a590d612b966", // Oprah
		"ad716557157c1f191d8b5f8c6757ea41af49de27dc619fc87f337ca85be325ee", // GC bot
	}, ",")
)

var (
	// Error to signal loadConfig() completed everything the cmd had to do
	// and main() should exit.
	errCmdDone = errors.New("cmd done")
)

type errConfigDoesNotExist struct {
	configPath string
}

func (err errConfigDoesNotExist) Error() string {
	return fmt.Sprintf("config file %q does not exist", err.configPath)
}

type cfgStringArray []string

func (c *cfgStringArray) String() string {
	return strings.Join(*c, " ")
}
func (c *cfgStringArray) Set(value string) error {
	*c = append(*c, value)
	return nil
}

type simpleStorePayType string

const (
	ssPayTypeNone    simpleStorePayType = ""
	ssPayTypeLN      simpleStorePayType = "ln"
	ssPayTypeOnChain simpleStorePayType = "onchain"
)

func (sspt simpleStorePayType) isValid() bool {
	return (sspt == ssPayTypeNone) || (sspt == ssPayTypeLN) ||
		(sspt == ssPayTypeOnChain)
}

type config struct {
	ServerAddr        string
	Root              string
	DBRoot            string
	MsgRoot           string
	DownloadsRoot     string
	LNRPCHost         string
	LNTLSCertPath     string
	LNMacaroonPath    string
	LNDebugLevel      string
	LNMaxLogFiles     int
	LNRPCListen       []string
	LogFile           string
	MaxLogFiles       int
	DebugLevel        string
	WalletType        string
	CompressLevel     int
	CmdHistoryPath    string
	NickColor         string
	GCOtherColor      string
	PMOtherColor      string
	BlinkCursor       bool
	BellCmd           string
	Network           string
	CPUProfile        string
	CPUProfileHz      int
	MemProfile        string
	LogPings          bool
	NoLoadChatHistory bool
	SendRecvReceipts  bool

	AutoHandshakeInterval       time.Duration
	AutoRemoveIdleUsersInterval time.Duration
	AutoRemoveIdleUsersIgnore   []string

	SyncFreeList bool

	ProxyAddr    string
	ProxyUser    string
	ProxyPass    string
	TorIsolation bool

	MinWalletBal dcrutil.Amount
	MinRecvBal   dcrutil.Amount
	MinSendBal   dcrutil.Amount

	WinPin             []string
	MimeMap            map[string]string
	InviteFundsAccount string

	JSONRPCListen      []string
	RPCCertPath        string
	RPCKeyPath         string
	RPCClientCAPath    string
	RPCIssueClientCert bool

	ExternalEditorForComments bool

	ResourcesUpstream     string
	SimpleStorePayType    simpleStorePayType
	SimpleStoreAccount    string
	SimpleStoreShipCharge float64

	dialFunc func(context.Context, string, string) (net.Conn, error)
}

func defaultAppDataDir(homeDir string) string {
	switch runtime.GOOS {
	// Attempt to use the LOCALAPPDATA or APPDATA environment variable on
	// Windows.
	case "windows":
		// Windows XP and before didn't have a LOCALAPPDATA, so fallback
		// to regular APPDATA when LOCALAPPDATA is not set.
		appData := os.Getenv("LOCALAPPDATA")
		if appData == "" {
			appData = os.Getenv("APPDATA")
		}

		if appData != "" {
			return filepath.Join(appData, appName)
		}

	case "darwin":
		if homeDir != "" {
			return filepath.Join(homeDir, "Library",
				"Application Support", appName)
		}

	case "plan9":
		if homeDir != "" {
			return filepath.Join(homeDir, appName)
		}

	default:
		if homeDir != "" {
			return filepath.Join(homeDir, "."+appName)
		}
	}

	return filepath.Join(".", appName)
}

func expandPath(homeDir, path string) string {
	if len(path) > 0 && path[0] == '~' {
		path = filepath.Join(homeDir, path[1:])
	}

	return path
}

// defaultRootDir returns the default root dir for data for the given
// cfgFilePath.
func defaultRootDir(cfgFilePath string) string {
	return filepath.Dir(cfgFilePath)
}

// defaultLNWalletDir returns the default dir for an internal LNWallet data,
// given the root data dir.
func defaultLNWalletDir(rootDir string) string {
	return filepath.Join(rootDir, "ln-wallet")
}

func loadConfig() (*config, error) {
	const (
		rpcCertFileName     = "rpc.cert"
		rpcKeyFileName      = "rpc.key"
		rpcClientCAFileName = "rpc-ca.cert"
	)

	// Setup defaults.
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	defaultAppDir := defaultAppDataDir(homeDir)
	defaultCfgFile := filepath.Join(defaultAppDir, appName+".conf")
	defaultLogFile := filepath.Join(defaultAppDir, "applogs", appName+".log")
	defaultMsgRoot := filepath.Join(defaultAppDir, "logs")
	defaultDebugLevel := "info"
	defaultCompressLevel := 4
	defaultWalletType := "disabled"
	defaultRPCCertPath := filepath.Join(defaultAppDir, rpcCertFileName)
	defaultRPCKeyPath := filepath.Join(defaultAppDir, rpcKeyFileName)
	defaultRPCClientCA := filepath.Join(defaultAppDir, rpcClientCAFileName)

	// Parse CLI arguments.
	fs := flag.NewFlagSet("CLI Arguments", flag.ContinueOnError)
	flagVersion := fs.Bool("version", false, "Display current version and exit")
	flagCfgFile := fs.String("cfg", defaultCfgFile, "Config file to load")
	flagProfile := fs.String("profile", "", "ip:port of where to run the go profiler")
	flagCPUProfile := fs.String("cpuprofile", "", "filename to dump CPU profiling")
	flagCPUProfileHz := fs.Int("cpuprofilehz", 0, "Frequency to sample cpu profiling")
	flagMemProfile := fs.String("memprofile", "", "filename to dump mem profiling")
	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, errCmdDone
		}
		return nil, err
	}

	if *flagProfile != "" {
		go http.ListenAndServe(*flagProfile, nil)
	}

	if *flagVersion {
		fmt.Println("Version: " + version.String())
		return nil, errCmdDone
	}

	// Make sure cfgFile is not empty.
	cfgFile := *flagCfgFile
	if cfgFile == "" {
		cfgFile = defaultCfgFile
	}
	cfgFile = expandPath(homeDir, cfgFile)

	// Open config file.
	f, err := os.Open(cfgFile)
	if os.IsNotExist(err) {
		// Config file does not exist. Make UI go into initial config
		// wizard.
		return nil, errConfigDoesNotExist{configPath: cfgFile}
	} else if err != nil {
		return nil, err
	}
	defer f.Close()

	// Define config file flags.
	fs = flag.NewFlagSet("Config Options", flag.ContinueOnError)
	flagServerAddr := fs.String("server", "127.0.0.1:12345", "Address and port of the CR server")
	flagRootDir := fs.String("root", defaultAppDir, "Root of all app data")
	flagWinPin := fs.String("winpin", "", "Comma delimited list of DM and GC windows to launch on start")
	flagSendRecvReceipts := fs.Bool("sendrecvreceipts", true, "Send receive receipts")
	flagCompressLevel := fs.Int("compresslevel", defaultCompressLevel, "Compression level")
	flagProxyAddr := fs.String("proxyaddr", "", "")
	flagProxyUser := fs.String("proxyuser", "", "")
	flagProxyPass := fs.String("proxypass", "", "")
	flagTorIsolation := fs.Bool("torisolation", false, "")
	flagCircuitLimit := fs.Uint("circuitlimit", 32, "max number of open connections per proxy connection")
	var mimetypes cfgStringArray
	fs.Var(&mimetypes, "mimetype", "List of mimetypes with viewer")

	flagBellCmd := fs.String("bellcmd", "", "Bell command on new msgs")
	flagSyncFreeList := fs.Bool("syncfreelist", true, "")

	flagExternalEditorForComments := fs.Bool("externaleditorforcomments", false, "")
	flagNoLoadChatHistory := fs.Bool("noloadchathistory", false, "Whether to read chat logs to build chat history")

	flagAutoHandshake := fs.String("autohandshakeinterval", "21d", "")
	flagAutoRemove := fs.String("autoremoveidleusersinterval", "60d", "")
	flagAutoRemoveIgnoreList := fs.String("autoremoveignorelist", defaultAutoRemoveIgnoreList, "")

	// log
	flagMsgRoot := fs.String("log.msglog", defaultMsgRoot, "Root for message log files")
	flagLogFile := fs.String("log.logfile", defaultLogFile, "Log file location")
	flagMaxLogFiles := fs.Int("log.maxlogfiles", 0, "Max log files")
	flagDebugLevel := fs.String("log.debuglevel", defaultDebugLevel, "Debug Level")
	flagSaveHistory := fs.Bool("log.savehistory", false, "Whether to save history to a file")
	flagLogPings := fs.Bool("log.pings", false, "Whether to log pings")

	// theme
	flagNickColor := fs.String("theme.nickcolor", "bold:white:na", "color of the nick")
	flagGCOtherColor := fs.String("theme.gcothercolor", "bold:green:na", "color of other nicks in gc")
	flagPMOtherColor := fs.String("theme.pmothercolor", "bold:cyan:na", "color of other nicks in pms")
	flagBlinkCursor := fs.Bool("theme.blinkcursor", true, "Blink cursor")

	// payment
	flagWalletType := fs.String("payment.wallettype", defaultWalletType, "Wallet type to use")
	flagNetwork := fs.String("payment.network", "mainnet", "Network to connect")
	flagLNHost := fs.String("payment.lnrpchost", "127.0.0.1:10009", "dcrlnd network address")
	flagLNTLSCert := fs.String("payment.lntlscert", "~/.dcrlnd/tls.cert", "path to dcrlnd tls.cert")
	flagLNMacaroonPath := fs.String("payment.lnmacaroonpath", "", "path do dcrlnd admin.macaroon")
	flagLNDebugLevel := fs.String("payment.lndebuglevel", "info", "LN log level")
	flagLNMaxLogFiles := fs.Int("payment.lnmaxlogfiles", 3, "LN Max Log Files")
	flagMinWalletBal := fs.Float64("payment.minimumwalletbalance", 1.0, "Minimum wallet balance before warn")
	flagMinRecvBal := fs.Float64("payment.minimumrecvbalance", 0.01, "Minimum receive balance before warn")
	flagMinSendBal := fs.Float64("payment.minimumsendbalance", 0.01, "Minimum send balance before warn")
	flagLNRPCListen := fs.String("payment.lnrpclisten", "", "list of addrs for the embedded ln to listen on")
	flagInviteFundsAccount := fs.String("payment.invitefundsaccount", "", "")

	// clientrpc
	flagJSONRPCListen := fs.String("clientrpc.jsonrpclisten", "", "Comma delimited list of JSON-RPC server binding addresses")
	flagRPCCertPath := fs.String("clientrpc.rpccertpath", defaultRPCCertPath, "")
	flagRPCKeyPath := fs.String("clientrpc.rpckeypath", defaultRPCKeyPath, "")
	flagRPCClientCAPath := fs.String("clientrpc.rpcclientcapath", defaultRPCClientCA, "")
	flagRPCIssueClientCert := fs.Bool("clientrpc.rpcissueclientcert", true, "")

	// resources
	flagResourcesUpstream := fs.String("resources.upstream", "", "Upstream processor of resource requests")

	// simplestore
	flagSimpleStorePayType := fs.String("simplestore.paytype", "", "How to charge for paystore purchases")
	flagSimpleStoreAccount := fs.String("simplestore.account", "", "Account to use for on-chain adresses")
	flagSimpleStoreShipCharge := fs.Float64("simplestore.shipcharge", 0, "How much to charge for s&h")

	// Load config from file.
	parser := flagfile.Parser{
		ParseSections: true,
	}
	if err := parser.Parse(f, fs); err != nil {
		return nil, err
	}

	// Sanity check loaded flags.
	if *flagServerAddr == "" {
		return nil, fmt.Errorf("flag 'server' cannot be empty")
	}
	if *flagRootDir == "" {
		return nil, fmt.Errorf("flag 'root' cannot be empty")
	}
	autoHandshakeInterval, err := strduration.ParseDuration(*flagAutoHandshake)
	if err != nil {
		return nil, fmt.Errorf("invalid value for flag 'autohandshakeinterval': %v", err)
	}
	autoRemoveInterval, err := strduration.ParseDuration(*flagAutoRemove)
	if err != nil {
		return nil, fmt.Errorf("invalid value for flag 'autoremoveidleusersinterval': %v", err)
	}

	// Clean paths.
	*flagRootDir = expandPath(homeDir, *flagRootDir)
	switch {
	case *flagResourcesUpstream == "",
		*flagResourcesUpstream == "clientrpc",
		strings.HasPrefix(*flagResourcesUpstream, "http://"),
		strings.HasPrefix(*flagResourcesUpstream, "https://"):
		// Valid, and no more processing needed.
	case strings.HasPrefix(*flagResourcesUpstream, "pages:"):
		path := (*flagResourcesUpstream)[len("pages:"):]
		path = expandPath(homeDir, path)
		*flagResourcesUpstream = "pages:" + path
	case strings.HasPrefix(*flagResourcesUpstream, "simplestore:"):
		path := (*flagResourcesUpstream)[len("simplestore:"):]
		path = expandPath(homeDir, path)
		*flagResourcesUpstream = "simplestore:" + path
	default:
		return nil, fmt.Errorf("unknown resources upstream provider %q", *flagResourcesUpstream)

	}

	// Reconfigure dirs that are based on the root dir when they are not
	// specified.
	if *flagRPCCertPath == defaultRPCCertPath {
		*flagRPCCertPath = filepath.Join(*flagRootDir, rpcCertFileName)
		*flagRPCKeyPath = filepath.Join(*flagRootDir, rpcKeyFileName)
		*flagRPCClientCAPath = filepath.Join(*flagRootDir, rpcClientCAFileName)
	}

	// Clean paths.
	*flagLogFile = expandPath(homeDir, *flagLogFile)
	*flagLNTLSCert = expandPath(homeDir, *flagLNTLSCert)
	*flagLNMacaroonPath = expandPath(homeDir, *flagLNMacaroonPath)
	*flagMsgRoot = expandPath(homeDir, *flagMsgRoot)
	*flagRPCKeyPath = expandPath(homeDir, *flagRPCKeyPath)
	*flagRPCCertPath = expandPath(homeDir, *flagRPCCertPath)
	*flagRPCClientCAPath = expandPath(homeDir, *flagRPCClientCAPath)

	var cmdHistoryPath string
	if *flagSaveHistory {
		cmdHistoryPath = filepath.Join(*flagRootDir, "history")
	}

	minWalletBal, err := dcrutil.NewAmount(*flagMinWalletBal)
	if err != nil || minWalletBal < 0 {
		return nil, fmt.Errorf("invalid minimum wallet balance")
	}
	minRecvBal, err := dcrutil.NewAmount(*flagMinRecvBal)
	if err != nil || minRecvBal < 0 {
		return nil, fmt.Errorf("invalid minimum receive balance")
	}
	minSendBal, err := dcrutil.NewAmount(*flagMinSendBal)
	if err != nil || minSendBal < 0 {
		return nil, fmt.Errorf("invalid minimum send balance")
	}
	var winpin []string
	if *flagWinPin != "" {
		winpin = strings.Split(*flagWinPin, ",")
	}
	mimeMap := make(map[string]string)
	for _, mimetype := range mimetypes {
		spl := strings.Split(mimetype, ",")
		if len(spl) != 2 {
			return nil, fmt.Errorf("invalid mimetype line: %v", mimetype)
		}
		mimeMap[spl[0]] = spl[1]
	}

	autoRemoveIgnoreList := strings.Split(*flagAutoRemoveIgnoreList, ",")
	for i := range autoRemoveIgnoreList {
		autoRemoveIgnoreList[i] = strings.TrimSpace(autoRemoveIgnoreList[i])
	}

	var jrpcListen []string
	if *flagJSONRPCListen != "" {
		jrpcListen = strings.Split(*flagJSONRPCListen, ",")
	}

	var lnRPCListen []string
	if *flagLNRPCListen != "" {
		lnRPCListen = strings.Split(*flagLNRPCListen, ",")
		for i := 0; i < len(lnRPCListen); i++ {
			v := strings.TrimSpace(lnRPCListen[i])
			if v == "" {
				lnRPCListen = slices.Delete(lnRPCListen, i, i)
			} else {
				lnRPCListen[i] = v
				i += 1
			}
		}
	}

	ssPayType := simpleStorePayType(*flagSimpleStorePayType)
	if !ssPayType.isValid() {
		return nil, fmt.Errorf("invalid simple store payment type %q",
			ssPayType)
	}

	var d net.Dialer
	dialFunc := d.DialContext
	if *flagProxyAddr != "" {
		proxy := socks.Proxy{
			Addr:         *flagProxyAddr,
			Username:     *flagProxyUser,
			Password:     *flagProxyPass,
			TorIsolation: *flagTorIsolation,
		}
		var proxyDialer clientintf.DialFunc
		if *flagTorIsolation {
			proxyDialer = socks.NewPool(proxy, uint32(*flagCircuitLimit)).DialContext
		} else {
			proxyDialer = proxy.DialContext
		}
		dialFunc = proxyDialer
	}

	// Return the final cfg object.
	return &config{
		ServerAddr:         *flagServerAddr,
		Root:               *flagRootDir,
		DBRoot:             filepath.Join(*flagRootDir, "db"),
		DownloadsRoot:      filepath.Join(*flagRootDir, "downloads"),
		WalletType:         *flagWalletType,
		MsgRoot:            *flagMsgRoot,
		LNRPCHost:          *flagLNHost,
		LNTLSCertPath:      *flagLNTLSCert,
		LNMacaroonPath:     *flagLNMacaroonPath,
		LNDebugLevel:       *flagLNDebugLevel,
		LNMaxLogFiles:      *flagLNMaxLogFiles,
		LNRPCListen:        lnRPCListen,
		LogFile:            *flagLogFile,
		MaxLogFiles:        *flagMaxLogFiles,
		DebugLevel:         *flagDebugLevel,
		CompressLevel:      *flagCompressLevel,
		CmdHistoryPath:     cmdHistoryPath,
		NickColor:          *flagNickColor,
		GCOtherColor:       *flagGCOtherColor,
		PMOtherColor:       *flagPMOtherColor,
		BlinkCursor:        *flagBlinkCursor,
		BellCmd:            strings.TrimSpace(*flagBellCmd),
		Network:            *flagNetwork,
		CPUProfile:         *flagCPUProfile,
		CPUProfileHz:       *flagCPUProfileHz,
		MemProfile:         *flagMemProfile,
		LogPings:           *flagLogPings,
		SendRecvReceipts:   *flagSendRecvReceipts,
		NoLoadChatHistory:  *flagNoLoadChatHistory,
		ProxyAddr:          *flagProxyAddr,
		ProxyUser:          *flagProxyUser,
		ProxyPass:          *flagProxyPass,
		TorIsolation:       *flagTorIsolation,
		MinWalletBal:       minWalletBal,
		MinRecvBal:         minRecvBal,
		MinSendBal:         minSendBal,
		WinPin:             winpin,
		MimeMap:            mimeMap,
		JSONRPCListen:      jrpcListen,
		RPCCertPath:        *flagRPCCertPath,
		RPCKeyPath:         *flagRPCKeyPath,
		RPCClientCAPath:    *flagRPCClientCAPath,
		RPCIssueClientCert: *flagRPCIssueClientCert,
		InviteFundsAccount: *flagInviteFundsAccount,
		ResourcesUpstream:  *flagResourcesUpstream,

		AutoHandshakeInterval:       autoHandshakeInterval,
		AutoRemoveIdleUsersInterval: autoRemoveInterval,
		AutoRemoveIdleUsersIgnore:   autoRemoveIgnoreList,

		SyncFreeList:              *flagSyncFreeList,
		ExternalEditorForComments: *flagExternalEditorForComments,

		SimpleStorePayType:    ssPayType,
		SimpleStoreAccount:    *flagSimpleStoreAccount,
		SimpleStoreShipCharge: *flagSimpleStoreShipCharge,

		dialFunc: dialFunc,
	}, nil
}

func saveNewConfig(cfgFile string, cfg *config) error {
	// Figure out the config file name (which also establishes the data
	// root).
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	defaultAppDir := defaultAppDataDir(homeDir)
	defaultCfgFile := filepath.Join(defaultAppDir, appName+".conf")

	if cfgFile == "" {
		cfgFile = defaultCfgFile
	}

	// Override the dirs when saving a new config. We also replace the home
	// dir prefix by "~" in the saved config.
	cfg.Root = defaultRootDir(cfgFile)
	if cfg.Root[0] != '~' && strings.HasPrefix(cfg.Root, homeDir) {
		cfg.Root = "~" + cfg.Root[len(homeDir):]
	}
	cfg.DBRoot = filepath.Join(cfg.Root, "db")
	cfg.MsgRoot = filepath.Join(cfg.Root, "logs")
	cfg.LogFile = filepath.Join(cfg.Root, "applogs", appName+".log")

	tmpl, err := template.New("configfile").Parse(defaultConfigFileContent)
	if err != nil {
		return err
	}

	var generated bytes.Buffer
	if err := tmpl.Execute(&generated, cfg); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(cfgFile), 0o700); err != nil {
		return fmt.Errorf("unable to create data dir: %v", err)
	}

	return os.WriteFile(cfgFile, generated.Bytes(), 0o600)
}

// cleanAndExpandPath expands environment variables and leading ~ in the
// passed path, cleans the result, and returns it.
func cleanAndExpandPath(path string) string {
	// Nothing to do when no path is given.
	if path == "" {
		return path
	}

	// NOTE: The os.ExpandEnv doesn't work with Windows cmd.exe-style
	// %VARIABLE%, but the variables can still be expanded via POSIX-style
	// $VARIABLE.
	path = os.ExpandEnv(path)

	if !strings.HasPrefix(path, "~") {
		return filepath.Clean(path)
	}

	// Expand initial ~ to the current user's home directory, or ~otheruser
	// to otheruser's home directory.  On Windows, both forward and backward
	// slashes can be used.
	path = path[1:]

	var pathSeparators string
	if runtime.GOOS == "windows" {
		pathSeparators = string(os.PathSeparator) + "/"
	} else {
		pathSeparators = string(os.PathSeparator)
	}

	userName := ""
	if i := strings.IndexAny(path, pathSeparators); i != -1 {
		userName = path[:i]
		path = path[i:]
	}

	homeDir := ""
	var u *user.User
	var err error
	if userName == "" {
		u, err = user.Current()
	} else {
		u, err = user.Lookup(userName)
	}
	if err == nil {
		homeDir = u.HomeDir
	}
	// Fallback to CWD if user lookup fails or user has no home directory.
	if homeDir == "" {
		homeDir = "."
	}

	return filepath.Join(homeDir, path)
}
