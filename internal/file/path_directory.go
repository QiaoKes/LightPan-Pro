package file

import (
	"context"
	"fmt"
	"litepan/internal/driver"
)

func (s *Service) ResolveDirectoryID(ctx context.Context, accountID int64, remote string) (string, error) {
	var id string
	err := s.exec.Run(ctx, accountID, func(d driver.Driver) error {
		resolver, ok := d.(driver.PathDirectoryResolver)
		if !ok {
			return fmt.Errorf("remote directory path resolution unsupported")
		}
		var err error
		id, err = resolver.ResolveDirectoryID(ctx, remote)
		if err == nil && id == "" {
			return fmt.Errorf("directory resolution returned empty identity")
		}
		return err
	})
	return id, err
}
