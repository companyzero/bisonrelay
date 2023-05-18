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
	"path/filepath"
	"runtime"
	"strings"

	"github.com/companyzero/bisonrelay/brclient/internal/version"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/go-socks/socks"
	"github.com/jrick/flagfile"
	"golang.org/x/exp/slices"
)

const (
	appName = "brclient"
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
	// NOTE: ssPayTypeLN is disabled here because brclient does not
	// wrap long lines that do not contain a wrapping character,
	// which make it impossible for users to pay for such payments.
	return (sspt == ssPayTypeNone) || /*(sspt == ssPayTypeLN) ||*/
		(sspt == ssPayTypeOnChain)
}

type config struct {
	ServerAddr     string
	Root           string
	DBRoot         string
	MsgRoot        string
	DownloadsRoot  string
	LNRPCHost      string
	LNTLSCertPath  string
	LNMacaroonPath string
	LNDebugLevel   string
	LNMaxLogFiles  int
	LNRPCListen    []string
	LogFile        string
	MaxLogFiles    int
	DebugLevel     string
	WalletType     string
	CompressLevel  int
	CmdHistoryPath string
	NickColor      string
	GCOtherColor   string
	PMOtherColor   string
	BlinkCursor    bool
	BellCmd        string
	Network        string
	CPUProfile     string
	CPUProfileHz   int
	MemProfile     string
	LogPings       bool

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

func cleanAndExpandPath(homeDir, path string) string {
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
	cfgFile = cleanAndExpandPath(homeDir, cfgFile)

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
	flagSaveHistory := fs.Bool("savehistory", false, "Whether to save history to a file")
	flagMsgRoot := fs.String("msglog", defaultMsgRoot, "Root for message log files")
	flagLogFile := fs.String("logfile", defaultLogFile, "Log file location")
	flagMaxLogFiles := fs.Int("maxlogfiles", 0, "Max log files")
	flagDebugLevel := fs.String("debuglevel", defaultDebugLevel, "Debug Level")
	flagCompressLevel := fs.Int("compresslevel", defaultCompressLevel, "Compression level")
	flagLNHost := fs.String("lnrpchost", "127.0.0.1:10009", "dcrlnd network address")
	flagLNMacaroonPath := fs.String("lnmacaroonpath", "", "path do dcrlnd admin.macaroon")
	flagLNTLSCert := fs.String("lntlscert", "~/.dcrlnd/tls.cert", "path to dcrlnd tls.cert")
	flagLNDebugLevel := fs.String("lndebuglevel", "info", "LN log level")
	flagLNMaxLogFiles := fs.Int("lnmaxlogfiles", 3, "LN Max Log Files")
	flagLNRPCListen := fs.String("lnrpclisten", "", "list of addrs for the embedded ln to listen on")
	flagNickColor := fs.String("nickcolor", "bold:white:na", "color of the nick")
	flagGCOtherColor := fs.String("gcothercolor", "bold:green:na", "color of other nicks in gc")
	flagPMOtherColor := fs.String("pmothercolor", "bold:cyan:na", "color of other nicks in pms")
	flagWalletType := fs.String("wallettype", defaultWalletType, "Wallet type to use")
	flagNetwork := fs.String("network", "mainnet", "Network to connect")
	flagLogPings := fs.Bool("logpings", false, "Whether to log pings")
	flagMinWalletBal := fs.Float64("minimumwalletbalance", 1.0, "Minimum wallet balance before warn")
	flagMinRecvBal := fs.Float64("minimumrecvbalance", 0.01, "Minimum receive balance before warn")
	flagMinSendBal := fs.Float64("minimumsendbalance", 0.01, "Minimum send balance before warn")
	flagWinPin := fs.String("winpin", "", "Comma delimited list of DM and GC windows to launch on start")
	flagBlinkCursor := fs.Bool("blinkcursor", true, "Blink cursor")
	flagBellCmd := fs.String("bellcmd", "", "Bell command on new msgs")
	flagInviteFundsAccount := fs.String("invitefundsaccount", "", "")
	flagJSONRPCListen := fs.String("jsonrpclisten", "", "Comma delimited list of JSON-RPC server binding addresses")
	flagRPCCertPath := fs.String("rpccertpath", defaultRPCCertPath, "")
	flagRPCKeyPath := fs.String("rpckeypath", defaultRPCKeyPath, "")
	flagRPCClientCAPath := fs.String("rpcclientcapath", defaultRPCClientCA, "")
	flagRPCIssueClientCert := fs.Bool("rpcissueclientcert", true, "")
	flagResourcesUpstream := fs.String("resourcesupstream", "", "Upstream processor of resource requests")
	flagSimpleStorePayType := fs.String("simplestorepaytype", "", "How to charge for paystore purchases")
	flagSimpleStoreAccount := fs.String("simplestoreaccount", "", "Account to use for on-chain adresses")
	flagSimpleStoreShipCharge := fs.Float64("simplestoreshipcharge", 0, "How much to charge for s&h")

	flagProxyAddr := fs.String("proxyaddr", "", "")
	flagProxyUser := fs.String("proxyuser", "", "")
	flagProxyPass := fs.String("proxypass", "", "")
	flagTorIsolation := fs.Bool("torisolation", false, "")
	flagCircuitLimit := fs.Uint("circuitlimit", 32, "max number of open connections per proxy connection")

	var mimetypes cfgStringArray
	fs.Var(&mimetypes, "mimetype", "List of mimetypes with viewer")

	// Load config from file.
	if err := flagfile.Parse(f, fs); err != nil {
		return nil, err
	}

	// Sanity check loaded flags.
	if *flagServerAddr == "" {
		return nil, fmt.Errorf("flag 'server' cannot be empty")
	}
	if *flagRootDir == "" {
		return nil, fmt.Errorf("flag 'root' cannot be empty")
	}

	// Clean paths.
	*flagRootDir = cleanAndExpandPath(homeDir, *flagRootDir)
	switch {
	case *flagResourcesUpstream == "",
		*flagResourcesUpstream == "clientrpc",
		strings.HasPrefix(*flagResourcesUpstream, "http://"),
		strings.HasPrefix(*flagResourcesUpstream, "https://"):
		// Valid, and no more processing needed.
	case strings.HasPrefix(*flagResourcesUpstream, "pages:"):
		path := (*flagResourcesUpstream)[len("pages:"):]
		path = cleanAndExpandPath(homeDir, path)
		*flagResourcesUpstream = "pages:" + path
	case strings.HasPrefix(*flagResourcesUpstream, "simplestore:"):
		path := (*flagResourcesUpstream)[len("simplestore:"):]
		path = cleanAndExpandPath(homeDir, path)
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
	*flagLogFile = cleanAndExpandPath(homeDir, *flagLogFile)
	*flagLNTLSCert = cleanAndExpandPath(homeDir, *flagLNTLSCert)
	*flagLNMacaroonPath = cleanAndExpandPath(homeDir, *flagLNMacaroonPath)
	*flagMsgRoot = cleanAndExpandPath(homeDir, *flagMsgRoot)
	*flagRPCKeyPath = cleanAndExpandPath(homeDir, *flagRPCKeyPath)
	*flagRPCCertPath = cleanAndExpandPath(homeDir, *flagRPCCertPath)
	*flagRPCClientCAPath = cleanAndExpandPath(homeDir, *flagRPCClientCAPath)

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
