package resources

import "github.com/companyzero/bisonrelay/rpc"

// exactPathMatcher is a matcher that matches only the exact path.
func exactPathMatcher(path []string) routeMatcher {
	return func(req *rpc.RMFetchResource) bool {
		if req == nil {
			return false
		}
		if len(req.Path) != len(path) {
			return false
		}

		for i := 0; i < len(req.Path); i++ {
			if req.Path[i] != path[i] {
				return false
			}
		}

		return true
	}
}

// prefixPathMatcher is a matcher that matches all paths with the passed
// prefix.
func prefixPathMatcher(prefixPath []string) routeMatcher {
	return func(req *rpc.RMFetchResource) bool {
		if req == nil {
			return false
		}
		if len(req.Path) < len(prefixPath) {
			return false
		}
		for i := 0; i < len(prefixPath); i++ {
			if req.Path[i] != prefixPath[i] {
				return false
			}
		}
		return true
	}
}
