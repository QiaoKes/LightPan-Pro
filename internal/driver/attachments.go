package driver

import "context"

// AttachmentProvider transfers non-media sidecars through their provider-specific credentials.
type AttachmentProvider interface {
	DownloadAttachment(context.Context, string) ([]byte, error)
}
type PathAttachmentProvider interface {
	DownloadAttachmentPath(context.Context, string) ([]byte, error)
}

type AttachmentPickcodeProvider interface {
	DownloadAttachmentPickcode(context.Context, string) ([]byte, error)
}
