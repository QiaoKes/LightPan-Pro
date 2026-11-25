package pan115

import (
	"context"
	"litepan/internal/driver"
	"net/http"
	"reflect"
	"testing"
)

// Public entry points must preserve their wire fields through the shared Open transport.
func TestRetainedMutationWire(t *testing.T) {
	var trace []string
	d := referenceWireDriver(t, &trace, func(r *http.Request) (int, string) { return 200, `{"state":true,"data":{"cid":"77"}}` })
	ctx := context.Background()
	folder, err := d.CreateFolder(ctx, "9", " new & folder ")
	if err != nil || folder.ID != "77" || folder.Name != "new & folder" {
		t.Fatalf("folder=%+v err=%v", folder, err)
	}
	for _, err := range []error{d.RenameFile(ctx, " 12 ", " new & name "), d.MoveFiles(ctx, []string{"12", "13", "12"}, "9", ""), d.CopyFiles(ctx, []string{"12", "13"}, "9"), d.DeleteFiles(ctx, []string{"12", "13"})} {
		if err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"POST /open/folder/add file_name=new+%26+folder&pid=9", "POST /open/ufile/update file_id=12&file_name=new+%26+name", "POST /open/ufile/move file_ids=12%2C13%2C12&to_cid=9", "POST /open/ufile/copy file_id=12%2C13&nodupli=0&pid=9", "POST /open/ufile/delete file_ids=12%2C13"}
	if !reflect.DeepEqual(trace, want) {
		t.Fatalf("trace=%q want=%q", trace, want)
	}
}

func TestRetainedProfileOfflineWire(t *testing.T) {
	var trace []string
	d := referenceWireDriver(t, &trace, func(r *http.Request) (int, string) {
		switch r.URL.Path {
		case pathUserInfo:
			return 200, `{"state":true,"data":{"user_id":123,"user_name":"fixture","rt_space_info":{"all_total":{"size":100},"all_use":{"size":30}},"vip_info":{"level_name":"VIP"}}}`
		case pathOfflineAddURLs:
			return 200, `{"state":true,"data":[{"state":true,"info_hash":"ABC","url":"https://example.invalid/a"}]}`
		case pathOfflineTaskList:
			return 200, `{"state":true,"data":{"page_count":1,"tasks":[{"info_hash":"ABC","percentDone":"42.5","status":1,"name":"file"}]}}`
		default:
			return 200, `{"state":true,"data":{}}`
		}
	})
	ctx := context.Background()
	p, err := d.GetAccountProfile(ctx)
	if err != nil || p.UserID != "123" || p.UsedBytes != 30 || p.TotalBytes != 100 || p.Membership != "VIP" {
		t.Fatalf("profile=%+v err=%v", p, err)
	}
	added, err := d.AddOfflineURLs(ctx, driver.OfflineURLRequest{URLs: []string{" https://example.invalid/a ", ""}, ParentID: "9"})
	if err != nil || len(added) != 1 || !added[0].Success {
		t.Fatalf("added=%+v err=%v", added, err)
	}
	updates, err := d.RefreshOfflineTasks(ctx, []driver.OfflineTaskRef{{InfoHash: "abc"}})
	if err != nil || len(updates) != 1 || updates[0].Progress != 42 || updates[0].Status != driver.OfflineStatusRunning {
		t.Fatalf("updates=%+v err=%v", updates, err)
	}
	if err := d.DeleteOfflineTask(ctx, driver.OfflineTaskRef{InfoHash: " ABC "}, true); err != nil {
		t.Fatal(err)
	}
	want := []string{"GET /open/user/info ", "POST /open/offline/add_task_urls urls=https%3A%2F%2Fexample.invalid%2Fa&wp_path_id=9", "GET /open/offline/get_task_list page=1", "POST /open/offline/del_task del_source_file=1&info_hash=ABC"}
	if !reflect.DeepEqual(trace, want) {
		t.Fatalf("trace=%q want=%q", trace, want)
	}
}
