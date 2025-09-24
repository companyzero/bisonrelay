package seederserver

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/companyzero/bisonrelay/rpc"
	"github.com/gorilla/websocket"
)

type smi struct {
	token       string
	dboffline   time.Time
	nodeoffline time.Time
}

// create the http response for brclients.
func (s *Server) createClientReply() rpc.SeederClientAPI {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	var clientAPI rpc.SeederClientAPI
	for tokenStr, status := range s.serverMap {
		serverAddr := net.JoinHostPort(status.Node.Alias, "443")
		nodeAddr := net.JoinHostPort(status.Node.Alias, "9735")

		isMaster := s.serverMaster.token == tokenStr

		clientAPI.ServerGroups = append(clientAPI.ServerGroups, rpc.SeederServerGroup{
			Server:   serverAddr,
			LND:      fmt.Sprintf("%s@%s", status.Node.PublicKey, nodeAddr),
			IsMaster: isMaster,
			Online:   time.Since(time.Unix(status.LastUpdated, 0)) < time.Minute && status.Database.Online && status.Node.Online,
		})
	}
	return clientAPI
}

func (s *Server) handleClientStatusQuery(w http.ResponseWriter, r *http.Request) {
	flush, ok := w.(http.Flusher)
	if !ok {
		http.NotFound(w, r)
		return
	}

	//w.Header().Set("Content-Type", "text/plain; charset=utf-8") // not a json array
	w.Header().Set("Content-Type", "application/json")
	// Replace the Server response header. When used with nginx's "server_tokens
	// off;" and "proxy_pass_header Server;" options.
	w.Header().Set("Server", s.cfg.appName)
	w.WriteHeader(http.StatusOK)
	flush.Flush()

	reply := s.createClientReply()
	if err := json.NewEncoder(w).Encode(reply); err != nil {
		s.log.Errorf("Error encoding client query reply: %v", err)
	}
}

// create the http response for brservers.
// must be called with serverMtx locked.
func (s *Server) createServerReply(mytoken string) bool {
	status := s.serverMap[mytoken]

	healthy := status.Database.Online && status.Node.Online

	// no current master?
	if s.serverMaster.token == "" {
		if !healthy {
			s.log.Warnf("no current master - %v is unhealthy", mytoken)
			return false
		}
		uptime := time.Since(s.timeStarted)
		if status.Database.Master || uptime >= s.cfg.waitForMaster {
			s.log.Warnf("no current master - promoting %v", mytoken)
			s.serverMaster = smi{
				token: mytoken,
			}
			return true
		}

		s.log.Infof("no current master - waiting %v longer", s.cfg.waitForMaster-uptime)
		return false
	}

	now := time.Now()

	// still master and healthy?
	if s.serverMaster.token == mytoken {
		if !status.Database.Master {
			s.log.Warnf("current master %v claims they are no longer master", mytoken)
			s.serverMaster = smi{}
			return false
		}

		if healthy {
			s.log.Infof("current master %v is still healthy", mytoken)
			s.serverMaster = smi{
				token: mytoken,
			}
			return true
		}

		if !status.Database.Online {
			if s.serverMaster.dboffline.IsZero() {
				s.log.Warnf("current master %v db is offline", mytoken)
				s.serverMaster.dboffline = now
			} else if now.Sub(s.serverMaster.dboffline) > time.Minute {
				s.log.Warnf("current master %v db offline too long -- demoting", mytoken)
				s.serverMaster = smi{}
				return false
			} else {
				s.log.Warnf("current master %v db offline for %v", mytoken, time.Since(s.serverMaster.dboffline))
			}
		}
		if !status.Node.Online {
			if s.serverMaster.nodeoffline.IsZero() {
				s.log.Warnf("current master %v dcrlnd is offline", mytoken)
				s.serverMaster.nodeoffline = now
			} else if now.Sub(s.serverMaster.nodeoffline) > time.Minute {
				s.log.Warnf("current master %v dcrlnd offline too long -- demoting", mytoken)
				s.serverMaster = smi{}
				return false
			} else {
				s.log.Warnf("current master %v dcrlnd offline for %v", mytoken, time.Since(s.serverMaster.nodeoffline))
			}
		}
	}

	masterStatus := s.serverMap[s.serverMaster.token]

	// master disappeared for over a minute - switch
	if now.Sub(time.Unix(masterStatus.LastUpdated, 0)) > time.Minute {
		s.log.Warnf("master %v has been offline too long -- promoting %v", s.serverMaster.token, mytoken)
		s.serverMaster = smi{token: mytoken}
		return true
	}
	if !s.serverMaster.dboffline.IsZero() &&
		now.Sub(s.serverMaster.dboffline) > time.Minute {
		s.log.Warnf("master %v db has been offline too long -- promoting %v", s.serverMaster.token, mytoken)
		s.serverMaster = smi{token: mytoken}
		return true
	}
	if !s.serverMaster.nodeoffline.IsZero() &&
		now.Sub(s.serverMaster.nodeoffline) > time.Minute {
		s.log.Warnf("master %v dcrlnd has been offline too long -- promoting %v", s.serverMaster.token, mytoken)
		s.serverMaster = smi{token: mytoken}
		return true
	}

	return false
}

func (s *Server) handleBRServerStatus(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	tokenStr := auth[len("Bearer "):]
	if _, exists := s.cfg.tokens[tokenStr]; !exists {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Errorf("%v", err)
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
			s.log.Errorf("failed to send pong to %v: %v", remoteAddr, str)
			return err
		}
		return nil
	})

	// RPCError represents a JSON-RPC error object.
	type RPCError struct {
		Code    int64           `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data,omitempty"`
	}
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
			s.log.Errorf("server %v: %v", remoteAddr, err)
			break
		}
		if len(req.Params) == 0 {
			s.log.Warnf("server %v sent no params", remoteAddr)
			break
		}

		var rpcError RPCError
		switch req.Method {
		case "status":
			var status rpc.SeederCommandStatus
			if err = json.Unmarshal(req.Params[0], &status); err != nil {
				s.log.Errorf("failed to parse status from %v: %v", remoteAddr, err)
				rpcError.Message = "failed to parse status json"
				break
			}
			if status.Node.Alias == "" {
				s.log.Warnf("no alias set from %v", remoteAddr)
				rpcError.Message = "no alias set"
				break
			}

			// fix your clock.
			if time.Since(time.Unix(status.LastUpdated, 0)) > 5*time.Minute {
				s.log.Warnf("last update is too old from %v", remoteAddr)
				rpcError.Message = "lastUpdated is too old"
				break
			}
			status.LastUpdated = time.Now().Unix()

			s.mtx.Lock()
			s.serverMap[tokenStr] = &status
			rep := s.createServerReply(tokenStr)
			s.mtx.Unlock()

			statusReply := rpc.SeederCommandStatusReply{
				Master: rep,
			}
			params, err = json.Marshal(statusReply)
			if err != nil {
				s.log.Errorf("failed to marshal status reply: %v", err)
				rpcError.Message = "server side error"
				break
			}
		default:
			s.log.Warnf("unhandled command %q from %v", req.Method, remoteAddr)
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
			s.log.Errorf("failed to write status response to %v: %v", remoteAddr, err)
			break
		}
	}
}
