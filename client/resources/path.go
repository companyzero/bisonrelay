package resources

import "strings"

// SplitPath splits the path into path elements according to the separator.
func SplitPath(path string) []string {
	return strings.Split(path, "/")
}
