package pan115client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

var pickcodeTables = map[string]string{"a": "fuln1ytpj3smg8d5a094qh7cxkbi62zvewro", "b": "sk721n9a0emlfpcrzbqdw3gjh6ty5xui48vo", "c": "ywcz3hite6f1j0guoakvdb2ns7p8qr9ml5x4", "d": "rq2vl5o7wsken9u8tp4jg3zbyc6xmhifd01a", "e": "ljm9eqbcfhw7ktv3x1dgp5ua8y6s4znr2io0", "fa": "fumk0ytpj3sng8d5a194qh7cxlbi62zvewro", "fb": "sk732o9a1enmfpcrzbqdw4gjh6ty5xui08vl", "fc": "ywcz6hite9f4j3gup2kvdb5osal0qr1nm8x7", "fd": "on6vl0r2wpkeq9u3ts8jg7zbyc1xmhifd45a", "fe": "ljm0es2cfhwakqv6x4dgp8r1by9u7znt5io3"}

const pickcodeAlphabet = "0123456789abcdefghijklmnopqrstuvwxyz"

func decodePickcode(p string) (id, stable string, err error) {
	if len(p) < 6 {
		return "", "", fmt.Errorf("invalid pickcode")
	}
	prefix := p[:1]
	if prefix == "f" {
		prefix = p[:2]
	}
	table, ok := pickcodeTables[prefix]
	if !ok {
		return "", "", fmt.Errorf("invalid pickcode prefix")
	}
	var plain strings.Builder
	for _, ch := range []byte(p[len(prefix):]) {
		i := strings.IndexByte(pickcodeAlphabet, ch)
		if i < 0 {
			return "", "", fmt.Errorf("invalid pickcode character")
		}
		plain.WriteByte(table[i])
	}
	decoded := plain.String()
	if len(decoded) <= 4 {
		return "", "", fmt.Errorf("invalid pickcode body")
	}
	n, e := strconv.ParseUint(decoded[:len(decoded)-4], 36, 64)
	return strconv.FormatUint(n, 10), decoded[len(decoded)-4:], e
}
func encodePickcode(id, stable string) (string, error) {
	n, e := strconv.ParseUint(id, 10, 64)
	if e != nil {
		return "", e
	}
	if n == 0 {
		return "", nil
	}
	var out strings.Builder
	out.WriteByte('a')
	for _, ch := range []byte(strconv.FormatUint(n, 36) + stable) {
		i := strings.IndexByte(pickcodeTables["a"], ch)
		if i < 0 {
			return "", fmt.Errorf("invalid stable point")
		}
		out.WriteByte(pickcodeAlphabet[i])
	}
	return out.String(), nil
}

var stablePoints sync.Map

func (c *Client) directoryPickcode(ctx context.Context, id, uid string) (string, error) {
	if stable, ok := stablePoints.Load(uid); ok {
		return encodePickcode(id, stable.(string))
	}
	raw, err := c.Request(ctx, http.MethodGet, "https://webapi.115.com/files", url.Values{"aid": {"1"}, "count_folders": {"1"}, "record_open_time": {"1"}, "show_dir": {"1"}, "limit": {"1"}, "offset": {"0"}, "cid": {"0"}}, nil, nil)
	if err != nil {
		return "", err
	}
	if err = CheckResponse(raw); err != nil {
		return "", err
	}
	var resp struct {
		Data []struct {
			PC string `json:"pc"`
		} `json:"data"`
	}
	if err = json.Unmarshal(raw, &resp); err != nil {
		return "", err
	}
	if len(resp.Data) == 0 {
		return "", fmt.Errorf("no root pickcode available")
	}
	_, stable, err := decodePickcode(resp.Data[0].PC)
	if err != nil {
		return "", err
	}
	stablePoints.Store(uid, stable)
	return encodePickcode(id, stable)
}

type DirectoryEventFile struct {
	ID, ParentID, Name, Path, Pickcode string
	Size                               int64
}
type downloadFolder struct{ ID, ParentID, Name string }

func rawText(m map[string]json.RawMessage, key string) string {
	var s string
	if json.Unmarshal(m[key], &s) == nil {
		if key == "n" || key == "name" || key == "fn" || key == "file_name" {
			return unescapeName(s)
		}
		return s
	}
	return string(m[key])
}
func (c *Client) downloadNodes(ctx context.Context, pickcode string, files bool) ([]map[string]json.RawMessage, error) {
	endpoint := "downfolders"
	if files {
		endpoint = "downfiles"
	}
	var out []map[string]json.RawMessage
	err := referencePageWorkers(ctx, func(ctx context.Context, page int) ([]byte, bool, error) {
		raw, err := c.ECDHRequest(ctx, http.MethodGet, "https://proapi.115.com/os_windows/2.0/ufile/"+endpoint, url.Values{"pickcode": {pickcode}, "page": {strconv.Itoa(page)}, "per_page": {"5000"}}, nil, nil)
		if err != nil {
			return nil, false, err
		}
		if err = CheckResponse(raw); err != nil {
			return nil, false, err
		}
		var response struct {
			Data struct {
				HasNext json.RawMessage `json:"has_next_page"`
			} `json:"data"`
		}
		if err = json.Unmarshal(raw, &response); err != nil {
			return nil, false, err
		}
		if len(response.Data.HasNext) == 0 {
			return nil, false, &pagingStopError{fmt.Errorf("download nodes missing has_next_page")}
		}
		value := string(response.Data.HasNext)
		return raw, value == "" || value == "false" || value == "0" || value == "null", nil
	}, func(raw []byte) error {
		var resp struct {
			Data struct {
				List []map[string]json.RawMessage `json:"list"`
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &resp); err != nil {
			return err
		}
		if files && len(resp.Data.List) > 0 {
			ids := make([]string, 0, len(resp.Data.List))
			for _, item := range resp.Data.List {
				id, _, e := decodePickcode(rawText(item, "pc"))
				if e != nil {
					return e
				}
				ids = append(ids, id)
			}
			for {
				skim, e := c.Request(ctx, http.MethodPost, "https://webapi.115.com/files/file", nil, url.Values{"file_id": {strings.Join(ids, ",")}}, nil)
				if e != nil {
					return e
				}
				var response struct {
					State bool                         `json:"state"`
					Error string                       `json:"error"`
					Data  []map[string]json.RawMessage `json:"data"`
				}
				if e = json.Unmarshal(skim, &response); e != nil {
					return e
				}
				if !response.State && response.Error == "参数错误。" {
					if e = ctx.Err(); e != nil {
						return e
					}
					continue
				}
				if !response.State && response.Error == "文件不存在" {
					break
				}
				if e = CheckResponse(skim); e != nil {
					return e
				}
				names := map[string]json.RawMessage{}
				for _, item := range response.Data {
					names[rawText(item, "file_id")] = item["file_name"]
				}
				for i, item := range resp.Data.List {
					if name, ok := names[ids[i]]; ok {
						item["name"] = name
					}
				}
				break
			}
		}
		out = append(out, resp.Data.List...)
		return nil
	})
	return out, err
}

// DirectoryEventFiles ports the bundled skim traversal's concurrent hierarchy
// and file loading; the caller preserves the reference three-second delay.
func (c *Client) DirectoryEventFiles(ctx context.Context, id, uid string) ([]DirectoryEventFile, error) {
	pc, err := c.directoryPickcode(ctx, id, uid)
	if err != nil {
		return nil, err
	}
	type result struct {
		nodes []map[string]json.RawMessage
		err   error
	}
	folders := make(chan result, 1)
	files := make(chan result, 1)
	go func() { n, e := c.downloadNodes(ctx, pc, false); folders <- result{n, e} }()
	go func() { n, e := c.downloadNodes(ctx, pc, true); files <- result{n, e} }()
	base, cached := cachedDirectoryPath(uid, id)
	var baseErr error
	if !cached {
		base, baseErr = c.FolderPath(ctx, id, uid)
	}
	directoryResult, fileResult := <-folders, <-files
	if baseErr != nil {
		return nil, baseErr
	}
	if directoryResult.err != nil {
		return nil, directoryResult.err
	}
	if fileResult.err != nil {
		return nil, fileResult.err
	}
	paths := map[string]string{id: base}
	parents := map[string]downloadFolder{}
	rawNodes := map[string]cachedDirNode{}
	for _, item := range directoryResult.nodes {
		fid := rawText(item, "fid")
		parents[fid] = downloadFolder{fid, rawText(item, "pid"), rawText(item, "fn")}
		rawNodes[fid] = cachedDirNode{rawText(item, "fn"), rawText(item, "pid")}
	}
	cacheDirectoryNodes(uid, rawNodes)
	var bind func(string, map[string]bool) (string, error)
	bind = func(fid string, visiting map[string]bool) (string, error) {
		if p, ok := paths[fid]; ok {
			return p, nil
		}
		if visiting[fid] {
			return "", fmt.Errorf("directory cycle")
		}
		visiting[fid] = true
		node, ok := parents[fid]
		if !ok {
			return "", fmt.Errorf("missing directory ancestor")
		}
		parent, e := bind(node.ParentID, visiting)
		if e != nil {
			return "", e
		}
		paths[fid] = parent + "/" + escapeMonitorName(node.Name)
		return paths[fid], nil
	}
	var out []DirectoryEventFile
	for _, item := range fileResult.nodes {
		pc := rawText(item, "pc")
		fid, _, e := decodePickcode(pc)
		if e != nil {
			return nil, e
		}
		pid := rawText(item, "pid")
		parent, e := bind(pid, map[string]bool{})
		if e != nil {
			return nil, e
		}
		name := rawText(item, "name")
		if name == "" {
			return nil, fmt.Errorf("missing or escaped directory file name")
		}
		size, _ := strconv.ParseInt(rawText(item, "fs"), 10, 64)
		out = append(out, DirectoryEventFile{fid, pid, name, parent + "/" + escapeMonitorName(name), pc, size})
	}
	return out, nil
}
