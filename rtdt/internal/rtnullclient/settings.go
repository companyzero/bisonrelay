package main

import (
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"

	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/vaughan0/go-ini"
)

const maxLogFiles = 100

type settings struct {
	Listen        string // listen address
	ServerAddr    string
	ServerPubKey  string
	CookieKey     *zkidentity.FixedSizeSymmetricKey
	ReadRoutines  int
	BasePeerID    int
	EnableE2E     bool
	EnableE2EAuth bool

	// log section
	LogFile    string // log filename
	DebugLevel string // debug level config string

	// Debug section
	Profile string // Profiler bind addr
}

func version() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "(unknown version)"
	}
	var vcs, revision string
	for _, bs := range bi.Settings {
		switch bs.Key {
		case "vcs":
			vcs = bs.Value
		case "vcs.revision":
			revision = bs.Value
		}
	}
	if vcs == "" {
		return "(unknown version)"
	}
	if vcs == "git" && len(revision) > 9 {
		revision = revision[:9]
	}
	return revision
}

func obtainSettings() (*settings, error) {
	// setup default paths
	usr, err := user.Current()
	if err != nil {
		return nil, err
	}

	// config file
	rootDir := filepath.Join(usr.HomeDir, ".rtnullclient")
	filename := flag.String("cfg", filepath.Join(rootDir, "rtnullclient.conf"), "config file")
	versionFlag := flag.Bool("version", false, "show version")
	flag.Parse()

	if *versionFlag {
		fmt.Fprintf(os.Stderr, "rtnullclient %s (%s) protocol version %d\n",
			version(), runtime.Version(), rpc.ProtocolVersion)
		os.Exit(0)
	}

	// parse file
	cfg, err := ini.LoadFile(*filename)
	if err != nil {
		return nil, err
	}

	get := func(s *string, section, field string) {
		v, ok := cfg.Get(section, field)
		if ok {
			*s = v
		}
	}
	getInt := func(i *int, section, field string) {
		s, ok := cfg.Get(section, field)
		if ok {
			v, err := strconv.Atoi(s)
			if err == nil {
				*i = v
			}
		}
	}
	getBool := func(b *bool, section, field string) {
		s, ok := cfg.Get(section, field)
		if ok {
			v, err := strconv.ParseBool(s)
			if err == nil {
				*b = v
			}
		}
	}

	// Default settings.
	s := &settings{
		Listen:       "127.0.0.1:9400",
		ServerAddr:   "127.0.0.1:7943",
		LogFile:      filepath.Join(rootDir, "logs", "brrtdtserver.log"),
		DebugLevel:   "info",
		ReadRoutines: 1,
		BasePeerID:   rand.IntN(1 << 16),
	}

	// Fill settings.
	get(&s.ServerAddr, "", "serveraddr")
	get(&s.ServerPubKey, "", "serverpubkey")
	get(&s.Listen, "", "listen")
	getInt(&s.ReadRoutines, "", "readroutines")
	getInt(&s.BasePeerID, "", "basepeerid")
	getBool(&s.EnableE2E, "", "enablee2e")
	getBool(&s.EnableE2EAuth, "", "enablee2eauth")
	get(&s.LogFile, "log", "logfile")
	get(&s.DebugLevel, "log", "debuglevel")
	get(&s.Profile, "debug", "profile")

	var cookieKey string
	get(&cookieKey, "", "cookiekey")
	if cookieKey != "" {
		var ck zkidentity.FixedSizeSymmetricKey
		err := ck.FromString(cookieKey)
		if err != nil {
			return s, fmt.Errorf("invalid cookie key: %v", err)
		}
		s.CookieKey = &ck
	}

	return s, nil
}
