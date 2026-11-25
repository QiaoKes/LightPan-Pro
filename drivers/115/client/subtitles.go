package pan115client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// VideoSubtitles preserves the Cookie SDK fs_video_subtitle response envelope.
func (c *Client) VideoSubtitles(ctx context.Context, pickcode string) (json.RawMessage, error) {
	return c.Request(ctx, http.MethodGet, "https://webapi.115.com/movies/subtitle", url.Values{"pickcode": {pickcode}}, nil, nil)
}

// DownloadAttachment follows monitor download_files: skim, Cookie downurl,
// sleep1, GET(empty UA), sleep0.5. It does not use Open media playback.
func (c *Client) DownloadAttachment(ctx context.Context, id string) ([]byte, error) {
	return queueAttachment(ctx, func() ([]byte, error) { return c.downloadAttachment(ctx, id) })
}
func (c *Client) downloadAttachment(ctx context.Context, id string) ([]byte, error) {
	raw, err := c.Request(ctx, http.MethodGet, "https://webapi.115.com/files/file", url.Values{"file_id": {id}}, nil, nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		State bool `json:"state"`
		Data  []struct {
			Pickcode string `json:"pick_code"`
		} `json:"data"`
	}
	if err = CheckResponse(raw); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	if !resp.State || len(resp.Data) == 0 || resp.Data[0].Pickcode == "" {
		return nil, fmt.Errorf("attachment skim missing pickcode")
	}
	return c.downloadAttachmentPickcode(ctx, resp.Data[0].Pickcode)
}

func (c *Client) DownloadAttachmentPickcode(ctx context.Context, pickcode string) ([]byte, error) {
	return queueAttachment(ctx, func() ([]byte, error) { return c.downloadAttachmentPickcode(ctx, pickcode) })
}
func (c *Client) downloadAttachmentPickcode(ctx context.Context, pickcode string) ([]byte, error) {
	download, err := c.CookieDownload(ctx, pickcode, "")
	if err != nil {
		return nil, err
	}
	pause := func(d time.Duration) error {
		timer := time.NewTimer(d)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return nil
		}
	}
	if err = pause(time.Second); err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, download.URL, nil)
	if err != nil {
		return nil, err
	}
	request.Header = download.Headers.Clone()
	request.Header.Set("User-Agent", "")
	response, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if err = pause(500 * time.Millisecond); err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("attachment HTTP %d", response.StatusCode)
	}
	return body, nil
}
