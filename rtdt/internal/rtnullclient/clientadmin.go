package main

import (
	"fmt"
	"math/rand/v2"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/companyzero/bisonrelay/rpc"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Burst that simulates opus-encoded speech with the default
	// settings (mono, 40kbps target bitrate, CBR, no DTX).
	speechBurst = outBurst{
		Interval:   20 * time.Millisecond,
		Packets:    1,
		PacketSize: uniformDistribution{minVal: 100, maxVal: 200},
	}

	// Burst that tries to flood 1k packets/second (1 packet every
	// millisecond).
	flood1k = outBurst{
		Interval:   time.Millisecond,
		Packets:    1,
		PacketSize: uniformDistribution{minVal: 100, maxVal: 200},
	}

	// Burst that tries to flood 10k packets/second (10 packets every
	// millisecond).
	flood10k = outBurst{
		Interval:   time.Millisecond,
		Packets:    10,
		PacketSize: uniformDistribution{minVal: 100, maxVal: 200},
	}

	// Burst that tries to flood 50k packets/second (50 packets every
	// millisecond).
	flood50k = outBurst{
		Interval:   time.Millisecond,
		Packets:    50,
		PacketSize: uniformDistribution{minVal: 100, maxVal: 200},
	}

	// Burst that tries to flood 75k packets/second (75 packets every
	// millisecond).
	flood75k = outBurst{
		Interval:   time.Millisecond,
		Packets:    75,
		PacketSize: uniformDistribution{minVal: 100, maxVal: 200},
	}
)

func (c *client) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/index", c.handleIndex)
	mux.HandleFunc("/addSpeech", c.handleAddSpeech)
	mux.HandleFunc("/join", c.handleJoin)
	mux.HandleFunc("/batch", c.handleBatch)
	mux.Handle("/metrics", promhttp.Handler())
	return mux
}

func (c *client) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("null RTDT client - admin interface\n"))
}

func (c *client) handleAddSpeech(w http.ResponseWriter, r *http.Request) {
	id := rpc.RTDTPeerID(rand.Uint64())
	if strID := r.URL.Query().Get("id"); strID != "" {
		v, err := strconv.ParseUint(strID, 16, 32)
		if err == nil {
			id = rpc.RTDTPeerID(v)
		}
	} else if strSess := r.URL.Query().Get("sess"); strSess != "" {
		v, err := strconv.ParseUint(strSess, 16, 16)
		if err == nil {
			id = (rpc.RTDTPeerID(v) << 16) | (id & 0x0000ffff)
		}
	}

	var connID int
	if strConn := r.URL.Query().Get("conn"); strConn != "" {
		v, err := strconv.ParseUint(strConn, 16, 32)
		if err == nil {
			connID = int(v)
		}
	}

	bursts := []outBurst{speechBurst}
	err := c.newSession(connID, id, bursts)
	if err != nil {
		fmt.Fprintf(w, "Error creating speech session: %v\n", err)
	} else {
		fmt.Fprintln(w, "Created speech session")
	}
}

func (c *client) handleJoin(w http.ResponseWriter, r *http.Request) {
	id := rpc.RTDTPeerID(rand.Uint64())
	if strID := r.URL.Query().Get("id"); strID != "" {
		v, err := strconv.ParseUint(strID, 16, 32)
		if err == nil {
			id = rpc.RTDTPeerID(v)
		}
	} else if strSess := r.URL.Query().Get("sess"); strSess != "" {
		v, err := strconv.ParseUint(strSess, 16, 16)
		if err == nil {
			id = (rpc.RTDTPeerID(v) << 16) | (id & 0x0000ffff)
		}
	}

	var connID int
	if strConn := r.URL.Query().Get("conn"); strConn != "" {
		v, err := strconv.ParseUint(strConn, 16, 32)
		if err == nil {
			connID = int(v)
		}
	}

	err := c.newSession(connID, id, nil)
	if err != nil {
		fmt.Fprintf(w, "Error joining session: %v\n", err)
	} else {
		fmt.Fprintf(w, "Joined session %s\n", id)
	}
}

func (c *client) handleBatch(w http.ResponseWriter, r *http.Request) {
	var batchSession rpc.RTDTPeerID

	if strSess := r.URL.Query().Get("sess"); strSess != "" {
		v, err := strconv.ParseUint(strSess, 16, 16)
		if err == nil {
			batchSession = (rpc.RTDTPeerID(v) << 16)
		}
	}

	// How many sessions to open up.
	var count int = 100
	if str := r.URL.Query().Get("count"); str != "" {
		v, err := strconv.ParseUint(str, 10, 32)
		if err == nil {
			count = int(v)
		}
	}

	// How many sessions to multiplex per conn.
	var sessPerConn = 20
	if str := r.URL.Query().Get("sessPerConn"); str != "" {
		v, err := strconv.ParseUint(str, 10, 32)
		if err == nil {
			sessPerConn = int(v)
		}
	}

	// How many sessions per second to create (on average).
	var sessCreationPerSec = 20
	if str := r.URL.Query().Get("sessPerSec"); str != "" {
		v, err := strconv.ParseUint(str, 10, 32)
		if err == nil {
			sessCreationPerSec = int(v)
		}
	}
	sleepDuration := time.Second * time.Duration(count+1) / time.Duration(sessCreationPerSec)

	// Determine whether to use a single conn (query parameter conn is
	// specified) or use different conns per session.
	var batchConn int
	if str := r.URL.Query().Get("conn"); str != "" {
		v, err := strconv.ParseUint(str, 16, 32)
		if err == nil {
			batchConn = int(v)
		}
	}

	var startConnId int = 1

	// Determine base peer id. If peer id is specified, also ensure the
	// conn id is unique per peer id (so we don't use the same conn for
	// different peers).
	var peerID rpc.RTDTPeerID = c.basePeerID
	if str := r.URL.Query().Get("peer"); str != "" {
		v, err := strconv.ParseUint(str, 16, 32)
		if err == nil {
			peerID = rpc.RTDTPeerID(v)
			startConnId = int(v) << 16
		}
	}

	var bursts []outBurst

	if r.URL.Query().Has("addSpeech") {
		bursts = append(bursts, speechBurst)
	}
	if r.URL.Query().Has("flood1k") {
		bursts = append(bursts, flood1k)
	}
	if r.URL.Query().Has("flood10k") {
		bursts = append(bursts, flood10k)
	}
	if r.URL.Query().Has("flood50k") {
		bursts = append(bursts, flood50k)
	}
	if r.URL.Query().Has("flood75k") {
		bursts = append(bursts, flood75k)
	}

	var wg sync.WaitGroup
	doneMsg := make(chan string, count)
	for i := 1; i <= count; i++ {
		conn := batchConn
		id := batchSession
		if batchConn == 0 { // Use different conns per session
			conn = startConnId + (i-1)/sessPerConn
		}
		if batchSession == 0 { // Use different sessions
			id = rpc.RTDTPeerID(i<<16) | peerID
		}
		wg.Add(1)
		go func() {
			// Spread creation around 1 second / 50 sessions.
			time.Sleep(time.Millisecond + randomDuration(sleepDuration))
			err := c.newSession(conn, id, bursts)
			var msg string
			if err != nil {
				c.log.Errorf("Unable to join session %s through conn %d: %v", id, conn, err)
				msg = fmt.Sprintf("Error joining session: %v\n", err)
			} else {
				msg = fmt.Sprintf("Joined session %s through conn 0x%x\n", id, conn)
			}
			doneMsg <- msg
			wg.Done()
		}()
		// time.Sleep(time.Millisecond * 10)
	}

	go func() {
		wg.Wait()
		close(doneMsg)
	}()

	for msg := range doneMsg {
		w.Write([]byte(msg))
	}
}
