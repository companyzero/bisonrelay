package resources

import (
	"context"
	"os"
	"path/filepath"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/strescape"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/decred/slog"
)

// StaticResource is a resource that always returns the same data.
type StaticResource struct {
	Data []byte
	Meta map[string]string
}

// Fulfill is part of the Provider interface.
func (sr *StaticResource) Fulfill(ctx context.Context, uid clientintf.UserID,
	req *rpc.RMFetchResource) (*rpc.RMFetchResourceReply, error) {

	return &rpc.RMFetchResourceReply{
		Status: rpc.ResourceStatusOk,
		Data:   sr.Data,
		Meta:   sr.Meta,
	}, nil
}

// FilesystemResource is a resource that returns data from a root dir in the
// filesystem.
type FilesystemResource struct {
	root string
	log  slog.Logger
}

func NewFilesystemResource(root string, log slog.Logger) *FilesystemResource {
	if log == nil {
		log = slog.Disabled
	}
	return &FilesystemResource{
		root: root,
		log:  log,
	}
}

// Fulfill is part of the Provider interface.
func (fr *FilesystemResource) Fulfill(ctx context.Context, uid clientintf.UserID,
	req *rpc.RMFetchResource) (*rpc.RMFetchResourceReply, error) {

	escapedPath := make([]string, 0, 1+len(req.Path))
	escapedPath = append(escapedPath, fr.root)
	for _, e := range req.Path {
		escapedPath = append(escapedPath, strescape.PathElement(e))
	}
	filename := filepath.Join(escapedPath...)

	// TODO: support chunked response.
	data, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		return &rpc.RMFetchResourceReply{
			Status: rpc.ResourceStatusNotFound,
		}, nil
	} else if err != nil {
		return nil, err
	}

	// Process embeds.
	if filepath.Ext(filename) == ".md" {
		data = []byte(ProcessEmbeds(string(data), fr.root, fr.log))
	}

	return &rpc.RMFetchResourceReply{
		Data:   data,
		Status: rpc.ResourceStatusOk,
	}, nil
}
