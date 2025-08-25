package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/server"
	"github.com/decred/slog"
	"github.com/gorilla/websocket"
	"github.com/jrick/logrotate/rotator"
)

const (
	appName            = "brseeder"
	defaultHTTPTimeout = 20 * time.Second
)

// RPCError represents a JSON-RPC error object.
type RPCError struct {
	Code    int64           `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func main() {
	ctx, cancel := shutdownListener()
	defer cancel()

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "loadConfig: %v\n", err)
		os.Exit(1)
	}
	tokens := make(map[string]struct{})
	for i := range cfg.Tokens {
		tokens[cfg.Tokens[i]] = struct{}{}
	}

	var listenCfg net.ListenConfig
	listener, err := listenCfg.Listen(ctx, "tcp", cfg.Listen)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to listen on %v: %v", cfg.Listen, err)
		os.Exit(1)
	}

	logDir := filepath.Join(defaultHomeDir, "logs")
	if err = os.MkdirAll(logDir, 0o700); err != nil {
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
	type smi struct {
		token       string
		dboffline   time.Time
		nodeoffline time.Time
	}
	var serverMaster smi
	serverMap := make(map[string]*server.CommandStatus)

	const waitForMaster = 5 * time.Minute
	timeStarted := time.Now()

	// create the http response for brservers.
	// must be called with serverMtx locked.
	createServerReply := func(mytoken string) bool {
		status := serverMap[mytoken]

		healthy := status.Database.Online && status.Node.Online

		// no current master?
		if serverMaster.token == "" {
			if !healthy {
				logger.Warnf("no current master - %v is unhealthy", mytoken)
				return false
			}
			uptime := time.Since(timeStarted)
			if status.Database.Master || uptime >= waitForMaster {
				logger.Warnf("no current master - promoting %v", mytoken)
				serverMaster = smi{
					token: mytoken,
				}
				return true
			}

			logger.Infof("no current master - waiting %v longer", waitForMaster-uptime)
			return false
		}

		now := time.Now()

		// still master and healthy?
		if serverMaster.token == mytoken {
			if !status.Database.Master {
				logger.Warnf("current master %v claims they are no longer master", mytoken)
				serverMaster = smi{}
				return false
			}

			if healthy {
				logger.Infof("current master %v is still healthy", mytoken)
				serverMaster = smi{
					token: mytoken,
				}
				return true
			}

			if !status.Database.Online {
				if serverMaster.dboffline.IsZero() {
					logger.Warnf("current master %v db is offline", mytoken)
					serverMaster.dboffline = now
				} else if now.Sub(serverMaster.dboffline) > time.Minute {
					logger.Warnf("current master %v db offline too long -- demoting", mytoken)
					serverMaster = smi{}
					return false
				} else {
					logger.Warnf("current master %v db offline for %v", mytoken, time.Since(serverMaster.dboffline))
				}
			}
			if !status.Node.Online {
				if serverMaster.nodeoffline.IsZero() {
					logger.Warnf("current master %v dcrlnd is offline", mytoken)
					serverMaster.nodeoffline = now
				} else if now.Sub(serverMaster.nodeoffline) > time.Minute {
					logger.Warnf("current master %v dcrlnd offline too long -- demoting", mytoken)
					serverMaster = smi{}
					return false
				} else {
					logger.Warnf("current master %v dcrlnd offline for %v", mytoken, time.Since(serverMaster.nodeoffline))
				}
			}
		}

		masterStatus := serverMap[serverMaster.token]

		// master disappeared for over a minute - switch
		if now.Sub(time.Unix(masterStatus.LastUpdated, 0)) > time.Minute {
			logger.Warnf("master %v has been offline too long -- promoting %v", serverMaster.token, mytoken)
			serverMaster = smi{token: mytoken}
			return true
		}
		if !serverMaster.dboffline.IsZero() &&
			now.Sub(serverMaster.dboffline) > time.Minute {
			logger.Warnf("master %v db has been offline too long -- promoting %v", serverMaster.token, mytoken)
			serverMaster = smi{token: mytoken}
			return true
		}
		if !serverMaster.nodeoffline.IsZero() &&
			now.Sub(serverMaster.nodeoffline) > time.Minute {
			logger.Warnf("master %v dcrlnd has been offline too long -- promoting %v", serverMaster.token, mytoken)
			serverMaster = smi{token: mytoken}
			return true
		}

		return false
	}

	// create the http response for brclients.
	createClientReply := func() clientintf.ClientAPI {
		serverMtx.Lock()
		defer serverMtx.Unlock()

		var clientAPI clientintf.ClientAPI
		for tokenStr, status := range serverMap {
			serverAddr := net.JoinHostPort(status.Node.Alias, "443")
			nodeAddr := net.JoinHostPort(status.Node.Alias, "9735")

			isMaster := serverMaster.token == tokenStr

			clientAPI.ServerGroups = append(clientAPI.ServerGroups, clientintf.ServerGroup{
				Server:   serverAddr,
				LND:      fmt.Sprintf("%s@%s", status.Node.PublicKey, nodeAddr),
				IsMaster: isMaster,
				Online:   time.Since(time.Unix(status.LastUpdated, 0)) < time.Minute && status.Database.Online && status.Node.Online,
			})
		}
		return clientAPI
	}

	var upgrader = websocket.Upgrader{
		HandshakeTimeout: time.Minute,
		ReadBufferSize:   1024,
		WriteBufferSize:  1024,
	}

	mux := http.NewServeMux()

	// serve api for brservers
	mux.HandleFunc("/api/v1/status", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		tokenStr := auth[len("Bearer "):]
		if _, exists := tokens[tokenStr]; !exists {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Errorf("%v", err)
			return
		}
		defer conn.Close()

		remoteAddr := conn.RemoteAddr()

		// safety
		conn.SetReadLimit(1024 * 1024)

		conn.SetPingHandler(func(str string) error {
			conn.SetReadDeadline(time.Now().Add(time.Minute))
			err := conn.WriteControl(websocket.PongMessage, []byte(str),
				time.Now().Add(15*time.Second))
			if err != nil {
				logger.Errorf("failed to send pong to %v: %v", remoteAddr, str)
				return err
			}
			return nil
		})

		type request struct {
			JSONRPC string            `json:"jsonrpc"`
			Method  string            `json:"method"`
			Params  []json.RawMessage `json:"params,omitempty"`
			ID      uint32            `json:"id"`
		}
		type response struct {
			Result json.RawMessage `json:"result"`
			Error  *RPCError       `json:"error"`
			ID     uint32          `json:"id"`
		}

		var params []byte
		conn.SetReadDeadline(time.Now().Add(time.Minute))
		for {
			var req request
			err := conn.ReadJSON(&req)
			conn.SetReadDeadline(time.Now().Add(time.Minute))
			if err != nil {
				logger.Errorf("server %v: %v", remoteAddr, err)
				break
			}
			if len(req.Params) == 0 {
				logger.Warnf("server %v sent no params", remoteAddr)
				break
			}

			var rpcError RPCError
			switch req.Method {
			case "status":
				var status server.CommandStatus
				if err = json.Unmarshal(req.Params[0], &status); err != nil {
					logger.Errorf("failed to parse status from %v: %v", remoteAddr, err)
					rpcError.Message = "failed to parse status json"
					break
				}
				if status.Node.Alias == "" {
					logger.Warnf("no alias set from %v", remoteAddr)
					rpcError.Message = "no alias set"
					break
				}

				// fix your clock.
				if time.Since(time.Unix(status.LastUpdated, 0)) > 5*time.Minute {
					logger.Warnf("last update is too old from %v", remoteAddr)
					rpcError.Message = "lastUpdated is too old"
					break
				}
				status.LastUpdated = time.Now().Unix()

				serverMtx.Lock()
				serverMap[tokenStr] = &status
				rep := createServerReply(tokenStr)
				serverMtx.Unlock()

				statusReply := server.CommandStatusReply{
					Master: rep,
				}
				params, err = json.Marshal(statusReply)
				if err != nil {
					logger.Errorf("failed to marshal status reply: %v", err)
					rpcError.Message = "server side error"
					break
				}
			default:
				logger.Warnf("unhandled command %q from %v", req.Method, remoteAddr)
				rpcError.Message = "unknown command"
			}

			reply := response{
				ID:     req.ID,
				Result: json.RawMessage(params),
			}
			if rpcError.Message != "" {
				reply.Error = &rpcError
			}

			if err = conn.WriteJSON(&reply); err != nil {
				logger.Errorf("failed to write status response to %v: %v", remoteAddr, err)
				break
			}

		}
	})

	// serve api for brclients
	mux.HandleFunc("/api/v1/live", func(w http.ResponseWriter, r *http.Request) {
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
