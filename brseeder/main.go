package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/server"
	"github.com/decred/slog"
	"github.com/jrick/logrotate/rotator"
	"golang.org/x/sync/errgroup"
)

const (
	appName            = "brseeder"
	defaultHTTPTimeout = 20 * time.Second
)

type ServerConfig struct {
	PublicKey string `json:"publicKey"`
	IsMaster  bool   `json:"isMaster"`
}

type ServerAPI struct {
	ServerConfigs []ServerConfig `json:"serverConfigs"`
}

func main() {
	ctx, cancel := shutdownListener()
	defer cancel()

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "loadConfig: %v\n", err)
		os.Exit(1)
	}

	var listenCfg net.ListenConfig
	listener, err := listenCfg.Listen(ctx, "tcp", cfg.Listen)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to listen on %v: %v", cfg.Listen, err)
		os.Exit(1)
	}

	logDir := filepath.Join(defaultHomeDir, "logs")
	if err = os.Mkdir(logDir, 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create %v: %v", logDir, err)
		os.Exit(1)
	}
	logPath := filepath.Join(logDir, "brseeder.log")
	logFd, err := rotator.New(logPath, 32*1024, true, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup logfile %s: %v", logPath, err)
		os.Exit(1)
	}
	defer logFd.Close()

	bknd := slog.NewBackend(&logWriter{logFd}, slog.WithFlags(slog.LUTC))
	logger := bknd.Logger("BRSEED")

	var serverMtx sync.Mutex
	serverMap := make(map[string]server.APIStatus)

	// create the http response for brservers.
	createServerReply := func() ServerAPI {
		serverMtx.Lock()
		defer serverMtx.Unlock()

		// find current master
		var currentMaster, newMaster *server.APIStatus
		for _, apiStatus := range serverMap {
			if apiStatus.Database.Master {
				currentMaster = &apiStatus
				break
			}
		}
		switch {
		case currentMaster == nil:
			logger.Warn("no current master")
			// no current master - pick one.
			for _, apiStatus := range serverMap {
				// Whichever server can write will be Online.
				if apiStatus.Database.Online {
					newMaster = &apiStatus
					break
				}
			}
		case currentMaster.Database.Online:
			logger.Info("current master is healthy")
			// current master is still healthy.
			newMaster = currentMaster

		default:
			logger.Warn("current master offline")
			// pick a new master
			var brserver string
			for server, apiStatus := range serverMap {
				// skip current master, it is unhealthy.
				if currentMaster == &apiStatus {
					continue
				}
				if apiStatus.Database.Online {
					newMaster = &apiStatus
					brserver = server
					break
				}
			}
			if newMaster == nil {
				logger.Warn("no master chosen")
			} else {
				logger.Infof("new master chosen: %v", brserver)
			}
		}
		var serverAPI ServerAPI
		for _, apiStatus := range serverMap {
			serverAPI.ServerConfigs = append(serverAPI.ServerConfigs, ServerConfig{
				PublicKey: apiStatus.Node.PublicKey,
				IsMaster:  newMaster == &apiStatus,
			})
		}
		return serverAPI
	}

	// create the http response for brclients.
	createClientReply := func() clientintf.ClientAPI {
		serverMtx.Lock()
		defer serverMtx.Unlock()

		var clientAPI clientintf.ClientAPI
		for brserver, apiStatus := range serverMap {
			host, _, _ := net.SplitHostPort(brserver)
			clientAPI.ServerGroups = append(clientAPI.ServerGroups, clientintf.ServerGroup{
				Server:   fmt.Sprintf("%s:443", host),
				LND:      fmt.Sprintf("%s@%s:9735", apiStatus.Node.PublicKey, host),
				IsMaster: apiStatus.Database.Master && apiStatus.Database.Online, // XXX
			})
		}
		return clientAPI
	}

	// launch routine to monitor servers.
	go func() {
		const tickerTime = 20 * time.Second
		httpClient := http.Client{
			Timeout: tickerTime,
		}
		t := time.NewTicker(tickerTime)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				var eg errgroup.Group
				for i := range cfg.BRs {
					brserver := cfg.BRs[i]
					eg.Go(func() error {
						apiurl := fmt.Sprintf("http://%v/api/live", brserver)
						req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiurl, nil)
						if err != nil {
							logger.Errorf("%v: %v", brserver, err)
							return err
						}
						rep, err := httpClient.Do(req)
						if err != nil {
							logger.Errorf("%v: %v", brserver, err)
							return err
						}
						if rep.StatusCode != 200 {
							logger.Errorf("%v: %v", rep.Status)
							return err
						}
						body, err := io.ReadAll(rep.Body)
						rep.Body.Close()
						if err != nil {
							logger.Errorf("%v: %v", brserver, err)
							return err
						}
						logger.Errorf("%s", string(body))
						var apiStatus server.APIStatus
						if err = json.Unmarshal(body, &apiStatus); err != nil {
							logger.Errorf("%v: %v", brserver, err)
							return err
						}

						serverMtx.Lock()
						serverMap[brserver] = apiStatus
						serverMtx.Unlock()

						return nil
					})
				}
				err := eg.Wait()
				if err != nil {
					logger.Errorf("%v", err)
				}
			}
		}
	}()

	mux := http.NewServeMux()

	// serve api for brservers
	mux.HandleFunc("/api/servers", func(w http.ResponseWriter, r *http.Request) {
		flush, ok := w.(http.Flusher)
		if !ok {
			http.NotFound(w, r)
			return
		}

		//w.Header().Set("Content-Type", "text/plain; charset=utf-8") // not a json array
		w.Header().Set("Content-Type", "application/json")
		// Replace the Server response header. When used with nginx's "server_tokens
		// off;" and "proxy_pass_header Server;" options.
		w.Header().Set("Server", appName)
		w.WriteHeader(http.StatusOK)
		flush.Flush()

		reply := createServerReply()
		json.NewEncoder(w).Encode(reply)
	})

	// serve api for brclients
	mux.HandleFunc("/api/live", func(w http.ResponseWriter, r *http.Request) {
		flush, ok := w.(http.Flusher)
		if !ok {
			http.NotFound(w, r)
			return
		}

		//w.Header().Set("Content-Type", "text/plain; charset=utf-8") // not a json array
		w.Header().Set("Content-Type", "application/json")
		// Replace the Server response header. When used with nginx's "server_tokens
		// off;" and "proxy_pass_header Server;" options.
		w.Header().Set("Server", appName)
		w.WriteHeader(http.StatusOK)
		flush.Flush()

		reply := createClientReply()
		json.NewEncoder(w).Encode(reply)
	})

	srv := &http.Server{
		Handler:      mux,
		ReadTimeout:  defaultHTTPTimeout, // slow requests should not hold connections opened
		WriteTimeout: defaultHTTPTimeout, // request to response time
	}

	go func() {
		logger.Infof("Listening on %s", listener.Addr())
		err = srv.Serve(listener)
		// ErrServerClosed is expected from a graceful server shutdown, it can
		// be ignored. Anything else should be logged.
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Errorf("unexpected (http.Server).Serve error: %v", err)
		}
	}()

	<-ctx.Done()
	srv.Close()
}

type logWriter struct {
	r *rotator.Rotator
}

func (l *logWriter) Write(p []byte) (n int, err error) {
	os.Stdout.Write(p)
	return l.r.Write(p)
}
