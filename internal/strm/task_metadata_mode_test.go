package strm

import (
	"context"
	"litepan/internal/domain"
	"testing"
)

func TestTaskMetadataModeDefaultsAndOverrides(t *testing.T) {
	ctx := context.Background()
	s, st := testService(t)
	s.accounts = st.Accounts
	s.strmDir = t.TempDir()
	for _, row := range []struct{ driver, explicit, want string }{{"115", "", MetadataSyncCloudPrimary}, {"other", "", ""}, {"115", MetadataSyncLocalPrimary, MetadataSyncLocalPrimary}, {"115", MetadataSyncBidirectional, MetadataSyncBidirectional}} {
		id, err := st.Accounts.Create(ctx, &domain.Account{Name: row.driver + row.explicit, DriverType: row.driver, Config: "{}", IsActive: true})
		if err != nil {
			t.Fatal(err)
		}
		task, err := s.CreateTask(ctx, &domain.StrmTask{Name: row.driver + row.explicit, AccountID: id, OutputFolder: row.driver + row.explicit, MetadataSyncMode: row.explicit})
		if err != nil {
			t.Fatal(err)
		}
		if task.MetadataSyncMode != row.want {
			t.Fatalf("%s mode=%q want %q", row.driver, task.MetadataSyncMode, row.want)
		}
		inherited := ScanSettings{MetadataSyncMode: MetadataSyncLocalPrimary}
		got := taskScanSettings(task, inherited)
		want := row.want
		if want == "" {
			want = MetadataSyncLocalPrimary
		}
		if got.MetadataSyncMode != want {
			t.Fatalf("scan mode=%q want %q", got.MetadataSyncMode, want)
		}
		task.MetadataSyncMode = MetadataSyncBidirectional
		updated, err := s.UpdateTask(ctx, task.ID, task)
		if err != nil {
			t.Fatal(err)
		}
		if updated.MetadataSyncMode != MetadataSyncBidirectional {
			t.Fatal("edit lost explicit mode")
		}
		task.MetadataSyncMode = ""
		updated, err = s.UpdateTask(ctx, task.ID, task)
		if err != nil {
			t.Fatal(err)
		}
		if updated.MetadataSyncMode != "" {
			t.Fatal("explicit inheritance lost")
		}
	}
	if _, err := s.CreateTask(ctx, &domain.StrmTask{Name: "bad", MetadataSyncMode: "unknown"}); err == nil {
		t.Fatal("invalid mode accepted")
	}
}
