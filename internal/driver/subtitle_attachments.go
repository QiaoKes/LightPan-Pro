package driver

import "context"

type SubtitleAttachment struct {
	Path string
	Data []byte
}
type SubtitleAttachmentProvider interface {
	SyncSubtitleAttachments(context.Context, string, func(string) bool, func(SubtitleAttachment) error) error
}
