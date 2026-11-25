package embyproxy

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

func differentPlaybackDevice(devices []string, device string) bool {
	return len(devices) > 0 && !(len(devices) == 1 && devices[0] == device)
}

func (s *Service) needsTemporaryCopy(r *http.Request, cfg Config) bool {
	if s.playback == nil {
		return false
	}
	source := stripMediaSourcePrefix(queryValue(r, "mediasourceid"))
	if source == "" {
		return false
	}
	base := strings.TrimRight(cfg.EmbyURL, "/")
	if !strings.HasSuffix(strings.ToLower(base), "/emby") {
		base += "/emby"
	}
	q := url.Values{"IncludeAllSessionsIfAdmin": {"true"}, "IsPlaying": {"true"}, "api_key": {cfg.APIKey}}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, base+"/Sessions?"+q.Encode(), nil)
	if err != nil {
		return false
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var sessions []struct {
		DeviceID  string `json:"DeviceId"`
		PlayState struct {
			MediaSourceID string `json:"MediaSourceId"`
		} `json:"PlayState"`
	}
	if resp.StatusCode >= 400 || json.NewDecoder(resp.Body).Decode(&sessions) != nil {
		return false
	}
	var devices []string
	for _, session := range sessions {
		if session.PlayState.MediaSourceID == source {
			devices = append(devices, session.DeviceID)
		}
	}
	return differentPlaybackDevice(devices, playbackDevice(r))
}
