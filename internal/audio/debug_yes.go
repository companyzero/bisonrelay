//go:build audiodebug

// This file is conditionally compiled when the build tag 'audiodebug' is set
// to include additional debug and trace statements throughout the code.
//
// These usally add overhead, so for production builds they are removed during
// compilation.

package audio

const addDebugTrace = true
