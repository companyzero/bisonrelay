package main

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"hash/maphash"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrlnd/lnrpc"
	"github.com/decred/dcrlnd/zpay32"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

const (
	defaultLNPort = "9735"

	ISO8601DateTimeMs = "2006-01-02 15:04:05.000"
	ISO8601DateTime   = "2006-01-02 15:04:05"
	ISO8601Date       = "2006-01-02"
)

const baseExternalNewPostContent = `

--endofpost--

Everything after the first "--endofpost--" line will be removed by brclient
before creating the post.

To include an embedded content, include the following snippet:

--embed[alt=some+alt,type=image/png,localfilename=test.png]--

Replace the "alt=" attribute with an URL-encoded description of the image. Set
the "type=" attribute to the image type, and set "localfilename=" to the
filename of the image to embed.

Use absolute file paths. Relative paths are based on either the post file
(when using /post new <filename>) or on the CWD of brclient (usually the dir
from which brclient was executed).
`

// Helper mixin to avoid having to add an Init() function everywhere.
type initless struct{}

func (initless) Init() tea.Cmd { return nil }

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
	}
	return s + strings.Repeat(" ", l-len(s))
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

// stringsContains returns true if any of the strings contains the substring.
func stringsContains(s []string, substr string) bool {
	for i := range s {
		if strings.Contains(s[i], substr) {
			return true
		}
	}
	return false
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
	tx, err := lnrpc.GetChanPointFundingTxid(cp)
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

	switch runtime.GOOS {
	case "darwin":
		return "open"
	case "windows":
		return "start"
	default:
		return "xdg-open"
	}
}

func fromHex(h string) []byte {
	v, err := hex.DecodeString(h)
	if err != nil {
		panic(err)
	}
	return v
}

var (
	// https://en.wikipedia.org/wiki/List_of_file_signatures
	knownImageTypes = map[string][][]byte{
		"image/png": {fromHex("89504e470d0a1a0a")},
		"image/jpg": {
			fromHex("ffd8ffdb"),
			fromHex("ffd8ffe000104a4649460001"),
			fromHex("ffd8ffee"),
			fromHex("ffd8ffe18790457869660000"),
		},
		"image/jp2": {
			fromHex("ff4fff51"),
			fromHex("0000000c6a5020200d0a870a"),
		},
		"image/gif": {
			fromHex("474946383761"),
			fromHex("474946383961"),
		},
	}
)

// imageMimeType returns image/{jpg,gif,png,webp} if the byte slice is one
// of the supported image types or binary/octet-stream if not.
func imageMimeType(b []byte) string {
	for mime, prefixes := range knownImageTypes {
		for _, prefix := range prefixes {
			if bytes.HasPrefix(b, prefix) {
				return mime
			}
		}
	}
	return "binary/octet-stream"
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
	var max = 10
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

// chainHashMapHashHasher is a hasher function to use with xsync typed maps.
func chainHashMapHashHasher(seed maphash.Seed, k chainhash.Hash) uint64 {
	var h maphash.Hash
	h.SetSeed(seed)
	h.Write(k[:])
	return h.Sum64()
}

// isPayReqExpired returns true if the payreq has been expired.
func isPayReqExpired(payReq *zpay32.Invoice) bool {
	return payReq.Timestamp.Add(payReq.Expiry()).Before(time.Now())
}

// payReqStrAmount returns a string with a description of the amount of the
// given payreq.
func payReqStrAmount(payReq *zpay32.Invoice) string {
	if payReq.MilliAt == nil || *payReq.MilliAt == 0 {
		return "0 DCR"
	}

	if *payReq.MilliAt < 0 {
		return fmt.Sprintf("negative amount (%d atoms)", *payReq.MilliAt)
	}

	if *payReq.MilliAt < 1000 {
		return fmt.Sprintf("%d milliatoms", *payReq.MilliAt)
	}

	return dcrutil.Amount(*payReq.MilliAt / 1000).String()
}

// preferredCollator creates the preferred collator for strings.
func preferredCollator() *collate.Collator {
	// Get POSIX locale following POSIX rules for env var priority.
	posixLocale := os.Getenv("LC_ALL")
	if posixLocale == "" {
		posixLocale = os.Getenv("LC_COLLATE")
	}
	if posixLocale == "" {
		posixLocale = os.Getenv("LANG")
	}
	if posixLocale == "" {
		posixLocale = "en-US.UTF-8"
	}

	// Remove any charset or modifier components
	localeComponents := strings.Split(posixLocale, ".")
	baseLocale := localeComponents[0]

	// Remove any modifier (e.g., @euro)
	baseLocaleComponents := strings.Split(baseLocale, "@")
	baseLocale = baseLocaleComponents[0]

	// Replace underscores with hyphens to create an approximate BCP 47 language tag
	bcp47Tag := strings.Replace(baseLocale, "_", "-", -1)

	// Parse the BCP 47 value to ensure it's well-formed
	tag, err := language.Parse(bcp47Tag)
	if err != nil {
		tag = language.Und

		// Ignore C/POSIX languages as they are default.
		if bcp47Tag != "C" && bcp47Tag != "POSIX" {
			internalLog.Warnf("Unable to parse lang %q: %v", bcp47Tag, err)
		}
	} else {
		internalLog.Debugf("Using collator for lang %s: %s", bcp47Tag,
			tag)
	}

	// TODO: parametrize the options?
	opts := []collate.Option{collate.Loose}
	return collate.New(tag, opts...)
}
