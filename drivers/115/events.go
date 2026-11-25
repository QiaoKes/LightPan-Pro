package pan115

import (
	"context"
	"encoding/json"
	"fmt"
	pan115client "litepan/drivers/115/client"
	"litepan/internal/domain"
	"litepan/internal/driver"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

func (d *Driver) ReadLifeEvents(ctx context.Context, req driver.LifeRequest) (batch driver.LifeBatch, err error) {
	startedUnix := time.Now().Unix()
	handleErr := d.cookieErrorHandler()
	defer func() { err = handleErr(ctx, err) }()
	if err := d.requireCapability(domain.AuthCapabilityCookieAPI); err != nil {
		return batch, err
	}
	d.mu.Lock()
	cookie, uid, openUID := d.cookie, d.cookieHealth.VerifiedUID, d.openHealth.VerifiedUID
	d.mu.Unlock()
	if uid == "" || uid != openUID {
		return batch, fmt.Errorf("life events require matching verified identity")
	}
	c := pan115client.New(d.client, cookie)
	now := time.Now()
	firstDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if req.SinceUnix > 0 {
		checkpoint := time.Unix(req.SinceUnix, 0).In(now.Location())
		firstDay = time.Date(checkpoint.Year(), checkpoint.Month(), checkpoint.Day(), 0, 0, 0, 0, now.Location())
	}
	var records []map[string]json.RawMessage
	for day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()); !day.Before(firstDay); day = day.AddDate(0, 0, -1) {
		items, e := c.LifeBehaviorOnce(ctx, day.Format("2006-01-02"), req.SinceUnix)
		if e != nil {
			return batch, e
		}
		records = append(records, items...)
	}
	cache := map[string]string{}
	folder := func(id string) (string, error) {
		if v, ok := cache[id]; ok {
			return v, nil
		}
		v, e := c.FolderPath(ctx, id, uid)
		if e == nil {
			cache[id] = v
		}
		return v, e
	}
	text := func(m map[string]json.RawMessage, key string) string {
		raw := m[key]
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return s
		}
		return string(raw)
	}
	var recycle []map[string]json.RawMessage
	var recycleAt int64
	for _, r := range records {
		kind, _ := strconv.Atoi(text(r, "type"))
		ts, _ := strconv.ParseInt(text(r, "update_time"), 10, 64)
		event := driver.LifeEvent{ID: text(r, "id"), FileID: text(r, "file_id"), ParentID: text(r, "parent_id"), UpdateUnix: ts, Verified: true}
		event.IsDir = !lifeJSONTruthy(r["file_size"])
		name := text(r, "file_name")
		if strings.Contains(name, "/") || name == ".." || name == "." {
			return batch, fmt.Errorf("invalid event filename")
		}
		switch kind {
		case 1, 2, 5, 6, 14, 18:
			event.Kind = "create"
			if event.IsDir {
				event.Path, err = folder(event.FileID)
			} else {
				var parent string
				parent, err = folder(event.ParentID)
				event.Path = path.Join("/", parent, name)
			}
		case 20, 24:
			event.Kind = "rename"
			oldName := text(r, "old_file_name")
			if oldName == "" || oldName == "." || oldName == ".." || strings.Contains(oldName, "/") {
				return batch, fmt.Errorf("invalid old event filename")
			}
			var parent string
			parent, err = folder(event.ParentID)
			event.Path = path.Join("/", parent, name)
			event.OldPath = path.Join("/", parent, text(r, "old_file_name"))
		case 22:
			event.Kind = "delete"
			event.Verified = false
			if len(recycle) == 0 || recycleAt < ts {
				raw, e := c.RecycleBin(ctx)
				if e != nil {
					return batch, e
				}
				var resp struct {
					State bool                         `json:"state"`
					Data  []map[string]json.RawMessage `json:"data"`
				}
				if e = pan115client.CheckResponse(raw); e != nil {
					return batch, e
				}
				if e = json.Unmarshal(raw, &resp); e != nil {
					return batch, e
				}
				if !resp.State {
					return batch, fmt.Errorf("recycle verification rejected")
				}
				recycle = resp.Data
				recycleAt = time.Now().Unix()
			}
			for _, item := range recycle {
				deleted, _ := strconv.ParseInt(text(item, "dtime"), 10, 64)
				if text(item, "file_name") == name && deleted-ts <= 3 {
					var parent string
					parent, err = folder(text(item, "cid"))
					event.Path = path.Join("/", parent, name)
					event.Verified = err == nil
					break
				}
			}
			if !event.Verified {
				continue
			}
		default:
			continue
		}
		if err != nil {
			return batch, err
		}
		if event.Kind == "create" && strings.HasPrefix(event.Path, "/subtitle-") {
			if event.IsDir {
				continue
			}
			if !strings.HasPrefix(path.Dir(event.Path), "/subtitle-") {
				batch.Events = append(batch.Events, event)
				continue
			}
			switch path.Ext(event.Path) {
			case ".ass", ".srt", ".ssa", ".sub", ".smi", ".sup":
				continue
			}
		}
		batch.Events = append(batch.Events, event)
	}
	batch.ThroughUnix = startedUnix
	batch.Complete = true
	return batch, nil
}

var _ driver.LifeEventProvider = (*Driver)(nil)

func (d *Driver) DownloadAttachment(ctx context.Context, id string) (body []byte, err error) {
	handleErr := d.cookieErrorHandler()
	defer func() { err = handleErr(ctx, err) }()
	if err := d.requireCapability(domain.AuthCapabilityCookieAPI); err != nil {
		return nil, err
	}
	d.mu.Lock()
	cookie := d.cookie
	d.mu.Unlock()
	return pan115client.New(d.client, cookie).DownloadAttachment(ctx, id)
}
func (d *Driver) DownloadAttachmentPath(ctx context.Context, remote string) (body []byte, err error) {
	handleErr := d.cookieErrorHandler()
	defer func() { err = handleErr(ctx, err) }()
	if err = d.requireCapability(domain.AuthCapabilityCookieAPI); err != nil {
		return nil, err
	}
	d.mu.Lock()
	cookie := d.cookie
	d.mu.Unlock()
	return pan115client.New(d.client, cookie).DownloadAttachmentPath(ctx, remote)
}
func (d *Driver) ExpandDirectoryEvents(ctx context.Context, id string) (events []driver.LifeEvent, err error) {
	handleErr := d.cookieErrorHandler()
	defer func() { err = handleErr(ctx, err) }()
	if err = d.requireCapability(domain.AuthCapabilityCookieAPI); err != nil {
		return nil, err
	}
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
	}
	d.mu.Lock()
	cookie, uid := d.cookie, d.cookieHealth.VerifiedUID
	d.mu.Unlock()
	nodes, err := pan115client.New(d.client, cookie).DirectoryEventFiles(ctx, id, uid)
	if err != nil {
		return nil, err
	}
	for _, node := range nodes {
		events = append(events, driver.LifeEvent{FileID: node.ID, ParentID: node.ParentID, Path: node.Path, Pickcode: node.Pickcode, Kind: "create", Verified: true})
	}
	return events, nil
}
func (d *Driver) ResolveDirectoryID(ctx context.Context, remote string) (id string, err error) {
	handleErr := d.cookieErrorHandler()
	defer func() { err = handleErr(ctx, err) }()
	if err = d.requireCapability(domain.AuthCapabilityCookieAPI); err != nil {
		return "", err
	}
	if remote == "" || remote == "/" {
		return "0", nil
	}
	d.mu.Lock()
	cookie := d.cookie
	d.mu.Unlock()
	raw, err := pan115client.New(d.client, cookie).Request(ctx, http.MethodGet, "https://webapi.115.com/files/getid", url.Values{"path": {strings.Trim(remote, "/")}}, nil, nil)
	if err != nil {
		return "", err
	}
	if err = pan115client.CheckResponse(raw); err != nil {
		return "", err
	}
	var resp struct {
		ID json.RawMessage `json:"id"`
	}
	if err = json.Unmarshal(raw, &resp); err != nil {
		return "", err
	}
	id = strings.Trim(string(resp.ID), "\"")
	if id == "" || id == "0" || id == "null" {
		return "", fmt.Errorf("directory path missing")
	}
	return id, nil
}
func (d *Driver) SyncSubtitleAttachments(ctx context.Context, remote string, skip func(string) bool, emit func(driver.SubtitleAttachment) error) (err error) {
	handleErr := d.cookieErrorHandler()
	defer func() { err = handleErr(ctx, err) }()
	if err = d.requireCapability(domain.AuthCapabilityCookieAPI); err != nil {
		return err
	}
	id, err := d.ResolveDirectoryID(ctx, remote)
	if err != nil {
		return err
	}
	d.mu.Lock()
	cookie, uid := d.cookie, d.cookieHealth.VerifiedUID
	d.mu.Unlock()
	return pan115client.New(d.client, cookie).SubtitleAttachments(ctx, id, uid, remote, skip, func(p string, b []byte) error { return emit(driver.SubtitleAttachment{Path: p, Data: b}) })
}

// lifeJSONTruthy retains Python truthiness rather than normalizing numeric strings.
func lifeJSONTruthy(raw json.RawMessage) bool {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return false
	}
	switch v := value.(type) {
	case nil:
		return false
	case bool:
		return v
	case float64:
		return v != 0
	case string:
		return v != ""
	case []any:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	}
	return false
}

func (d *Driver) DownloadAttachmentPickcode(ctx context.Context, pickcode string) (body []byte, err error) {
	handleErr := d.cookieErrorHandler()
	defer func() { err = handleErr(ctx, err) }()
	if err = d.requireCapability(domain.AuthCapabilityCookieAPI); err != nil {
		return nil, err
	}
	d.mu.Lock()
	c := pan115client.New(d.client, d.cookie)
	d.mu.Unlock()
	return c.DownloadAttachmentPickcode(ctx, pickcode)
}
