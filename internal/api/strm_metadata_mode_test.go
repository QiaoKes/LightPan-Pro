package api

import (
	"encoding/json"
	"litepan/internal/domain"
	"litepan/internal/strm"
	"testing"
)

func TestTaskMetadataModeDTO(t *testing.T) {
	for _, mode := range []string{"", "cloud_primary", "local_primary", "bidirectional"} {
		task := &domain.StrmTask{ID: 1, MetadataSyncMode: mode}
		dto := toStrmTaskDTO(task, strm.TaskListMeta{}, false, "")
		raw, err := json.Marshal(dto)
		if err != nil {
			t.Fatal(err)
		}
		var decoded strmTaskDTO
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatal(err)
		}
		if !decoded.metadataSyncModeProvided || fromStrmTaskDTO(decoded).MetadataSyncMode != mode {
			t.Fatal("task metadata mode lost across API")
		}
	}
	var omitted strmTaskDTO
	if err := json.Unmarshal([]byte(`{"name":"old client"}`), &omitted); err != nil {
		t.Fatal(err)
	}
	if omitted.metadataSyncModeProvided {
		t.Fatal("omission treated as explicit inheritance")
	}
}
