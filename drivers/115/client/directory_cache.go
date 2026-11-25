package pan115client

import (
	"strings"
	"sync"
)

type cachedDirNode struct{ name, parent string }

var directoryNodes struct {
	sync.RWMutex
	byUID map[string]map[string]cachedDirNode
}

func cacheDirectoryNodes(uid string, nodes map[string]cachedDirNode) {
	directoryNodes.Lock()
	defer directoryNodes.Unlock()
	if directoryNodes.byUID == nil {
		directoryNodes.byUID = map[string]map[string]cachedDirNode{}
	}
	if directoryNodes.byUID[uid] == nil {
		directoryNodes.byUID[uid] = map[string]cachedDirNode{}
	}
	for id, node := range nodes {
		directoryNodes.byUID[uid][id] = node
	}
}
func cachedDirectoryPath(uid, id string, escape ...bool) (string, bool) {
	directoryNodes.RLock()
	defer directoryNodes.RUnlock()
	nodes := directoryNodes.byUID[uid]
	var parts []string
	seen := map[string]bool{}
	for id != "0" && id != "" {
		if seen[id] {
			return "", false
		}
		seen[id] = true
		node, ok := nodes[id]
		if !ok {
			return "", false
		}
		name := node.name
		if len(escape) == 0 || escape[0] {
			name = escapeMonitorName(name)
		}
		parts = append(parts, name)
		id = node.parent
	}
	for left, right := 0, len(parts)-1; left < right; left, right = left+1, right-1 {
		parts[left], parts[right] = parts[right], parts[left]
	}
	if len(parts) == 0 {
		return "", true
	}
	return "/" + strings.Join(parts, "/"), true
}
