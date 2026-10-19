package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func renameFixture(t *testing.T) *appServer {
	t.Helper()
	root := t.TempDir()
	for _, id := range []string{"scan-old", "scan-another"} {
		dir := filepath.Join(root, id)
		os.Mkdir(dir, 0700)
		os.WriteFile(filepath.Join(dir, "page-001.jpg"), sampleJPEG(t), 0600)
		os.WriteFile(filepath.Join(dir, "old.pdf"), []byte("pdf-original"), 0600)
	}
	return &appServer{settings: Settings{Output: root}, docs: listDocuments(root)}
}
func TestRenameTitlesAndReferences(t *testing.T) {
	for _, tc := range []struct{ input, want string }{{"My scan", "My scan"}, {"实验 结果", "实验 结果"}, {`a/b:c*?<>|"`, "a b c"}, {" . \n ", "Untitled scan"}} {
		t.Run(tc.want, func(t *testing.T) {
			a := renameFixture(t)
			before, _ := os.ReadFile(filepath.Join(a.settings.Output, "scan-old", "page-001.jpg"))
			d, e := a.renameDocument("scan-old", tc.input)
			if e != nil || d.Title != tc.want {
				t.Fatal(d, e)
			}
			pdf, e := os.ReadFile(filepath.Join(a.settings.Output, d.ID, d.PDF))
			if e != nil || string(pdf) != "pdf-original" {
				t.Fatal("PDF changed", e)
			}
			after, _ := os.ReadFile(filepath.Join(a.settings.Output, d.ID, "page-001.jpg"))
			if !bytes.Equal(before, after) {
				t.Fatal("JPEG changed")
			}
			if _, e = a.documentAsset(d.ID, "page-001.jpg"); e != nil {
				t.Fatal("reference broken", e)
			}
			reloaded := listDocuments(a.settings.Output)
			if len(reloaded) != 2 {
				t.Fatal("legacy history missing")
			}
			found := false
			for _, doc := range reloaded {
				if doc.ID == d.ID {
					found = doc.Title == tc.want && doc.PDF == tc.want+".pdf"
				}
			}
			if !found {
				t.Fatal("rename did not survive reload")
			}
		})
	}
}
func TestRenameCollisionsAndBusy(t *testing.T) {
	a := renameFixture(t)
	if _, e := a.renameDocument("scan-old", "Report"); e != nil {
		t.Fatal(e)
	}
	d, e := a.renameDocument("scan-another", "Report")
	if e != nil || d.Title != "Report 2" {
		t.Fatal(d, e)
	}
	// Existing unrelated PDF in the same folder must never be overwritten.
	os.WriteFile(filepath.Join(a.settings.Output, "scan-old", "Reserved.pdf"), []byte("keep"), 0600)
	d, e = a.renameDocument("scan-old", "Reserved")
	if e != nil || d.Title != "Reserved 2" {
		t.Fatal(d, e)
	}
	b, _ := os.ReadFile(filepath.Join(a.settings.Output, "scan-old", "Reserved.pdf"))
	if string(b) != "keep" {
		t.Fatal("overwritten")
	}
	a.job.Busy = true
	if _, e = a.renameDocument("scan-old", "Busy"); e == nil {
		t.Fatal("rename during job")
	}
}

func TestRenameCaseAndMetadataPDF(t *testing.T) {
	a := renameFixture(t)
	if _, e := a.renameDocument("scan-old", "Report"); e != nil {
		t.Fatal(e)
	}
	d, e := a.renameDocument("scan-old", "report")
	if e != nil || d.Title != "report" {
		t.Fatal(d, e)
	}
	os.WriteFile(filepath.Join(a.settings.Output, "scan-old", "AAA-unrelated.pdf"), []byte("keep"), 0600)
	for _, d := range listDocuments(a.settings.Output) {
		if d.ID == "scan-old" && d.PDF != "report.pdf" {
			t.Fatal("lost main PDF reference", d)
		}
	}
}
