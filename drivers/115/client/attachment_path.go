package pan115client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

var attachmentOriginSequence atomic.Uint64
var attachmentOrigins = []string{"https://webapi.115.com", "http://web.api.115.com", "https://115cdn.com/webapi", "https://115vod.com/webapi", "https://f.115.com/api/proxy/115", "https://n.115.com/api/proxy/115"}

// AttachmentID preserves bundled get_id_to_path(refresh=True)'s ordinary path
// sequence, including the initial full-path directory lookup before parent lookup.
func (c *Client) AttachmentID(ctx context.Context, remote string) (string, error) {
	parts := splitMonitorPath(remote)
	if len(parts) == 0 {
		return "0", nil
	}
	getID := func(remote string) (string, error) {
		raw, err := c.Request(ctx, http.MethodGet, "https://webapi.115.com/files/getid", url.Values{"path": {remote}}, nil, nil)
		if err != nil {
			return "", err
		}
		if err = CheckResponse(raw); err != nil {
			return "", err
		}
		var resp struct {
			ID json.RawMessage `json:"id"`
		}
		if err = json.Unmarshal(raw, &resp); err != nil {
			return "", err
		}
		return strings.Trim(string(resp.ID), "\""), nil
	}
	stop := len(parts)
	for i, part := range parts {
		if strings.Contains(part, "/") {
			stop = i
			break
		}
	}
	cid := "0"
	for stop > 0 {
		id, err := getID(strings.Join(parts[:stop], "/"))
		if err != nil {
			return "", err
		}
		if id != "0" && id != "" && id != "null" {
			cid = id
			break
		}
		if stop == len(parts) {
			stop--
			continue
		}
		return "", fmt.Errorf("attachment directory missing")
	}
	if stop == len(parts) {
		return cid, nil
	}
	for _, name := range parts[stop : len(parts)-1] {
		id, err := c.findAttachmentChild(ctx, cid, name, true)
		if err != nil {
			return "", err
		}
		cid = id
	}
	return c.findAttachmentChild(ctx, cid, parts[len(parts)-1], false)
}
func (c *Client) findAttachmentChild(ctx context.Context, cid, name string, directoryOnly bool) (string, error) {
	params := url.Values{"aid": {"1"}, "record_open_time": {"1"}, "asc": {"1"}, "cid": {cid}, "cur": {"1"}, "count_folders": {"1"}, "fc_mix": {"1"}, "min_size": {"0"}, "max_size": {"0"}, "o": {"user_ptime"}, "offset": {"0"}, "show_dir": {"1"}, "suffix": {""}, "limit": {"1150"}, "custom_order": {"2"}}
	if directoryOnly {
		params.Set("nf", "1")
	}
	found := fmt.Errorf("attachment found")
	var matched string
	err := c.referenceFilePages(ctx, func() string {
		return attachmentOrigins[(attachmentOriginSequence.Add(1)-1)%uint64(len(attachmentOrigins))] + "/files"
	}, params, 1150, 500*time.Millisecond, func(raw []byte) error {
		var resp struct {
			Data []map[string]json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(raw, &resp); err != nil {
			return err
		}
		for _, item := range resp.Data {
			if directoryOnly && rawText(item, "fid") != "" {
				continue
			}
			if rawText(item, "n") == name {
				matched = rawText(item, "fid")
				if matched == "" {
					matched = rawText(item, "cid")
				}
				return found
			}
		}
		return nil
	})
	if err == found {
		return matched, nil
	}
	if err != nil {
		return "", err
	}
	return "", fmt.Errorf("attachment path missing")
}

func (c *Client) DownloadAttachmentPath(ctx context.Context, remote string) ([]byte, error) {
	return queueAttachment(ctx, func() ([]byte, error) { return c.downloadAttachmentPath(ctx, remote) })
}
func (c *Client) downloadAttachmentPath(ctx context.Context, remote string) ([]byte, error) {
	id, err := c.AttachmentID(ctx, remote)
	if err != nil {
		return nil, err
	}
	return c.downloadAttachment(ctx, id)
}
