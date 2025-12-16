package main

import (
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"

	"github.com/companyzero/bisonrelay/internal/version"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/server/settings"
)

func ObtainSettings() (*settings.Settings, error) {
	// defaults
	s := settings.New()

	// setup default paths
	usr, err := user.Current()
	if err != nil {
		return nil, err
	}

	// config file
	filename := flag.String("cfg", filepath.Join(usr.HomeDir, ".brserver", "brserver.conf"),
		"config file")
	versionFlag := flag.Bool("version", false, "show version")
	flag.Parse()

	if *versionFlag {
		fmt.Fprintf(os.Stderr, "brserver %s (%s) protocol version %d\n",
			version.String(), runtime.Version(), rpc.ProtocolVersion)
		os.Exit(0)
	}

	// load file
	err = s.Load(*filename)
	if err != nil {
		return nil, err
	}

	return s, nil
}
