package resources

import (
	"os"
	"path/filepath"
	"regexp"

	"github.com/companyzero/bisonrelay/internal/mdembeds"
	"github.com/decred/slog"
)

// ProcessEmbeds processes "localfilename" directive in the md file and replaces
// it with the contents of the specified local file.
func ProcessEmbeds(s string, root string, log slog.Logger) string {
	if log == nil {
		log = slog.Disabled
	}

	return mdembeds.ReplaceEmbeds(s, func(args mdembeds.EmbeddedArgs) string {
		if args.LocalFilename == "" {
			return args.String()
		}
		localFname := args.LocalFilename
		if !filepath.IsAbs(localFname) {
			localFname = filepath.Join(root, localFname)
		}
		args.LocalFilename = ""
		embedData, err := os.ReadFile(localFname)
		if err != nil {
			log.Warnf("Unable to read embedded file %s: %v",
				localFname, err)
			return args.String()
		}
		args.Data = embedData
		return args.String()
	})
}

var endPostMarkerRegex = regexp.MustCompile(`(^|\n)--endofpost--[\n]?`)

// RemoveEndOfPostMarker removes all text after a standard --endofpost-- marker
// line.
func RemoveEndOfPostMarker(s string) string {
	loc := endPostMarkerRegex.FindStringIndex(s)
	if loc == nil {
		return s
	}
	return s[:loc[0]]
}
