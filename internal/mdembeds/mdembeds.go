package mdembeds

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/zkidentity"
)

type EmbeddedArgs struct {
	// embedded file
	Name string
	Data []byte
	Alt  string
	Typ  string

	// shared link
	Download zkidentity.ShortID
	Filename string
	Size     uint64
	Cost     uint64

	// May be set externally, not on the link.
	Uid *clientintf.UserID
}

func (args EmbeddedArgs) String() string {
	var parts []string
	if args.Name != "" {
		parts = append(parts, "part="+args.Name)
	}
	if args.Alt != "" {
		parts = append(parts, "alt="+args.Alt)
	}
	if args.Typ != "" {
		parts = append(parts, "type="+args.Typ)
	}
	if !args.Download.IsEmpty() {
		parts = append(parts, "download="+args.Download.String())
	}
	if args.Filename != "" {
		parts = append(parts, "filename="+args.Filename)
	}
	if args.Size > 0 {
		parts = append(parts, "size="+strconv.FormatUint(args.Size, 10))
	}
	if args.Cost > 0 {
		parts = append(parts, "cost="+strconv.FormatUint(args.Cost, 10))
	}
	if args.Data != nil {
		parts = append(parts, "data="+base64.StdEncoding.EncodeToString(args.Data))
	}

	return "--embed[" + strings.Join(parts, ",") + "]--"
}

var embedRegexp = regexp.MustCompile(`--embed\[.*?\]--`)

// FindAllStringIndex returns a slice with start and end positions for all
// embeds within the specified string.
func FindAllStringIndex(s string) [][]int {
	return embedRegexp.FindAllStringIndex(s, -1)
}

// ParseEmbedArgs parses the given raw embed string, which should be --[]--,
// with the embed conted between brackets.
func ParseEmbedArgs(rawEmbedStr string) EmbeddedArgs {
	// Copy everything between the [] (the raw argument list).
	start, end := strings.Index(rawEmbedStr, "["), strings.LastIndex(rawEmbedStr, "]")
	rawArgs := rawEmbedStr[start+1 : end]

	// Split args by comma.
	splitArgs := strings.Split(rawArgs, ",")

	// Decode args.
	var args EmbeddedArgs
	for _, a := range splitArgs {
		// Split by "="
		kv := strings.SplitN(a, "=", 2)
		if len(kv) != 2 {
			continue
		}
		k, v := kv[0], kv[1]
		switch k {
		case "name":
			args.Name = v
		case "type":
			args.Typ = v
		case "data":
			decoded, err := base64.StdEncoding.DecodeString(v)
			if err != nil {
				decoded = []byte(fmt.Sprintf("[err decoding data: %v]", err))
			}
			args.Data = decoded
		case "alt":
			decoded, err := url.PathUnescape(v)
			if err != nil {
				decoded = fmt.Sprintf("[err processing alt: %v]", err)
			}
			args.Alt = decoded
		case "download":
			// Ignore the error and leave download empty.
			args.Download.FromString(v)
		case "filename":
			args.Filename = v
		case "size":
			args.Size, _ = strconv.ParseUint(v, 10, 64)
		case "cost":
			args.Cost, _ = strconv.ParseUint(v, 10, 64)
		}
	}

	return args
}

// ReplaceEmbeds replaces all the embeds tags of the given text with the result
// of the calling function.
func ReplaceEmbeds(src string, replF func(args EmbeddedArgs) string) string {
	return embedRegexp.ReplaceAllStringFunc(src, func(repl string) string {
		// Decode args.
		args := ParseEmbedArgs(repl)
		return replF(args)
	})
}
