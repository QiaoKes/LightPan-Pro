package pan115

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

type qrSession struct {
	created                                            time.Time
	mu                                                 sync.Mutex
	purpose, app, clientID, verifier, uid, stamp, sign string
	cancel                                             context.CancelFunc
	ctx                                                context.Context
	attempts                                           int
	next                                               time.Time
}

var qrSessions = struct {
	sync.Mutex
	entries map[string]*qrSession
}{entries: make(map[string]*qrSession)}

func qrRandom(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func removeQR(token string) {
	qrSessions.Lock()
	s := qrSessions.entries[token]
	delete(qrSessions.entries, token)
	qrSessions.Unlock()
	if s != nil {
		s.cancel()
	}
}

// Bound abandoned local sessions without asserting an upstream expiry duration.
func storeQR(token string, s *qrSession) {
	qrSessions.Lock()
	if len(qrSessions.entries) >= 1024 {
		var oldestToken string
		var oldest *qrSession
		for key, item := range qrSessions.entries {
			if oldest == nil || item.created.Before(oldest.created) {
				oldestToken = key
				oldest = item
			}
		}
		delete(qrSessions.entries, oldestToken)
		if oldest != nil {
			oldest.cancel()
		}
	}
	s.created = time.Now()
	qrSessions.entries[token] = s
	qrSessions.Unlock()
}
