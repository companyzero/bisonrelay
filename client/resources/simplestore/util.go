package simplestore

import "golang.org/x/exp/slices"

func pathEquals(path []string, target ...string) bool {
	return slices.Equal(path, target)
}

func pathHasPrefix(path []string, target ...string) bool {
	if len(path) < len(target) {
		return false
	}

	return slices.Equal(path[:len(target)], target)
}
