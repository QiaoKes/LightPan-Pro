package pan115client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// LifeBehaviorDetail is the active monitor's Android behavior/detail wire call.
// Pagination, latest-per-file filtering, and cooldown belong to its caller.
func (c *Client) LifeBehaviorDetail(ctx context.Context, date string, offset, limit int) (json.RawMessage, error) {
	return c.Request(ctx, http.MethodGet, "https://proapi.115.com/android/behavior/detail", url.Values{"type": {""}, "date": {date}, "limit": {strconv.Itoa(limit)}, "offset": {strconv.Itoa(offset)}}, nil, nil)
}

// LifeBehaviorOnce ports tool/life.py iter_life_behavior_once's Android default.
// The monitor explicitly requests today's date, first batch1000, cooldown2.
func (c *Client) LifeBehaviorOnce(ctx context.Context, date string, since int64) ([]map[string]json.RawMessage, error) {
	offset := 0
	seen := map[string]bool{}
	var out []map[string]json.RawMessage
	for {
		started := time.Now()
		raw, err := c.LifeBehaviorDetail(ctx, date, offset, 1000)
		if err != nil {
			return nil, err
		}
		var envelope struct {
			State bool `json:"state"`
			Data  struct {
				List  []map[string]json.RawMessage `json:"list"`
				Count json.Number                  `json:"count"`
			} `json:"data"`
		}
		if err = CheckResponse(raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &envelope); err != nil {
			return nil, err
		}
		if !envelope.State {
			return nil, fmt.Errorf("life behavior request rejected")
		}
		for _, event := range envelope.Data.List {
			timestamp, _ := strconv.ParseInt(strings.Trim(string(event["update_time"]), "\""), 10, 64)
			if since > 0 && len(event["update_time"]) > 0 && timestamp < since {
				return out, nil
			}
			fid := strings.Trim(string(event["file_id"]), "\"")
			if seen[fid] {
				continue
			}
			seen[fid] = true
			kind, _ := strconv.Atoi(strings.Trim(string(event["type"]), "\""))
			switch kind {
			case 3, 4, 7, 8, 9, 10, 19:
				continue
			}
			out = append(out, event)
		}
		offset += len(envelope.Data.List)
		count, _ := strconv.Atoi(string(envelope.Data.Count))
		if len(envelope.Data.List) == 0 || offset >= count {
			return out, nil
		}
		delay := 2*time.Second - time.Since(started)
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			}
		}
	}
}

// FolderPath mirrors get_path(refresh=True)'s Android folder probe.
func (c *Client) FolderPath(ctx context.Context, id, uid string) (string, error) {
	if id == "" || id == "0" {
		return "", nil
	}
	raw, err := c.Request(ctx, http.MethodGet, "https://proapi.115.com/android/folder/update", url.Values{"p_id": {id}, "limit": {"1"}, "offset": {"0"}, "user_id": {uid}}, nil, nil)
	if err != nil {
		return "", err
	}
	var result struct {
		State bool `json:"state"`
		Data  struct {
			FileID   json.RawMessage `json:"file_id"`
			FileName string          `json:"file_name"`
			Paths    []struct {
				ID   json.RawMessage `json:"file_id"`
				Name string          `json:"file_name"`
			} `json:"paths"`
		} `json:"data"`
	}
	if err = CheckResponse(raw); err != nil {
		return "", err
	}
	if err = json.Unmarshal(raw, &result); err != nil {
		return "", err
	}
	if !result.State {
		return "", fmt.Errorf("folder path rejected")
	}
	if strings.Trim(string(result.Data.FileID), "\"") != id {
		return "", fmt.Errorf("folder path identity mismatch")
	}
	var parts []string
	for _, p := range result.Data.Paths {
		if strings.Trim(string(p.ID), "\"") != "0" {
			parts = append(parts, unescapeName(p.Name))
		}
	}
	parts = append(parts, unescapeName(result.Data.FileName))
	for i, part := range parts {
		parts[i] = escapeMonitorName(part)
	}
	nodes := map[string]cachedDirNode{}
	parent := "0"
	for _, entry := range result.Data.Paths {
		nodeID := strings.Trim(string(entry.ID), "\"")
		if nodeID == "0" {
			continue
		}
		nodes[nodeID] = cachedDirNode{unescapeName(entry.Name), parent}
		parent = nodeID
	}
	nodes[id] = cachedDirNode{unescapeName(result.Data.FileName), parent}
	cacheDirectoryNodes(uid, nodes)
	return "/" + strings.Join(parts, "/"), nil
}

func (c *Client) RecycleBin(ctx context.Context) (json.RawMessage, error) {
	return c.Request(ctx, http.MethodGet, "https://webapi.115.com/rb", url.Values{"aid": {"7"}, "cid": {"0"}, "limit": {"32"}, "offset": {"0"}}, nil, nil)
}
