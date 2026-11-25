package pan115

import (
	"context"
	"net/http"
	"path"
	"strconv"
	"strings"

	"litepan/internal/domain"
	"litepan/internal/driver"
)

func (d *Driver) CreateTemporaryPlaybackCopy(ctx context.Context, source, name string) (driver.TemporaryPlaybackCopy, error) {
	return d.CreateTrackedTemporaryPlaybackCopy(ctx, source, name, nil)
}
func (d *Driver) CreateTrackedTemporaryPlaybackCopy(ctx context.Context, source, name string, track func(driver.TemporaryPlaybackCopy) error) (driver.TemporaryPlaybackCopy, error) {
	var owned driver.TemporaryPlaybackCopy
	var entry fileEntry
	var err error
	if _, err = strconv.ParseInt(source, 10, 64); err != nil {
		entry, err = d.playbackPathInfo(ctx, source)
	} else {
		err = nil
	}
	if err != nil {
		return owned, err
	}
	sourceID := entry.entryID()
	if sourceID == "" {
		sourceID = source
	}
	if sourceID == "" {
		return owned, domain.Errorf(domain.CodeDriverError, "temporary copy source has no ID")
	}
	if err = d.apiCall(ctx, http.MethodPost, pathCopy, nil, urlValues(map[string]string{"file_id": sourceID, "pid": "0"}), nil); err != nil {
		return owned, err
	}
	if entry.entryName() == "" {
		if err = d.apiCall(ctx, http.MethodGet, pathFileInfo, urlValues(map[string]string{"file_id": sourceID}), nil, &entry); err != nil {
			return owned, err
		}
	}
	copied, err := d.playbackPathInfo(ctx, "/"+entry.entryName())
	if err != nil {
		return owned, err
	}
	d.rememberPickCode(copied)
	owned = driver.TemporaryPlaybackCopy{FileID: copied.entryID(), PickCode: copied.pickCode(), Name: copied.entryName()}
	if owned.FileID == "" || owned.FileID == sourceID {
		return driver.TemporaryPlaybackCopy{}, domain.Errorf(domain.CodeDriverError, "temporary copy ownership could not be verified")
	}
	if track != nil {
		if err := track(owned); err != nil {
			return owned, err
		}
	}
	if name == "" {
		name = path.Base(source)
	} else if prefix, _, ok := strings.Cut(name, "_"); ok {
		if _, err := strconv.ParseInt(prefix, 10, 64); err == nil {
			name = prefix + "_" + entry.entryName()
		}
	}
	err = d.RenameFile(ctx, owned.FileID, name)
	return owned, err
}
func (d *Driver) DeleteTemporaryPlaybackCopy(ctx context.Context, copy driver.TemporaryPlaybackCopy) error {
	if copy.FileID == "" {
		return domain.Errorf(domain.CodeValidation, "temporary copy has no owned ID")
	}
	return d.trashFiles(ctx, []string{copy.FileID})
}
