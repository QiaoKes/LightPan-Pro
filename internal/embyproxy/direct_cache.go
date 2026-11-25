package embyproxy

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// directPlaybackKey preserves device identity independently of player UA.
// PlaySessionId and Range intentionally do not participate in link reuse.
type directPlaybackKey struct {
	generation                       uint64
	server, item, source, ua, device string
}
type directPlaybackEntry struct {
	accountID int64
	location  string
	expires   time.Time
}
type directPlaybackSlot struct {
	sync.Mutex
	entry   directPlaybackEntry
	users   int
	touched time.Time
}

func playbackDevice(r *http.Request) string {
	if device := queryValue(r, "deviceid"); device != "" {
		return device
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

// redirectCapture observes headers only; streaming responses retain the original
// ResponseWriter and are never buffered into the direct-link cache.
type redirectCapture struct {
	committed bool
	accountID int64
	http.ResponseWriter
	location  string
	cacheable bool
}

func (w *redirectCapture) WriteHeader(status int) {
	w.committed = true
	if status == http.StatusFound {
		w.location = w.Header().Get("Location")
	}
	w.ResponseWriter.WriteHeader(status)
}
func (w *redirectCapture) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (s *Service) acquireDirectSlot(key directPlaybackKey) *directPlaybackSlot {
	s.directMu.Lock()
	defer s.directMu.Unlock()
	if s.directSlots == nil {
		s.directSlots = make(map[directPlaybackKey]*directPlaybackSlot)
	}
	now := time.Now()
	for k, v := range s.directSlots {
		if v.users == 0 && (now.Sub(v.touched) > 1200*time.Second || k.generation != key.generation) {
			delete(s.directSlots, k)
		}
	}
	slot := s.directSlots[key]
	if slot == nil {
		slot = &directPlaybackSlot{}
		s.directSlots[key] = slot
	}
	slot.users++
	slot.touched = now
	return slot
}
func (s *Service) releaseDirectSlot(key directPlaybackKey, slot *directPlaybackSlot) {
	s.directMu.Lock()
	defer s.directMu.Unlock()
	slot.users--
	slot.touched = time.Now()
	// Bound retained idle identities without evicting a concurrent resolver.
	if len(s.directSlots) > 512 && slot.users == 0 {
		delete(s.directSlots, key)
	}
}

func (w *redirectCapture) Write(p []byte) (int, error) {
	w.committed = true
	return w.ResponseWriter.Write(p)
}
