package pan115client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ExportTree preserves the reference single-page root listing and export order.
// Complete=false explicitly prevents deletion after a partial root listing.
func (c *Client) ExportTree(ctx context.Context, root string) (paths []string, complete bool, err error) {
	return c.exportTree(ctx, root, c.CookieDownload)
}
func (c *Client) exportTree(ctx context.Context, root string, resolve func(context.Context, string, string) (*Download, error)) (paths []string, complete bool, err error) {
	return c.exportTreeWithTiming(ctx, root, resolve, time.Now, func(ctx context.Context) error {
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return nil
		}
	})
}

func (c *Client) exportTreeWithTiming(ctx context.Context, root string, resolve func(context.Context, string, string) (*Download, error), now func() time.Time, wait func(context.Context) error) (paths []string, complete bool, err error) {
	var exportID, fileID string
	defer func() {
		if err != nil && (exportID != "" || fileID != "") {
			err = &TreeExportError{ExportID: exportID, FileID: fileID, Err: err}
		}
	}()
	root = strings.TrimRight(root, "/")
	ids := ""
	complete = true
	if root == "" {
		raw, e := c.treeRequest(ctx, "GET", "https://webapi.115.com/files", url.Values{"aid": {"1"}, "count_folders": {"1"}, "record_open_time": {"1"}, "cid": {"0"}, "limit": {"32"}, "offset": {"0"}, "show_dir": {"1"}}, nil, nil)
		if e != nil {
			return nil, false, e
		}
		var r struct {
			Data  []map[string]json.RawMessage `json:"data"`
			Count int                          `json:"count"`
		}
		if e = json.Unmarshal(raw, &r); e != nil {
			return nil, false, e
		}
		if r.Data == nil {
			return nil, false, fmt.Errorf("tree root listing missing data")
		}
		var values []string
		for _, v := range r.Data {
			id, ok := v["fid"]
			if !ok {
				id = v["cid"]
			}
			s := treeScalar(id)
			if s == "" {
				return nil, false, fmt.Errorf("tree root entry missing ID")
			}
			values = append(values, s)
		}
		ids = strings.Join(values, ",")
		if r.Count > len(r.Data) {
			complete = false
		}
	} else {
		raw, e := c.treeRequest(ctx, "GET", "https://webapi.115.com/files/getid", url.Values{"path": {root}}, nil, nil)
		if e != nil {
			return nil, false, e
		}
		var r struct {
			ID json.RawMessage `json:"id"`
		}
		if e = json.Unmarshal(raw, &r); e != nil {
			return nil, false, e
		}
		ids = treeScalar(r.ID)
		if ids == "" || ids == "0" {
			return nil, false, fmt.Errorf("tree directory not found")
		}
	}
	raw, e := c.treeRequest(ctx, "POST", "https://webapi.115.com/files/export_dir", nil, url.Values{"file_ids": {ids}, "target": {"U_1_0"}}, nil)
	if e != nil {
		return nil, false, e
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if e = json.Unmarshal(raw, &envelope); e != nil {
		return nil, false, e
	}
	exportID = treeScalar(envelope.Data)
	if exportID == "" || exportID == "0" {
		return nil, false, fmt.Errorf("missing tree export ID")
	}
	start := now()
	for {
		if now().Sub(start) > 3000*time.Second {
			return nil, false, fmt.Errorf("tree export timeout")
		}
		raw, e = c.treeRequest(ctx, "GET", "https://webapi.115.com/files/export_dir", url.Values{"export_id": {exportID}}, nil, nil)
		if e != nil {
			return nil, false, e
		}
		if e = json.Unmarshal(raw, &envelope); e != nil {
			return nil, false, e
		}
		if len(envelope.Data) == 0 || string(envelope.Data) == "0" || string(envelope.Data) == "null" || string(envelope.Data) == "false" || string(envelope.Data) == "[]" || string(envelope.Data) == "{}" || string(envelope.Data) == "\"\"" {
			if e = wait(ctx); e != nil {
				return nil, false, e
			}
			continue
		}
		var status struct {
			PickCode string          `json:"pick_code"`
			FileID   json.RawMessage `json:"file_id"`
		}
		if e = json.Unmarshal(envelope.Data, &status); e != nil {
			return nil, false, e
		}
		if status.PickCode == "" || treeScalar(status.FileID) == "" {
			return nil, false, fmt.Errorf("malformed tree export status")
		}
		fileID = treeScalar(status.FileID)
		download, e := resolve(ctx, status.PickCode, "")
		if e != nil {
			return nil, false, e
		}
		var body []byte
		for attempt := 0; attempt < 6; attempt++ {
			var req *http.Request
			req, e = http.NewRequestWithContext(ctx, "GET", download.URL, nil)
			if e != nil {
				return nil, false, e
			}
			req.Header.Set("User-Agent", "")
			var resp *http.Response
			resp, e = c.http.Do(req)
			if e == nil {
				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					body, e = io.ReadAll(resp.Body)
				} else {
					e = fmt.Errorf("tree download HTTP %d", resp.StatusCode)
				}
				resp.Body.Close()
			}
			if e == nil {
				break
			}
			if ctx.Err() != nil {
				return nil, false, ctx.Err()
			}
		}
		if e != nil {
			return nil, false, e
		}
		// The owned export is retired before parsing, exactly as in the reference.
		raw, e = c.treeRequest(ctx, "POST", "https://webapi.115.com/rb/delete", nil, url.Values{"fid": {treeScalar(status.FileID)}}, nil)
		if e != nil {
			return nil, false, e
		}
		var deleted struct {
			State bool `json:"state"`
		}
		if e = json.Unmarshal(raw, &deleted); e != nil {
			return nil, false, e
		}
		if !deleted.State {
			return nil, false, fmt.Errorf("tree export cleanup rejected")
		}
		paths, e = ParseDirectoryTree(body, root)
		if e != nil {
			return nil, false, e
		}
		return paths, complete, nil
	}
}
func treeScalar(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var n json.Number
	if json.Unmarshal(raw, &n) == nil {
		return string(n)
	}
	return ""
}

func (c *Client) treeRequest(ctx context.Context, method, endpoint string, query, form url.Values, headers http.Header) (json.RawMessage, error) {
	raw, err := c.Request(ctx, method, endpoint, query, form, headers)
	if err != nil {
		return nil, err
	}
	if err = CheckResponse(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// TreeExportError records only server-owned operation IDs, never credentials or
// signed URLs. It preserves error classification through Unwrap.
type TreeExportError struct {
	ExportID, FileID string
	Err              error
}

func (e *TreeExportError) Error() string {
	return fmt.Sprintf("115 tree export %s temporary file %s: %v", e.ExportID, e.FileID, e.Err)
}
func (e *TreeExportError) Unwrap() error { return e.Err }
