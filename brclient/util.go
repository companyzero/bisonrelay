package main

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/decred/dcrlnd"
	"github.com/decred/dcrlnd/lnrpc"
)

const (
	ISO8601DateTime = "2006-01-02 15:04:00"
	ISO8601Date     = "2006-01-02"
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

type embeddedArgs struct {
	// embedded file
	name string
	data []byte
	alt  string
	typ  string

	// shared link
	download zkidentity.ShortID
	filename string
	size     uint64
	cost     uint64
}

func (args embeddedArgs) String() string {
	var parts []string
	if args.name != "" {
		parts = append(parts, "part="+args.name)
	}
	if args.alt != "" {
		parts = append(parts, "alt="+args.alt)
	}
	if args.typ != "" {
		parts = append(parts, "type="+args.typ)
	}
	if !args.download.IsEmpty() {
		parts = append(parts, "download="+args.download.String())
	}
	if args.filename != "" {
		parts = append(parts, "filename="+args.filename)
	}
	if args.size > 0 {
		parts = append(parts, "size="+strconv.FormatUint(args.size, 10))
	}
	if args.cost > 0 {
		parts = append(parts, "cost="+strconv.FormatUint(args.cost, 10))
	}
	if args.data != nil {
		parts = append(parts, "data="+base64.StdEncoding.EncodeToString(args.data))
	}

	return "--embed[" + strings.Join(parts, ",") + "]--"
}

var embedRegexp = regexp.MustCompile(`--embed\[.*?\]--`)

func parseEmbedArgs(rawEmbedStr string) embeddedArgs {
	// Copy everything between the [] (the raw argument list).
	start, end := strings.Index(rawEmbedStr, "["), strings.LastIndex(rawEmbedStr, "]")
	rawArgs := rawEmbedStr[start+1 : end]

	// Split args by comma.
	splitArgs := strings.Split(rawArgs, ",")

	// Decode args.
	var args embeddedArgs
	for _, a := range splitArgs {
		// Split by "="
		kv := strings.SplitN(a, "=", 2)
		if len(kv) != 2 {
			continue
		}
		k, v := kv[0], kv[1]
		switch k {
		case "name":
			args.name = v
		case "type":
			args.typ = v
		case "data":
			decoded, err := base64.StdEncoding.DecodeString(v)
			if err != nil {
				decoded = []byte(fmt.Sprintf("[err decoding data: %v]", err))
			}
			args.data = decoded
		case "alt":
			decoded, err := url.PathUnescape(v)
			if err != nil {
				decoded = fmt.Sprintf("[err processing alt: %v]", err)
			}
			args.alt = decoded
		case "download":
			// Ignore the error and leave download empty.
			args.download.FromString(v)
		case "filename":
			args.filename = v
		case "size":
			args.size, _ = strconv.ParseUint(v, 10, 64)
		case "cost":
			args.cost, _ = strconv.ParseUint(v, 10, 64)
		}
	}

	return args
}

// replaceEmbeds replaces all the embeds tags of the given text with the result
// of the calling function.
func replaceEmbeds(src string, replF func(args embeddedArgs) string) string {
	return embedRegexp.ReplaceAllStringFunc(src, func(repl string) string {
		// Decode args.
		args := parseEmbedArgs(repl)
		return replF(args)
	})
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
