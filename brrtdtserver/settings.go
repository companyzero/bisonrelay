package main

import (
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/companyzero/bisonrelay/internal/version"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/companyzero/sntrup4591761"
	"github.com/decred/slog"
	"github.com/vaughan0/go-ini"
)

const maxLogFiles = 100

type settings struct {
	Listen           []string // listen address
	ListenPrometheus string   // listen addr for metrics

	PrivateKeyFile   string
	CookieKey        string
	DecodeCookieKeys []string

	ReadRoutines int

	IgnoreSmallKernelBuffers bool

	// log section
	LogFile       string // log filename
	DebugLevel    string // debug level config string
	Profiler      string // go profiler link
	LogErrors     bool
	StatsInterval time.Duration
}

func obtainSettings() (*settings, error) {
	// setup default paths
	usr, err := user.Current()
	if err != nil {
		return nil, err
	}

	// config file
	rootDir := filepath.Join(usr.HomeDir, ".brrtdtserver")
	filename := flag.String("cfg", filepath.Join(rootDir, "brrtdtserver.conf"), "config file")
	versionFlag := flag.Bool("version", false, "show version")
	showEnvFlag := flag.Bool("showenv", false, "show environment and config information")
	flag.Parse()

	println := func(format string, args ...interface{}) {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
	if *versionFlag || *showEnvFlag {
		println("brrtdtserver %s (%s) protocol version %d",
			version.String(), runtime.Version(), rpc.ProtocolVersion)
	}
	if *versionFlag {
		os.Exit(0)
	}
	if *showEnvFlag {
		println("Username: %s", usr.Username)
		println("Uid: %s", usr.Uid)
		println("Home dir: %s", usr.HomeDir)
		println("Root dir: %s", rootDir)
		println("Config file path: %s", *filename)
	}

	// parse file
	cfg, err := ini.LoadFile(*filename)
	if err != nil {
		return nil, err
	}

	if *showEnvFlag {
		println("Config file successfully loaded!")
	}

	get := func(s *string, section, field string) bool {
		v, ok := cfg.Get(section, field)
		if ok {
			*s = v
		}
		return ok
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
		Listen:        []string{"127.0.0.1:7943"},
		LogFile:       filepath.Join(rootDir, "logs", "brrtdtserver.log"),
		DebugLevel:    "info",
		ReadRoutines:  1,
		StatsInterval: 10 * time.Second,
	}

	// Fill settings.
	get(&s.LogFile, "log", "logfile")
	get(&s.DebugLevel, "log", "debuglevel")
	get(&s.Profiler, "log", "profiler")
	getBool(&s.LogErrors, "log", "logerrors")
	get(&s.ListenPrometheus, "", "listenprometheus")
	get(&s.PrivateKeyFile, "", "privkeyfile")
	get(&s.CookieKey, "", "cookiekey")
	getInt(&s.ReadRoutines, "", "readroutines")
	getBool(&s.IgnoreSmallKernelBuffers, "", "ignoresmallkernelbuffers")

	var decodeKeys string
	get(&decodeKeys, "", "decodecookiekeys")
	for _, key := range strings.Split(decodeKeys, ",") {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		s.DecodeCookieKeys = append(s.DecodeCookieKeys, key)
	}

	rawListen, ok := cfg.Get("", "listen")
	if ok {
		listenList := strings.Split(rawListen, ",")
		for i := range listenList {
			listenList[i] = strings.TrimSpace(listenList[i])
		}
		s.Listen = listenList
	}
	if len(s.Listen) == 0 {
		return nil, errors.New("no listen addresses")
	}

	var statsInterval string
	if get(&statsInterval, "log", "statsinterval") {
		if statsInterval != "" {
			interval, err := time.ParseDuration(statsInterval)
			if err != nil {
				return nil, fmt.Errorf("unable to parse stats interval duration: %v", err)
			}
			s.StatsInterval = interval
		} else {
			// Disabled.
			s.StatsInterval = 0
		}
	}

	if *showEnvFlag {
		println("Private Key Filepath: %q", s.PrivateKeyFile)
		println("Cookie key is set: %v", s.CookieKey != "")
		println("Listening addresses:")
		for i, addr := range s.Listen {
			println("  %d - %q", i, addr)
		}
		os.Exit(0)
	}

	return s, nil
}

func loadPrivateKey(privKeyPath string, log slog.Logger) (*zkidentity.FixedSizeSntrupPrivateKey, error) {
	f, err := os.Open(privKeyPath)
	if errors.Is(err, os.ErrNotExist) {
		log.Infof("Private key file does not exist. Generating at %q",
			privKeyPath)

		// Generate new key.
		pubKey, privKey, err := sntrup4591761.GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}

		// Create dir.
		if err := os.MkdirAll(filepath.Dir(privKeyPath), 0o0700); err != nil {
			return nil, fmt.Errorf("unable to create dir for privKey %s: %v", privKeyPath, err)
		}

		if err := os.WriteFile(privKeyPath, privKey[:], 0o0600); err != nil {
			return nil, err
		}
		pubKeyPath := privKeyPath + ".pub"
		if err := os.WriteFile(pubKeyPath, pubKey[:], 0o0644); err != nil {
			return nil, err
		}

		log.Infof("Generated server encryption key. Public key is at %q",
			pubKeyPath)

		return (*zkidentity.FixedSizeSntrupPrivateKey)(privKey), nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var privKey zkidentity.FixedSizeSntrupPrivateKey
	if _, err := io.ReadFull(f, privKey[:]); err != nil {
		return nil, err
	}

	return &privKey, nil
}
