package driver

import "context"

// PathDirectoryResolver resolves a verified absolute remote directory path.
type PathDirectoryResolver interface {
	ResolveDirectoryID(context.Context, string) (string, error)
}
