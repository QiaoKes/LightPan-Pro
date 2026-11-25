package pan115client

import (
	"fmt"
	"litepan/drivers/115/client/charset"
	"strings"
)

// ParseDirectoryTree ports util_115_tree.parse_directory_tree's leaf stack.
// The export contains names, not stable IDs or sizes. It deliberately preserves
// the reference root/subtree cuts and does not invent empty-directory entries.
func ParseDirectoryTree(raw []byte, root string) ([]string, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil, fmt.Errorf("empty directory tree payload")
	}
	decoded, err := charset.Decode(raw)
	if err != nil {
		return nil, fmt.Errorf("decode directory tree: %w", err)
	}
	lines := strings.Split(decoded, "\n|")
	stack := []string{}
	cut := 2
	var leaves []string
	for i, line := range lines {
		var name string
		if i == 0 {
			name = strings.TrimSpace(strings.Trim(line, "|—"))
			if root == "/" || root == "" {
				cut = 1
				root = ""
			}
		} else {
			parts := strings.Split(strings.TrimSpace(line), "|-")
			name = parts[len(parts)-1]
		}
		depth := strings.Count(line, "|") + 1
		if name == "" {
			continue
		}
		for len(stack) >= depth {
			stack = stack[:len(stack)-1]
		}
		stack = append(stack, name)
		if i == len(lines)-1 || strings.Count(lines[i+1], "|")+1 <= depth {
			start := cut
			if start > len(stack) {
				start = len(stack)
			}
			leaves = append(leaves, root+"/"+strings.Join(stack[start:], "/"))
		}
	}
	return leaves, nil
}
