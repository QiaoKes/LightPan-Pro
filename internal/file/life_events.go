package file

import (
	"context"
	"fmt"
	"litepan/internal/driver"
)

func (s *Service) ReadLifeEvents(ctx context.Context, id int64, req driver.LifeRequest) (driver.LifeBatch, error) {
	var batch driver.LifeBatch
	err := s.exec.Run(ctx, id, func(d driver.Driver) error {
		p, ok := d.(driver.LifeEventProvider)
		if !ok {
			return fmt.Errorf("life events unsupported")
		}
		var e error
		batch, e = p.ReadLifeEvents(ctx, req)
		return e
	})
	return batch, err
}
func (s *Service) DownloadAttachment(ctx context.Context, id int64, fileID string) ([]byte, error) {
	var body []byte
	err := s.exec.Run(ctx, id, func(d driver.Driver) error {
		p, ok := d.(driver.AttachmentProvider)
		if !ok {
			return fmt.Errorf("Cookie attachment transfer unsupported")
		}
		var e error
		body, e = p.DownloadAttachment(ctx, fileID)
		return e
	})
	return body, err
}
func (s *Service) DownloadAttachmentPath(ctx context.Context, id int64, remote string) ([]byte, error) {
	var body []byte
	err := s.exec.Run(ctx, id, func(d driver.Driver) error {
		p, ok := d.(driver.PathAttachmentProvider)
		if !ok {
			return fmt.Errorf("Cookie attachment path transfer unsupported")
		}
		var e error
		body, e = p.DownloadAttachmentPath(ctx, remote)
		return e
	})
	return body, err
}
func (s *Service) ExpandDirectoryEvents(ctx context.Context, id int64, fileID string) ([]driver.LifeEvent, error) {
	var events []driver.LifeEvent
	err := s.exec.Run(ctx, id, func(d driver.Driver) error {
		p, ok := d.(driver.DirectoryEventProvider)
		if !ok {
			return fmt.Errorf("directory event traversal unsupported")
		}
		var e error
		events, e = p.ExpandDirectoryEvents(ctx, fileID)
		return e
	})
	return events, err
}
func (s *Service) SyncSubtitleAttachments(ctx context.Context, id int64, remote string, skip func(string) bool, emit func(driver.SubtitleAttachment) error) error {
	return s.exec.Run(ctx, id, func(d driver.Driver) error {
		p, ok := d.(driver.SubtitleAttachmentProvider)
		if !ok {
			return fmt.Errorf("special subtitle enumeration unsupported")
		}
		return p.SyncSubtitleAttachments(ctx, remote, skip, emit)
	})
}

func (s *Service) DownloadAttachmentPickcode(ctx context.Context, id int64, pickcode string) ([]byte, error) {
	var body []byte
	err := s.exec.Run(ctx, id, func(d driver.Driver) error {
		p, ok := d.(driver.AttachmentPickcodeProvider)
		if !ok {
			return fmt.Errorf("Cookie attachment pickcode transfer unsupported")
		}
		var err error
		body, err = p.DownloadAttachmentPickcode(ctx, pickcode)
		return err
	})
	return body, err
}
