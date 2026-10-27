package main

import (
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fileActionFixture(t *testing.T) *appServer {
	a := renameFixture(t)
	root, err := filepath.EvalSymlinks(a.settings.Output)
	if err != nil {
		t.Fatal(err)
	}
	a.settings.Output = root
	return a
}

func TestCopyDocumentFiles(t *testing.T) {
	a := fileActionFixture(t)
	var action string
	var paths []string
	a.fileAction = func(op string, p []string) error { action = op; paths = p; return nil }
	d, err := a.renameDocument("scan-old", "研究 Report")
	if err != nil {
		t.Fatal(err)
	}
	if err = a.copyDocument(d.ID); err != nil {
		t.Fatal(err)
	}
	if action != "copy-files" || !reflect.DeepEqual(paths, []string{filepath.Join(a.settings.Output, d.ID, "研究 Report.pdf")}) {
		t.Fatal(action, paths)
	}
	// JPEG-only scans copy all original pages, in page order.
	dir := filepath.Join(a.settings.Output, "scan-another")
	os.Remove(filepath.Join(dir, "old.pdf"))
	os.WriteFile(filepath.Join(dir, "page-002.jpg"), sampleJPEG(t), 0600)
	a.docs = listDocuments(a.settings.Output)
	if err = a.copyDocument("scan-another"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(paths, []string{filepath.Join(dir, "page-001.jpg"), filepath.Join(dir, "page-002.jpg")}) {
		t.Fatal(paths)
	}
}
func TestTrashConfirmationAndPreservation(t *testing.T) {
	a := fileActionFixture(t)
	dest := filepath.Join(t.TempDir(), "scan-old")
	calls := 0
	a.renameDocument("scan-old", "Keep record")
	dir := filepath.Join(a.settings.Output, "scan-old")
	before := map[string]string{}
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		b, _ := os.ReadFile(filepath.Join(dir, entry.Name()))
		before[entry.Name()] = string(b)
	}
	a.fileAction = func(op string, p []string) error {
		calls++
		if op != "trash-folder" || !reflect.DeepEqual(p, []string{dir}) {
			t.Fatal(op, p)
		}
		return os.Rename(p[0], dest)
	}
	if a.trashDocument("scan-old", false) == nil || calls != 0 {
		t.Fatal("missing confirmation accepted")
	}
	if e := a.trashDocument("scan-old", true); e != nil {
		t.Fatal(e)
	}
	if calls != 1 || len(a.docs) != 1 || len(listDocuments(a.settings.Output)) != 1 {
		t.Fatal("history not refreshed")
	}
	for name, want := range before {
		b, e := os.ReadFile(filepath.Join(dest, name))
		if e != nil || string(b) != want {
			t.Fatal("contents changed", name, e)
		}
	}
	if a.trashDocument("scan-old", true) == nil || calls != 1 {
		t.Fatal("stale target accepted")
	}
}
func TestFileActionsRejectUnsafeOrBusyTargets(t *testing.T) {
	a := fileActionFixture(t)
	a.fileAction = func(string, []string) error { t.Fatal("unsafe helper invocation"); return nil }
	for _, id := range []string{"", "../scan-old", "scan-missing", ".", "scan-old/../scan-another"} {
		if a.copyDocument(id) == nil || a.trashDocument(id, true) == nil {
			t.Fatal("accepted", id)
		}
	}
	a.job.Busy = true
	if a.copyDocument("scan-old") == nil || a.trashDocument("scan-old", true) == nil {
		t.Fatal("busy mutation accepted")
	}
	a.job.Busy = false
	// A document directory substituted by a symlink must not be followed.
	dir := filepath.Join(a.settings.Output, "scan-old")
	moved := filepath.Join(t.TempDir(), "scan-old")
	os.Rename(dir, moved)
	os.Symlink(moved, dir)
	if a.copyDocument("scan-old") == nil || a.trashDocument("scan-old", true) == nil {
		t.Fatal("symlink directory accepted")
	}
	file := filepath.Join(a.settings.Output, "scan-another", "old.pdf")
	os.Remove(file)
	os.Symlink(filepath.Join(moved, "old.pdf"), file)
	if a.copyDocument("scan-another") == nil {
		t.Fatal("symlink asset accepted")
	}
}
func TestFileActionFailurePreservesHistory(t *testing.T) {
	a := fileActionFixture(t)
	a.fileAction = func(string, []string) error { return errors.New("Trash unavailable") }
	if a.trashDocument("scan-old", true) == nil || a.copyDocument("scan-old") == nil {
		t.Fatal("failed operation reported success")
	}
	if len(a.docs) != 2 || len(listDocuments(a.settings.Output)) != 2 {
		t.Fatal("history lost")
	}
	a.fileAction = nil
	if a.copyDocument("scan-old") == nil || a.trashDocument("scan-old", true) == nil {
		t.Fatal("missing native helper accepted")
	}
}
func TestFileActionHTTPGuards(t *testing.T) {
	a := fileActionFixture(t)
	a.host = "127.0.0.1:12345"
	a.token = "test"
	calls := 0
	a.fileAction = func(string, []string) error { calls++; return nil }
	for _, tc := range []struct {
		method, path, body, guard, origin string
		want                              int
	}{
		{"GET", "copy-files", "", "", "", 404},
		{"GET", "trash", "", "", "", 404},
		{"POST", "copy-files", `{"id":"scan-old"}`, "", "", 403},
		{"POST", "trash", `{"id":"scan-old","confirmed":true}`, "1", "http://external.invalid", 403},
		{"POST", "trash", `{"id":"scan-old"}`, "1", "", 409},
		{"POST", "copy-files", `{"id":"scan-old","path":"/private"}`, "1", "", 400},
		{"POST", "copy-files", `{"id":"scan-old"}`, "1", "", 200},
	} {
		r := httptest.NewRequest(tc.method, "http://"+a.host+"/s/test/api/"+tc.path, strings.NewReader(tc.body))
		r.Header.Set("X-M175-Request", tc.guard)
		r.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		a.route(w, r)
		if w.Code != tc.want {
			t.Fatalf("%+v: %d %s", tc, w.Code, w.Body.String())
		}
	}
	if calls != 1 {
		t.Fatal("unguarded file operation", calls)
	}
}

func TestBatchCopyMixedFormatsAndDedupe(t *testing.T) {
	a := fileActionFixture(t)
	dir := filepath.Join(a.settings.Output, "scan-another")
	os.Remove(filepath.Join(dir, "old.pdf"))
	os.WriteFile(filepath.Join(dir, "page-002.jpg"), sampleJPEG(t), 0600)
	a.docs = listDocuments(a.settings.Output)
	calls := 0
	a.fileAction = func(action string, paths []string) error {
		calls++
		want := []string{filepath.Join(a.settings.Output, "scan-old", "old.pdf"), filepath.Join(dir, "page-001.jpg"), filepath.Join(dir, "page-002.jpg")}
		if action != "copy-files" || !reflect.DeepEqual(paths, want) {
			t.Fatal(action, paths)
		}
		return nil
	}
	if err := a.copyDocuments([]string{"scan-old", "scan-another", "scan-old"}); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal("clipboard replaced instead of one batch", calls)
	}
}
func TestBatchPreflightAndConfirmation(t *testing.T) {
	a := fileActionFixture(t)
	calls := 0
	a.fileAction = func(string, []string) error { calls++; return nil }
	for _, ids := range [][]string{nil, {}, {"scan-old", "scan-missing"}, {"scan-old", "../bad"}, make([]string, 201)} {
		if a.copyDocuments(ids) == nil {
			t.Fatal("invalid copy accepted", ids)
		}
		if _, err := a.trashDocuments(ids, true); err == nil {
			t.Fatal("invalid Trash accepted", ids)
		}
	}
	if _, err := a.trashDocuments([]string{"scan-old", "scan-another"}, false); err == nil {
		t.Fatal("unconfirmed batch")
	}
	a.job.Busy = true
	if a.copyDocuments([]string{"scan-old", "scan-another"}) == nil {
		t.Fatal("copy while busy")
	}
	if _, err := a.trashDocuments([]string{"scan-old", "scan-another"}, true); err == nil {
		t.Fatal("Trash while busy")
	}
	if calls != 0 {
		t.Fatal("mutation before whole-selection validation", calls)
	}
}
func TestBatchTrashPartialFailureAndRetry(t *testing.T) {
	a := fileActionFixture(t)
	trash := t.TempDir()
	failSecond := true
	calls := []string{}
	a.fileAction = func(action string, paths []string) error {
		id := filepath.Base(paths[0])
		calls = append(calls, id)
		if failSecond && id == "scan-another" {
			return errors.New("Permission denied")
		}
		return os.Rename(paths[0], filepath.Join(trash, id))
	}
	result, err := a.trashDocuments([]string{"scan-old", "scan-another", "scan-old"}, true)
	if err != nil || !reflect.DeepEqual(result.Moved, []string{"scan-old"}) || result.Failed["scan-another"] != "Permission denied" {
		t.Fatal(result, err)
	}
	if len(a.docs) != 1 || a.docs[0].ID != "scan-another" {
		t.Fatal(a.docs)
	}
	failSecond = false
	result, err = a.trashDocuments([]string{"scan-another"}, true)
	if err != nil || len(result.Moved) != 1 || len(result.Failed) != 0 || len(a.docs) != 0 {
		t.Fatal(result, err, a.docs)
	}
	if !reflect.DeepEqual(calls, []string{"scan-old", "scan-another", "scan-another"}) {
		t.Fatal(calls)
	}
	for _, id := range []string{"scan-old", "scan-another"} {
		b, err := os.ReadFile(filepath.Join(trash, id, "old.pdf"))
		if err != nil || string(b) != "pdf-original" {
			t.Fatal("original lost", id, err)
		}
	}
}
func TestBatchFileHTTP(t *testing.T) {
	a := fileActionFixture(t)
	a.host = "127.0.0.1:12345"
	a.token = "test"
	calls := 0
	a.fileAction = func(string, []string) error { calls++; return errors.New("native test failure") }
	for _, tc := range []struct {
		path, body string
		want       int
	}{
		{"copy-files", `{"ids":[]}`, 409},
		{"copy-files", `{"id":"scan-old","ids":["scan-another"]}`, 400},
		{"trash", `{"ids":["scan-old","scan-another"]}`, 409},
		{"trash", `{"ids":["scan-old","scan-another"],"confirmed":true}`, 200},
	} {
		r := httptest.NewRequest("POST", "http://"+a.host+"/s/test/api/"+tc.path, strings.NewReader(tc.body))
		r.Header.Set("X-M175-Request", "1")
		w := httptest.NewRecorder()
		a.route(w, r)
		if w.Code != tc.want {
			t.Fatal(tc, w.Code, w.Body.String())
		}
		if tc.want == 200 && !strings.Contains(w.Body.String(), `"failed":{"scan-another":"native test failure","scan-old":"native test failure"}`) {
			t.Fatal(w.Body.String())
		}
	}
	if calls != 2 {
		t.Fatal(calls)
	}
}
