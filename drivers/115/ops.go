package pan115

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"litepan/internal/domain"
	"litepan/internal/driver"
)

const maxPickCodeCacheEntries = 100_000

func normalizeIDs(fileIDs []string) []string {
	out := make([]string, 0, len(fileIDs))
	for _, id := range fileIDs {
		if id = strings.TrimSpace(id); id != "" {
			out = append(out, id)
		}
	}
	return out
}

func joinIDs(ids []string) string { return strings.Join(ids, ",") }

func (d *Driver) ResolveDownload(ctx context.Context, req driver.DownloadRequest) (*domain.DownloadInfo, error) {
	fileID := strings.TrimSpace(req.FileID)
	if fileID == "" {
		return nil, domain.Errorf(domain.CodeValidation, "file_id 不能为空")
	}

	var entry fileEntry
	pickCode := d.cachedPickCode(fileID)
	if strings.Contains(fileID, "/") {
		var err error
		// Reference download_url_path intentionally performs both lookups.
		for i := 0; i < 2; i++ {
			entry, err = d.playbackPathInfo(ctx, fileID)
			if err != nil {
				return nil, err
			}
		}
		pickCode = entry.pickCode()
		fileID = entry.entryID()
	}
	if pickCode == "" {
		query := urlValues(map[string]string{"file_id": fileID})
		if err := d.apiCall(ctx, http.MethodGet, pathFileInfo, query, nil, &entry); err != nil {
			return nil, err
		}
		d.rememberPickCode(entry)
		pickCode = entry.pickCode()
	}
	if pickCode == "" {
		name := entry.entryName()
		if name == "" {
			name = fileID
		}
		return nil, domain.Errorf(domain.CodeDriverError, "文件 %s 缺少 pick_code，无法获取下载链接", name)
	}

	ua := strings.TrimSpace(req.UA)
	if ua == "" {
		ua = "EmbyNginx"
	}
	d.mu.Lock()
	d.openSessionUA = ua
	d.mu.Unlock()
	form := urlValues(map[string]string{"pick_code": pickCode})
	var data json.RawMessage
	if err := d.apiCallWithHeaders(ctx, http.MethodPost, pathDownload, nil, form, map[string]string{
		"User-Agent": ua,
	}, &data); err != nil {
		return nil, err
	}
	downloadURL, size, name, err := parseDownloadURL(data, fileID)
	if err != nil {
		return nil, err
	}
	if size <= 0 {
		size = entry.entrySize()
	}
	if name == "" {
		name = entry.entryName()
	}
	headers := buildDownloadHeaders(ua)
	mode := domain.DownloadRedirect
	info := &domain.DownloadInfo{
		URL:         downloadURL,
		Mode:        mode,
		Size:        size,
		FileName:    name,
		Headers:     headers,
		ChunkSize:   downloadPartSize,
		Concurrency: downloadConcurrency,
		// 跨盘传输使用大分片，降低长时下载的请求频率和换链压力。
		TransferChunkSize: 128 << 20,
		// redirect 与 proxy 共用临时直链有效期。
		Expiration: downloadLinkTTL,
	}
	if strings.EqualFold(strings.TrimSpace(d.add.DownloadMode), "proxy") {
		info.Mode = domain.DownloadProxy
		info.ForceProxy = true
	}
	return info, nil
}

func buildDownloadHeaders(ua string) http.Header {
	if ua == "" {
		ua = "EmbyNginx"
	}
	h := http.Header{}
	h.Set("User-Agent", ua)
	h.Set("Accept", "*/*")
	h.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	h.Set("Accept-Encoding", "identity")
	h.Set("Referer", "https://115.com/")
	h.Set("Connection", "keep-alive")
	h.Set("Cache-Control", "no-cache")
	h.Set("Pragma", "no-cache")
	return h
}

func (d *Driver) rememberPickCode(entry fileEntry) {
	id := entry.entryID()
	pc := entry.pickCode()
	if id == "" || pc == "" {
		return
	}
	d.pickMu.Lock()
	if d.pickBy == nil {
		d.pickBy = make(map[string]string)
	}
	if _, exists := d.pickBy[id]; !exists && len(d.pickBy) >= maxPickCodeCacheEntries {
		clear(d.pickBy)
	}
	d.pickBy[id] = pc
	d.pickMu.Unlock()
}

func (d *Driver) cachedPickCode(fileID string) string {
	id := strings.TrimSpace(fileID)
	if id == "" {
		return ""
	}
	d.pickMu.RLock()
	pc := d.pickBy[id]
	d.pickMu.RUnlock()
	return pc
}

func (d *Driver) DeleteFiles(ctx context.Context, fileIDs []string) error {
	ids := normalizeIDs(fileIDs)
	if len(ids) == 0 {
		return nil
	}
	err := d.trashFiles(ctx, ids)
	if err == nil {
		d.pickMu.Lock()
		for _, id := range ids {
			delete(d.pickBy, id)
		}
		d.pickMu.Unlock()
	}
	return err
}

func (d *Driver) trashFiles(ctx context.Context, ids []string) error {
	form := urlValues(map[string]string{"file_ids": joinIDs(ids)})
	return d.apiCall(ctx, http.MethodPost, pathDelete, nil, form, nil)
}

func (d *Driver) MoveFiles(ctx context.Context, fileIDs []string, targetParentID, _ string) error {
	ids := normalizeIDs(fileIDs)
	if len(ids) == 0 {
		return nil
	}
	form := urlValues(map[string]string{
		"file_ids": joinIDs(ids),
		"to_cid":   d.normalizeParent(targetParentID),
	})
	return d.apiCall(ctx, http.MethodPost, pathMove, nil, form, nil)
}

func (d *Driver) CopyFiles(ctx context.Context, fileIDs []string, targetParentID string) error {
	ids := normalizeIDs(fileIDs)
	if len(ids) == 0 {
		return nil
	}
	form := urlValues(map[string]string{
		"file_id": joinIDs(ids),
		"pid":     d.normalizeParent(targetParentID),
		"nodupli": "0",
	})
	return d.apiCall(ctx, http.MethodPost, pathCopy, nil, form, nil)
}

func (d *Driver) RenameFile(ctx context.Context, fileID, newName string) error {
	id := strings.TrimSpace(fileID)
	name := strings.TrimSpace(newName)
	if id == "" {
		return domain.Errorf(domain.CodeValidation, "file_id 不能为空")
	}
	if name == "" {
		return domain.Errorf(domain.CodeValidation, "新名称不能为空")
	}
	form := urlValues(map[string]string{
		"file_id":   id,
		"file_name": name,
	})
	return d.apiCall(ctx, http.MethodPost, pathRename, nil, form, nil)
}

func (d *Driver) CreateFolder(ctx context.Context, parentID, name string) (*domain.FileItem, error) {
	folderName := strings.TrimSpace(name)
	if folderName == "" {
		return nil, domain.Errorf(domain.CodeValidation, "文件夹名称不能为空")
	}
	form := urlValues(map[string]string{
		"pid":       d.normalizeParent(parentID),
		"file_name": folderName,
	})
	var out mkdirResp
	if err := d.apiCall(ctx, http.MethodPost, pathMkdir, nil, form, &out); err != nil {
		return nil, err
	}
	folderID := strings.TrimSpace(out.Cid)
	if folderID == "" {
		folderID = strings.TrimSpace(out.FileID)
	}
	return &domain.FileItem{
		ID:     folderID,
		Name:   folderName,
		IsDir:  true,
		IDKind: domain.IDStable,
	}, nil
}

func (d *Driver) playbackPathInfo(ctx context.Context, path string) (fileEntry, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	var entry fileEntry
	for attempt := 0; attempt < 2; attempt++ {
		entry = fileEntry{}
		if err := d.apiCall(ctx, http.MethodPost, pathFileInfo, nil, urlValues(map[string]string{"path": path}), &entry); err != nil {
			return entry, err
		}
		if entry.entryID() != "" {
			return entry, nil
		}
		if attempt == 0 {
			if err := d.waitOpen(ctx, 30*time.Second); err != nil {
				return entry, err
			}
		}
	}
	return entry, domain.Errf(domain.CodeNotFound)
}
