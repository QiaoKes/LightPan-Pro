package driver

import (
	"context"

	"litepan/internal/domain"
)

// PathResolver preserves a provider's native path identity. A path-backed
// FileItem uses IDPath; resolving it must not fabricate a stable remote ID.
// Callers validate the relative path before invoking the provider.
type PathResolver interface {
	ResolvePath(ctx context.Context, rootID, relativePath string) (*domain.FileItem, error)
}
