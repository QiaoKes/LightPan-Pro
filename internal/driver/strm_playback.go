package driver

import (
	"context"
	"litepan/internal/domain"
)

// STRMDownloader optionally applies a provider's STRM-specific scheduling.
// Playback invokes it only after cache lookup/coalescing, never for Emby.
type STRMDownloader interface {
	ResolveSTRMDownload(context.Context, DownloadRequest) (*domain.DownloadInfo, error)
}
