package pan115client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"
)

// subtitleHierarchy loads descendant directory names with the reference OS API.
func (c *Client) subtitleHierarchy(ctx context.Context, id, uid, base string) (map[string]string, error) {
	paths := map[string]string{"0": ""}
	nodes := map[string]downloadFolder{}
	loadDirs := func(pc string) error {
		items, err := c.downloadNodes(ctx, pc, false)
		if err != nil {
			return err
		}
		rawNodes := map[string]cachedDirNode{}
		for _, item := range items {
			fid := rawText(item, "fid")
			nodes[fid] = downloadFolder{fid, rawText(item, "pid"), rawText(item, "fn")}
			rawNodes[fid] = cachedDirNode{rawText(item, "fn"), rawText(item, "pid")}
		}
		cacheDirectoryNodes(uid, rawNodes)
		return nil
	}
	if id != "0" {
		paths[id] = strings.TrimRight(base, "/")
		pc, err := c.directoryPickcode(ctx, id, uid)
		if err != nil {
			return nil, err
		}
		if err = loadDirs(pc); err != nil {
			return nil, err
		}
	} else {
		// foreach consumes each root child synchronously while iterdir is suspended.
		err := c.referenceFilePages(ctx, func() string { return "https://proapi.115.com/android/2.0/ufile/files" }, url.Values{"aid": {"1"}, "record_open_time": {"1"}, "asc": {"1"}, "cid": {"0"}, "cur": {"1"}, "count_folders": {"1"}, "fc_mix": {"1"}, "min_size": {"0"}, "max_size": {"0"}, "o": {"user_ptime"}, "offset": {"0"}, "show_dir": {"1"}, "suffix": {""}, "nf": {"1"}, "limit": {"7000"}, "custom_order": {"2"}}, 7000, 500*time.Millisecond, func(raw []byte) error {
			var response struct {
				Data []map[string]json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(raw, &response); err != nil {
				return err
			}
			for _, item := range response.Data {
				if rawText(item, "fid") != "" {
					continue
				}
				fid, name, pc := rawText(item, "cid"), rawText(item, "n"), rawText(item, "pc")
				if fid == "" || name == "." || name == ".." || strings.Contains(name, "/") || pc == "" {
					return fmt.Errorf("invalid root subtitle directory")
				}
				paths[fid] = "/" + name
				if err := loadDirs(pc); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	var resolve func(string, map[string]bool) (string, error)
	resolve = func(id string, seen map[string]bool) (string, error) {
		if p, ok := paths[id]; ok {
			return p, nil
		}
		if seen[id] {
			return "", fmt.Errorf("subtitle hierarchy cycle")
		}
		seen[id] = true
		node, ok := nodes[id]
		if !ok {
			return "", fmt.Errorf("missing subtitle hierarchy node")
		}
		parent, e := resolve(node.ParentID, seen)
		if e != nil {
			return "", e
		}
		if node.Name == "." || node.Name == ".." || strings.Contains(node.Name, "/") {
			return "", fmt.Errorf("invalid subtitle directory name")
		}
		paths[id] = strings.TrimRight(parent, "/") + "/" + node.Name
		return paths[id], nil
	}
	for id := range nodes {
		if _, e := resolve(id, map[string]bool{}); e != nil {
			return nil, e
		}
	}
	return paths, nil
}

// SubtitleAttachments preserves suffix pass order and direct subtitle URLs.
func (c *Client) SubtitleAttachments(ctx context.Context, id, uid, base string, skip func(string) bool, emit func(string, []byte) error) error {
	ctx, cancel := context.WithCancel(ctx)
	cache := map[string]bool{}
	type transfer struct {
		remote string
		body   []byte
		err    error
	}
	var pending []chan transfer
	type job struct {
		remote, rawURL string
		done           chan transfer
	}
	var mu sync.Mutex
	ready := sync.NewCond(&mu)
	var jobs []job
	closed := false
	var workers sync.WaitGroup
	for range 10 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for {
				mu.Lock()
				for len(jobs) == 0 && !closed {
					ready.Wait()
				}
				if len(jobs) == 0 {
					mu.Unlock()
					return
				}
				current := jobs[0]
				jobs = jobs[1:]
				mu.Unlock()
				result := transfer{remote: current.remote}
				if e := ctx.Err(); e != nil {
					result.err = e
					current.done <- result
					continue
				}
				request, e := http.NewRequestWithContext(ctx, http.MethodGet, current.rawURL, nil)
				if e != nil {
					result.err = e
					current.done <- result
					continue
				}
				response, e := c.http.Do(request)
				if e != nil {
					result.err = e
					current.done <- result
					continue
				}
				result.body, result.err = io.ReadAll(response.Body)
				response.Body.Close()
				if result.err == nil && (response.StatusCode < 200 || response.StatusCode >= 300) {
					result.err = fmt.Errorf("subtitle HTTP %d", response.StatusCode)
				}
				current.done <- result
			}
		}()
	}
	defer func() {
		cancel()
		mu.Lock()
		closed = true
		ready.Broadcast()
		mu.Unlock()
		workers.Wait()
	}()
	submit := func(remote, rawURL string) {
		done := make(chan transfer, 1)
		pending = append(pending, done)
		mu.Lock()
		jobs = append(jobs, job{remote, rawURL, done})
		ready.Signal()
		mu.Unlock()
	}

	for _, suffix := range []string{"srt", "ass", "ssa"} {
		type hierarchyResult struct {
			paths map[string]string
			err   error
		}
		hierarchy := make(chan hierarchyResult, 1)
		go func() { paths, e := c.subtitleHierarchy(ctx, id, uid, base); hierarchy <- hierarchyResult{paths, e} }()
		candidates, listErr := c.referenceFileList(ctx, func() string { return "https://proapi.115.com/android/2.0/ufile/files" }, url.Values{"aid": {"1"}, "record_open_time": {"1"}, "asc": {"1"}, "cid": {id}, "cur": {"0"}, "count_folders": {"0"}, "fc_mix": {"1"}, "min_size": {"0"}, "max_size": {"0"}, "o": {"user_ptime"}, "offset": {"0"}, "show_dir": {"0"}, "suffix": {suffix}, "limit": {"7000"}, "custom_order": {"2"}}, 7000, time.Second)
		dirs := <-hierarchy
		if listErr != nil {
			return listErr
		}
		if dirs.err != nil {
			return dirs.err
		}
		for _, candidate := range candidates {
			parent, ok := dirs.paths[rawText(candidate, "cid")]
			if !ok {
				return fmt.Errorf("subtitle parent path missing")
			}
			name := rawText(candidate, "n")
			candidatePath := strings.TrimRight(parent, "/") + "/" + name
			if cache[candidatePath] {
				continue
			}
			raw, e := c.VideoSubtitles(ctx, rawText(candidate, "pc"))
			if e != nil {
				return e
			}
			if e = CheckResponse(raw); e != nil {
				return e
			}
			var resp struct {
				Data struct {
					List []struct {
						Name string `json:"file_name"`
						URL  string `json:"url"`
					} `json:"list"`
				} `json:"data"`
			}
			if e = json.Unmarshal(raw, &resp); e != nil {
				return e
			}
			for _, subtitle := range resp.Data.List {
				subtitle.Name = unescapeName(subtitle.Name)
				if subtitle.Name == "" || subtitle.Name == "." || subtitle.Name == ".." || strings.Contains(subtitle.Name, "/") {
					return fmt.Errorf("invalid returned subtitle name")
				}
				remote := path.Join(parent, subtitle.Name)
				if cache[remote] || (skip != nil && skip(remote)) {
					continue
				}
				submit(remote, subtitle.URL)
				cache[remote] = true
			}
		}
	}
	for _, done := range pending {
		result := <-done
		if result.err != nil {
			return result.err
		}
		if e := emit(result.remote, result.body); e != nil {
			return e
		}
	}

	return nil
}
