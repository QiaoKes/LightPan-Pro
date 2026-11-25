package file

import (
	"context"
	"fmt"
	"litepan/internal/driver"
)

func (s *Service) SupportsTreeSnapshot(ctx context.Context, id int64) (bool, error) {
	var ok bool
	err := s.exec.Run(ctx, id, func(d driver.Driver) error { _, ok = d.(driver.TreeSnapshotProvider); return nil })
	return ok, err
}
func (s *Service) ExportTree(ctx context.Context, id int64, req driver.TreeRequest, emit func(driver.TreeEntry) error) (driver.TreeSnapshot, error) {
	var out driver.TreeSnapshot
	err := s.exec.Run(ctx, id, func(d driver.Driver) error {
		p, ok := d.(driver.TreeSnapshotProvider)
		if !ok {
			return fmt.Errorf("tree export unsupported")
		}
		var e error
		out, e = p.ExportTree(ctx, req, emit)
		return e
	})
	return out, err
}
