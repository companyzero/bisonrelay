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
		isOnline := time.Since(time.Unix(status.LastUpdated, 0)) < s.cfg.offlineLimit &&
			status.Database.Online &&
			status.Node.Online

		clientAPI.ServerGroups = append(clientAPI.ServerGroups, rpc.SeederServerGroup{
			Server:   serverAddr,
			LND:      fmt.Sprintf("%s@%s", status.Node.PublicKey, nodeAddr),
			IsMaster: isMaster,
			Online:   isOnline,
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

type statusUpdateAction int

const (
	actionNone statusUpdateAction = iota
	actionPromote
	actionDemote
)

// processStatusUpdate processes a status update from a remove brserver
// instance.
func (s *Server) processStatusUpdate(mytoken string, status rpc.SeederCommandStatus) (isMaster bool, err error) {
	if status.Node.Alias == "" {
		return false, errNoAlias
	}

	// fix your clock.
	now := time.Now()
	if now.Sub(time.Unix(status.LastUpdated, 0)) > 5*time.Minute {
		return false, errLastUpdateTooOld
	}

	// Helper to check if time since offlineTime is > 1 minute.
	offlineTooLong := func(offlineTime time.Time) bool {
		return !offlineTime.IsZero() && now.Sub(offlineTime) > s.cfg.offlineLimit
	}

	// Helper to log the correct offline time.
	logOfflineSince := func(subsys string, t time.Time) string {
		if t.IsZero() {
			return ""
		}
		return fmt.Sprintf("%s offline for %s", subsys, now.Sub(t))
	}

	// Rest of function is with state mutex locked.
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.serverMap[mytoken] = status

	// Relevant vars.
	uptime := now.Sub(s.timeStarted)
	healthy := status.Database.Online && status.Node.Online
	hasMaster := s.serverMaster.token != ""
	isMaster = hasMaster && s.serverMaster.token == mytoken
	masterStatus := s.serverMap[s.serverMaster.token]
	var action = actionNone

	// Decide what to do.
	switch {
	case !hasMaster && !healthy:
		s.log.Warnf("no current master - %v is unhealthy", mytoken)

	case !hasMaster && status.Database.Master:
		s.log.Warnf("no current master - promoting %v due to being DB master", mytoken)
		action = actionPromote

	case !hasMaster && uptime >= s.cfg.waitForMaster:
		s.log.Warnf("no current master - promoting %v due to uptime %v elapsed %v", mytoken,
			uptime, s.cfg.waitForMaster)
		action = actionPromote

	case !hasMaster:
		s.log.Infof("no current master - waiting %v longer", s.cfg.waitForMaster-uptime)

	case isMaster && !status.Database.Master:
		s.log.Warnf("current master %v claims they are no longer master", mytoken)
		action = actionDemote

	case isMaster && healthy:
		s.log.Infof("current master %v is still healthy", mytoken)

		// Zero out offline times in case master came back from offline.
		s.serverMaster.dboffline = time.Time{}
		s.serverMaster.nodeoffline = time.Time{}

	case isMaster && !status.Database.Online && offlineTooLong(s.serverMaster.dboffline):
		s.log.Warnf("current master %v db offline too long -- demoting", mytoken)
		action = actionDemote

	case isMaster && !status.Node.Online && offlineTooLong(s.serverMaster.nodeoffline):
		s.log.Warnf("current master %v dcrlnd offline too long -- demoting", mytoken)
		action = actionDemote

	case isMaster:
		// Not healthy (healthy and demotion cases already handled). Set
		// or reset offline times.
		if !status.Database.Online && s.serverMaster.dboffline.IsZero() {
			s.serverMaster.dboffline = now
		} else if status.Database.Online {
			s.serverMaster.dboffline = time.Time{} // Db came back.
		}
		if !status.Node.Online && s.serverMaster.nodeoffline.IsZero() {
			s.serverMaster.nodeoffline = now
		} else if status.Node.Online {
			s.serverMaster.nodeoffline = time.Time{} // LN came back.
		}

		s.log.Warnf("current master %v not healthy %s %s",
			mytoken, logOfflineSince("db", s.serverMaster.dboffline),
			logOfflineSince("dcrlnd", s.serverMaster.nodeoffline))

	case offlineTooLong(time.Unix(masterStatus.LastUpdated, 0)):
		s.log.Warnf("master %v has been offline too long (%s) -- promoting %v",
			s.serverMaster.token, now.Sub(time.Unix(masterStatus.LastUpdated, 0)), mytoken)
		action = actionPromote

	case offlineTooLong(s.serverMaster.dboffline):
		s.log.Warnf("master %v db has been offline too long (%s) -- promoting %v",
			s.serverMaster.token, now.Sub(s.serverMaster.dboffline), mytoken)
		action = actionPromote

	case offlineTooLong(s.serverMaster.nodeoffline):
		s.log.Warnf("master %v dcrlnd has been offline too long (%s) -- promoting %v",
			s.serverMaster.token, now.Sub(s.serverMaster.nodeoffline), mytoken)
		action = actionPromote

	default:
		s.log.Infof("status update from non-master %v (dbOnline=%v nodeOnline=%v)",
			mytoken, status.Database.Online, status.Node.Online)
	}

	// Take action.
	switch action {
	case actionPromote:
		s.serverMaster = smi{token: mytoken}
		isMaster = true

	case actionDemote:
		s.serverMaster = smi{}
		isMaster = false

		// Find a new master?
	}

	return
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

	conn.SetPingHandler(func(str string) error {
		s.log.Infof("Received ping from %v", remoteAddr)
		err := conn.WriteControl(websocket.PongMessage, []byte(str),
			time.Now().Add(20*time.Second))
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
	for {
		var req request
		err := conn.ReadJSON(&req)
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
			s.log.Infof("Received status from %v (%v)", tokenStr, remoteAddr)
			var newStatus rpc.SeederCommandStatus
			if err = json.Unmarshal(req.Params[0], &newStatus); err != nil {
				s.log.Errorf("failed to parse status from %v: %v", remoteAddr, err)
				rpcError.Message = "failed to parse status json"
				break
			}

			isMaster, err := s.processStatusUpdate(tokenStr, newStatus)
			if err != nil {
				s.log.Errorf("Error processing update from %s: %v", remoteAddr, err)
				rpcError.Message = err.Error()
				break
			}

			statusReply := rpc.SeederCommandStatusReply{
				Master: isMaster,
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
