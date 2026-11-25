package pan115client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

func (c *Client) referenceFileList(ctx context.Context, endpoint func() string, params url.Values, size int, cooldown time.Duration) ([]map[string]json.RawMessage, error) {
	var out []map[string]json.RawMessage
	err := c.referenceFilePages(ctx, endpoint, params, size, cooldown, func(raw []byte) error {
		var resp struct {
			Data []map[string]json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(raw, &resp); err != nil {
			return err
		}
		out = append(out, resp.Data...)
		return nil
	})
	return out, err
}

func (c *Client) referenceFilePages(ctx context.Context, endpoint func() string, params url.Values, size int, cooldown time.Duration, emit func([]byte) error) error {
	return referenceOffsetPages(ctx, size, cooldown, func(ctx context.Context, offset int) ([]byte, bool, error) {
		query := url.Values{}
		for key, values := range params {
			query[key] = append([]string(nil), values...)
		}
		query.Set("offset", strconv.Itoa(offset))
		raw, err := c.Request(ctx, http.MethodGet, endpoint(), query, nil, nil)
		if err != nil {
			return nil, false, err
		}
		if err = CheckResponse(raw); err != nil {
			return nil, false, err
		}
		var resp struct {
			Count json.Number                  `json:"count"`
			Data  []map[string]json.RawMessage `json:"data"`
		}
		if err = json.Unmarshal(raw, &resp); err != nil {
			return nil, false, err
		}
		count, _ := resp.Count.Int64()
		return raw, len(resp.Data) == 0 || int64(offset+len(resp.Data)) >= count, nil
	}, emit)
}
