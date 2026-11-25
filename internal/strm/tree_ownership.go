package strm

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type treeInventory struct {
	TaskID  int64
	Files   map[string]string
	Pending map[string]string `json:",omitempty"`
}

const treeInventoryName = ".litepan-tree-inventory.json"

// safeOwnedPath rejects symlinks in every existing component, including root.
func safeOwnedPath(root, rel string) (string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	// Canonicalize host ancestors (/var on macOS), while rejecting task-root links.
	if info, e := os.Lstat(abs); e == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("managed root is symlink")
	}
	parent := filepath.Dir(abs)
	if resolved, e := filepath.EvalSymlinks(parent); e == nil {
		abs = filepath.Join(resolved, filepath.Base(abs))
	}

	if rel == "" || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid managed output path")
	}
	full := filepath.Join(abs, rel)
	r, err := filepath.Rel(abs, full)
	if err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("managed output escapes root")
	}
	cur := string(filepath.Separator)
	for _, part := range strings.Split(strings.TrimPrefix(full, string(filepath.Separator)), string(filepath.Separator)) {
		cur = filepath.Join(cur, part)
		info, e := os.Lstat(cur)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return "", e
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("managed output traverses symlink: %s", cur)
		}
	}
	return full, nil
}
func treeFingerprint(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
func loadTreeInventory(root string, id int64) (treeInventory, error) {
	inv := treeInventory{TaskID: id, Files: map[string]string{}}
	_, err := safeOwnedPath(root, treeInventoryName)
	if err != nil {
		return inv, err
	}
	confined, e := os.OpenRoot(root)
	if os.IsNotExist(e) {
		return inv, nil
	}
	if e != nil {
		return inv, e
	}
	defer confined.Close()
	raw, err := confined.ReadFile(treeInventoryName)
	if os.IsNotExist(err) {
		entries, e := os.ReadDir(root)
		if os.IsNotExist(e) {
			return inv, nil
		}
		if e != nil {
			return inv, e
		}
		if len(entries) > 0 {
			return inv, fmt.Errorf("nonempty unmanaged output directory")
		}
		return inv, nil
	}
	if err != nil {
		return inv, err
	}
	if err = json.Unmarshal(raw, &inv); err != nil {
		return inv, err
	}
	if inv.TaskID != id {
		return inv, fmt.Errorf("output belongs to task %s", strconv.FormatInt(inv.TaskID, 10))
	}
	if inv.Files == nil {
		inv.Files = map[string]string{}
	}
	return inv, nil
}
func saveTreeInventory(root string, inv treeInventory) error {
	if _, err := safeOwnedPath(root, treeInventoryName); err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}
	confined, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer confined.Close()
	b, err := json.Marshal(inv)
	if err != nil {
		return err
	}
	var nonce [16]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return err
	}
	temp := ".litepan-inventory-" + hex.EncodeToString(nonce[:])
	f, err := confined.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer confined.Remove(temp)
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return confined.Rename(temp, treeInventoryName)
}

// ValidateTaskOutput rejects both equal and nested task roots before creation.
func ValidateTaskOutput(root string, taskID int64, relative string, others map[int64]string) error {
	target, err := safeOwnedPath(root, relative)
	if err != nil {
		return err
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	if target == absolute || target == string(filepath.Separator) {
		return fmt.Errorf("output cannot be root")
	}
	for id, other := range others {
		if id == taskID {
			continue
		}
		candidate, e := safeOwnedPath(root, other)
		if e != nil {
			return e
		}
		if target == candidate || strings.HasPrefix(target, candidate+string(filepath.Separator)) || strings.HasPrefix(candidate, target+string(filepath.Separator)) {
			return fmt.Errorf("output overlaps task %d", id)
		}
	}
	entries, err := os.ReadDir(target)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		_, err = loadTreeInventory(target, taskID)
		return err
	}
	return nil
}

func deleteTreeOutputs(root string) (bool, error) {
	marker, err := safeOwnedPath(root, treeInventoryName)
	if err != nil {
		return true, err
	}
	raw, err := os.ReadFile(marker)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return true, err
	}
	var inv treeInventory
	if err = json.Unmarshal(raw, &inv); err != nil {
		return true, err
	}
	confined, err := os.OpenRoot(root)
	if err != nil {
		return true, err
	}
	defer confined.Close()
	for rel, fingerprint := range inv.Files {
		_, e := safeOwnedPath(root, rel)
		if e != nil {
			return true, e
		}
		b, e := confined.ReadFile(rel)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return true, e
		}
		if treeFingerprint(b) != fingerprint {
			continue
		}
		if e = confined.Remove(rel); e != nil {
			return true, e
		}
	}
	// Unknown/manual files and nonempty directories survive explicit task removal.
	if err = confined.Remove(treeInventoryName); err != nil {
		return true, err
	}
	_ = os.Remove(root)
	return true, nil
}

// publishTreeOutput records recoverable intent before atomic replacement. Only
// an owned fingerprint or a previously recorded publication can be replaced.
func publishTreeOutput(root string, confined *os.Root, inv *treeInventory, rel string, body []byte) error {
	if _, err := safeOwnedPath(root, rel); err != nil {
		return err
	}
	desired := treeFingerprint(body)
	current, err := confined.ReadFile(rel)
	if err == nil {
		hash := treeFingerprint(current)
		if pending := inv.Pending[rel]; pending != "" && hash == pending {
			inv.Files[rel] = pending
			delete(inv.Pending, rel)
		}
		if owned, ok := inv.Files[rel]; !ok || hash != owned {
			return fmt.Errorf("unmanaged or modified output conflict: %s", rel)
		}
		if hash == desired {
			return saveTreeInventory(root, *inv)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if inv.Pending == nil {
		inv.Pending = map[string]string{}
	}
	inv.Pending[rel] = desired
	if err = saveTreeInventory(root, *inv); err != nil {
		return err
	}
	if err = confined.MkdirAll(filepath.Dir(rel), 0755); err != nil {
		return err
	}
	var nonce [16]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return err
	}
	temp := filepath.Join(filepath.Dir(rel), ".litepan-publish-"+hex.EncodeToString(nonce[:]))
	f, err := confined.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	defer confined.Remove(temp)
	if _, err = f.Write(body); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	// Recheck ownership immediately before publishing; Root confines traversal.
	if err = confined.Rename(temp, rel); err != nil {
		return err
	}
	inv.Files[rel] = desired
	delete(inv.Pending, rel)
	return saveTreeInventory(root, *inv)
}
