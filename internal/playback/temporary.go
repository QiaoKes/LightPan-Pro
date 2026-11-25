package playback

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"litepan/internal/domain"
	"litepan/internal/driver"
	"path"
	"time"
)

func (s *Service) SetTemporaryPlaybackRepository(repo domain.TemporaryPlaybackRepository) {
	s.temporaryLedger = repo
}

// RunTemporaryCleanup recovers pending ownership on startup and retains failed
// deletions for subsequent attempts. A shutdown never discards pending objects.
func (s *Service) RunTemporaryCleanup(ctx context.Context) {
	if s.temporaryLedger == nil {
		return
	}
	timer := time.NewTicker(time.Second)
	defer timer.Stop()
	for {
		s.cleanupTemporary(ctx)
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}
}
func (s *Service) cleanupTemporary(ctx context.Context) {
	records, err := s.temporaryLedger.Pending(ctx)
	if err != nil {
		return
	}
	for _, record := range records {
		_, active := s.temporaryActive.Load(record.ID)
		if active || time.Now().Before(record.Due) || record.FileID == "" {
			continue
		}
		err = s.exec.Run(ctx, record.AccountID, func(d driver.Driver) error {
			copier, ok := d.(driver.TemporaryPlaybackCopier)
			if !ok {
				return domain.Errf(domain.CodeNotImplement)
			}
			return copier.DeleteTemporaryPlaybackCopy(ctx, driver.TemporaryPlaybackCopy{FileID: record.FileID, PickCode: record.PickCode, Name: record.Name})
		})
		if err == nil {
			_ = s.temporaryLedger.Delete(ctx, record.ID)
		} else {
			record.LastError = err.Error()
			record.Due = time.Now().Add(time.Minute)
			_ = s.temporaryLedger.Save(ctx, record)
		}
	}
}

func (s *Service) ResolveTemporaryPlayback(ctx context.Context, accountID int64, source, ua string) (Resolved, error) {
	var result Resolved
	if s.temporaryLedger == nil {
		return result, domain.Errorf(domain.CodeDriverError, "temporary playback ownership store unavailable")
	}
	err := s.exec.Run(ctx, accountID, func(d driver.Driver) error {
		copier, ok := d.(driver.TemporaryPlaybackCopier)
		if !ok {
			return domain.Errf(domain.CodeNotImplement)
		}

		record := domain.TemporaryPlaybackRecord{ID: uuid.NewString(), AccountID: accountID, Due: time.Now()}
		s.temporaryActive.Store(record.ID, true)
		defer s.temporaryActive.Delete(record.ID)
		persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
		defer cancel()
		track := func(copy driver.TemporaryPlaybackCopy) error {
			record.FileID = copy.FileID
			record.PickCode = copy.PickCode
			record.Name = copy.Name
			return s.temporaryLedger.Save(persistCtx, record)
		}
		var copy driver.TemporaryPlaybackCopy
		var createErr error
		name := fmt.Sprintf("%d_%s", time.Now().Unix(), path.Base(source))
		if tracked, ok := d.(driver.TrackedTemporaryPlaybackCopier); ok {
			copy, createErr = tracked.CreateTrackedTemporaryPlaybackCopy(ctx, source, name, track)
		} else {
			copy, createErr = copier.CreateTemporaryPlaybackCopy(ctx, source, name)
		}
		if copy.FileID == "" {
			if createErr != nil {
				return createErr
			}
			return domain.Errorf(domain.CodeDriverError, "temporary copy has no ID")
		}
		if err := track(copy); err != nil {
			_ = copier.DeleteTemporaryPlaybackCopy(persistCtx, copy)
			return err
		}

		if createErr != nil {
			return createErr
		}
		dl, ok := d.(driver.Downloader)
		if !ok {
			return domain.Errf(domain.CodeNotImplement)
		}
		link, err := dl.ResolveDownload(ctx, driver.DownloadRequest{FileID: copy.FileID, UA: ua})
		// Reference delay starts after the link attempt, never before slow getlink.
		record.Due = time.Now().Add(10 * time.Second)
		if err != nil {
			record.LastError = err.Error()
		}
		if saveErr := s.temporaryLedger.Save(persistCtx, record); saveErr != nil {
			return saveErr
		}
		if err != nil {
			return err
		}
		result = Resolved{File: domain.FileItem{ID: copy.FileID, Name: link.FileName, Size: link.Size}, Link: *link, Mode: link.Mode}
		return nil
	})
	return result, err
}

func (s *Service) SupportsTemporaryPlayback(ctx context.Context, accountID int64) bool {
	supported := false
	_ = s.exec.Run(ctx, accountID, func(d driver.Driver) error { _, supported = d.(driver.TemporaryPlaybackCopier); return nil })
	return supported
}
