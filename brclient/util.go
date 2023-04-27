package main

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"net"
	"runtime"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/decred/dcrlnd"
	"github.com/decred/dcrlnd/lnrpc"
)

const (
	defaultLNPort = "9735"

	ISO8601DateTimeMs = "2006-01-02 15:04:05.000"
	ISO8601DateTime   = "2006-01-02 15:04:05"
	ISO8601Date       = "2006-01-02"
)

// Helper mixin to avoid having to add an Init() function everywhere.
type initless struct{}

func (_ initless) Init() tea.Cmd { return nil }

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// clamp v such that min <= v <= max
func clamp(v, min, max int) int {
	if v > max {
		v = max
	}
	if v < min {
		v = min
	}
	return v
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func limitStr(s string, maxLen int) string {
	if maxLen >= 0 && len(s) > maxLen {
		return s[:maxLen]
	}
	return s
}

// ltjustify truncates or left-justifies the given string, by either adding
// or removing characters at the end of the string until it is of the specified
// length.
func ltjustify(s string, l int) string {
	if len(s) >= l {
		return s[:l]
	} else {
		return s + strings.Repeat(" ", l-len(s))
	}
}

// Helper to only append non nil cmd to cmds.
func appendCmd(cmds []tea.Cmd, cmd ...tea.Cmd) []tea.Cmd {
	for i := range cmd {
		if cmd[i] == nil {
			continue
		}
		cmds = append(cmds, cmd[i])
	}
	return cmds
}

func countNewLines(s string) int {
	return strings.Count(s, "\n")
}

// batchCmds maybe batches the list of cmds if needed.
func batchCmds(cmds []tea.Cmd) tea.Cmd {
	switch len(cmds) {
	case 0:
		return nil
	case 1:
		return cmds[0]
	default:
		return tea.Batch(cmds...)
	}
}

// mentionPosition returns the position where the given mention starts in the
// message or -1 if there is no mention.
func mentionPosition(me, msg string) int {
	return strings.Index(strings.ToUpper(msg),
		strings.ToUpper(me))
}

// hasMention returns true if the given msg mentions the given nick.
func hasMention(me, msg string) bool {
	return mentionPosition(me, msg) > -1
}

// allStack returns the full stack trace of all goroutines.
func allStack() []byte {
	buf := make([]byte, 1024)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			return buf[:n]
		}
		buf = make([]byte, 2*len(buf))
	}
}

func chanPointToStr(cp *lnrpc.ChannelPoint) string {
	tx, err := dcrlnd.GetChanPointFundingTxid(cp)
	if err != nil {
		return fmt.Sprintf("[%v]", err)
	}
	return fmt.Sprintf("%s:%d", tx, cp.OutputIndex)
}

func strToChanPoint(str string) (*lnrpc.ChannelPoint, error) {
	p := strings.Index(str, ":")
	if p < 0 {
		return nil, fmt.Errorf("channel point does not have output index")
	}
	txid := str[:p]
	if len(txid) != 64 {
		return nil, fmt.Errorf("channel point txid does not have "+
			"required length (%d != 64)", len(txid))
	}
	txidBytes, err := hex.DecodeString(txid)
	if err != nil {
		return nil, fmt.Errorf("channel point txid not a valid hex: %v", err)
	}

	// Reverse the bytes, as the hash is reversed.
	for i, j := 0, len(txidBytes)-1; i < j; i, j = i+1, j-1 {
		txidBytes[i], txidBytes[j] = txidBytes[j], txidBytes[i]

	}
	outIdx, err := strconv.ParseUint(str[p+1:], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("channel point output index not valid: %v", err)
	}
	cp := &lnrpc.ChannelPoint{
		FundingTxid: &lnrpc.ChannelPoint_FundingTxidBytes{
			FundingTxidBytes: txidBytes,
		},
		OutputIndex: uint32(outIdx),
	}
	return cp, nil
}

// maxCmdLen returns the length of the longest command in the slice.
func maxCmdLen(cmds []tuicmd) int {
	var max int
	for _, cmd := range cmds {
		l := len(cmd.cmd)
		if l > max {
			max = l
		}
	}
	return max
}

// fingerprintDER returns the fingerprint of the standard TLS cert.
func fingerprintDER(c []*x509.Certificate) string {
	if len(c) != 1 {
		return "unexpected chained certificate"
	}

	d := sha256.New()
	d.Write(c[0].Raw)
	digest := d.Sum(nil)
	return hex.EncodeToString(digest)
}

// hbytes == "human bytes"
func hbytes(i int64) string {
	switch {
	case i < 1e3:
		return strconv.FormatInt(i, 10) + "B"
	case i < 1e6:
		f := float64(i)
		return strconv.FormatFloat(f/1e3, 'f', 2, 64) + "KB"
	case i < 1e9:
		f := float64(i)
		return strconv.FormatFloat(f/1e6, 'f', 2, 64) + "MB"
	case i < 1e12:
		f := float64(i)
		return strconv.FormatFloat(f/1e9, 'f', 2, 64) + "GB"
	case i < 1e15:
		f := float64(i)
		return strconv.FormatFloat(f/1e12, 'f', 2, 64) + "TB"
	default:
		return strconv.FormatInt(i, 10)
	}
}

func programByMimeType(mimeMap map[string]string, t string) string {
	f, exists := mimeMap[t]
	if exists {
		return f
	}
	sp := strings.Split(t, "/")
	if len(sp) != 2 {
		return ""
	}
	catchall := fmt.Sprintf("%v/*", sp[0])
	if n, exists := mimeMap[catchall]; exists {
		return n
	}
	return ""
}

// blankLines returns blank lines if nb > 0.
func blankLines(nb int) string {
	if nb <= 0 {
		return ""
	}
	return strings.Repeat("\n", nb)
}

func normalizeAddress(addr string, defaultPort string) string {
	port := defaultPort
	if a, p, err := net.SplitHostPort(addr); err == nil {
		addr = a
		port = p
	}
	return net.JoinHostPort(addr, port)
}

// stringsCommonPrefix returns the common prefix shared by all strings in the
// slice.
func stringsCommonPrefix(src []string) string {
	if len(src) == 0 {
		return ""
	}
	res := src[0]
	for i := 1; i < len(src); i++ {
		if len(src[i]) < len(res) {
			res = res[:len(src[i])]
		}
		for j := 0; j < len(res); j++ {
			if res[j] != src[i][j] {
				res = res[:j]
				break
			}
		}
		if len(res) == 0 {
			return ""
		}
	}
	return res
}

// channelBalanceDisplay generates a balance display for a channel.
func channelBalanceDisplay(local, remote int64) string {
	var max int = 10
	var c = "·"
	plocal := int(float64(local) / float64(local+remote) * 10)
	plocal = clamp(plocal, 0, 9)
	sep := "|"
	if plocal == 0 {
		sep = "<"
	} else if plocal == max-1 {
		sep = ">"
	}
	return fmt.Sprintf("[%s%s%s]", strings.Repeat(c, plocal), sep,
		strings.Repeat(c, max-plocal-1))
}
