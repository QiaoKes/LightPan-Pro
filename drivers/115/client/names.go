package pan115client

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

var characterReference = regexp.MustCompile("\\[\x02([0-9]+)\\]")

func unescapeName(value string) string {
	return characterReference.ReplaceAllStringFunc(value, func(match string) string {
		parts := characterReference.FindStringSubmatch(match)
		n, err := strconv.ParseInt(parts[1], 10, 32)
		if err != nil || n < 0 || n > utf8.MaxRune {
			return match
		}
		return string(rune(n))
	})
}

// escapeMonitorName is posixpatht.escape(parse_dots=True), used by monitor
// get_path and skim traversal. Subtitle escape=None deliberately bypasses it.
func escapeMonitorName(name string) string {
	if name == "." || name == ".." {
		return "\\" + name
	}
	return strings.ReplaceAll(strings.ReplaceAll(name, "\\", "\\\\"), "/", "\\/")
}
func unescapeMonitorName(name string) string {
	if name == "\\." || name == "\\.." {
		name = name[1:]
	}
	return strings.ReplaceAll(strings.ReplaceAll(name, "\\/", "/"), "\\\\", "\\")
}

// splitMonitorPath is the exercised absolute posixpatht.splits branch. Dot
// navigation is handled before unescaping, retaining escaped literal dot names.
func splitMonitorPath(remote string) []string {
	var rawParts []string
	start, slashes := 0, 0
	for i := 0; i < len(remote); i++ {
		if remote[i] == '/' && slashes%2 == 0 {
			rawParts = append(rawParts, remote[start:i])
			start = i + 1
		}
		if remote[i] == '\\' {
			slashes++
		} else {
			slashes = 0
		}
	}
	rawParts = append(rawParts, remote[start:])
	var parts []string
	for _, part := range rawParts {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			if len(parts) > 0 {
				parts = parts[:len(parts)-1]
			}
			continue
		}
		parts = append(parts, unescapeMonitorName(part))
	}
	return parts
}
