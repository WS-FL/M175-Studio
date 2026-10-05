package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSettingsValidation(t *testing.T) {
	s := defaults()
	if _, e := cleanSettings(s); e != nil {
		t.Fatal(e)
	}
	for _, ip := range []string{"8.8.8.8", "example.com", "http://192.168.5.44", ""} {
		x := s
		x.IP = ip
		if _, e := cleanSettings(x); e == nil {
			t.Fatal("accepted nonlocal IP", ip)
		}
	}
	x := s
	x.Output = "relative"
	if _, e := cleanSettings(x); e == nil {
		t.Fatal("relative path accepted")
	}
	x = s
	x.Prefix = "../test"
	if _, e := cleanSettings(x); e == nil {
		t.Fatal("unsafe name")
	}
	x = s
	x.DPI = 1200
	if _, e := cleanSettings(x); e == nil {
		t.Fatal("unsupported DPI")
	}
	x = s
	x.Mode = "gray"
	x.Format = "jpeg"
	if _, e := cleanSettings(x); e == nil {
		t.Fatal("unsupported mode")
	}
}
func TestGrayKeepsOriginal(t *testing.T) {
	p := filepath.Join(t.TempDir(), "page-001.jpg")
	raw := sampleJPEG(t)
	if e := os.WriteFile(p, raw, 0600); e != nil {
		t.Fatal(e)
	}
	out, e := grayPages([]string{p})
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(out[0])
	if e != nil {
		t.Fatal(e)
	}
	c, e := jpeg.DecodeConfig(bytes.NewReader(b))
	if e != nil || c.ColorModel != color.GrayModel {
		t.Fatal("not gray", e)
	}
	orig, _ := os.ReadFile(p)
	if !bytes.Equal(orig, raw) {
		t.Fatal("original changed")
	}
	if e = writePDF(filepath.Join(filepath.Dir(p), "gray.pdf"), out, 300); e != nil {
		t.Fatal(e)
	}
}
func TestHistoryAndTraversal(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "scan-example")
	os.Mkdir(dir, 0700)
	os.WriteFile(filepath.Join(dir, "page-001.jpg"), sampleJPEG(t), 0600)
	os.Mkdir(filepath.Join(root, "scan-connection-only"), 0700)
	doc := Document{Title: "Test Document", Created: time.Now().Format(time.RFC3339), DPI: 300, Source: "platen", Paper: "A4", Mode: "color"}
	atomicJSON(filepath.Join(dir, "m175-studio.json"), doc)
	a := &appServer{settings: Settings{Output: root}, docs: listDocuments(root)}
	if len(a.docs) != 1 || a.docs[0].Title != "Test Document" {
		t.Fatal(a.docs)
	}
	if _, e := a.documentAsset("scan-example", "page-001.jpg"); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"../../etc/passwd", "m175-studio.json", "", "../page-001.jpg"} {
		if _, e := a.documentAsset("scan-example", name); e == nil {
			t.Fatal("allowed unsafe asset", name)
		}
	}
	outside := filepath.Join(t.TempDir(), "private.jpg")
	os.WriteFile(outside, sampleJPEG(t), 0600)
	os.Remove(filepath.Join(dir, "page-001.jpg"))
	os.Symlink(outside, filepath.Join(dir, "page-001.jpg"))
	if _, e := a.documentAsset("scan-example", "page-001.jpg"); e == nil {
		t.Fatal("symlink escaped root")
	}
}
func TestHTTPGuards(t *testing.T) {
	a, e := loadServer(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	a.host = "127.0.0.1:9999"
	a.token = "secret"
	tests := []struct {
		method, path, host, origin, guard string
		want                              int
	}{
		{"GET", "/s/secret/api/state", a.host, "", "", 200},
		{"GET", "/s/wrong/api/state", a.host, "", "", 404},
		{"GET", "/s/secret/api/state", "evil.example", "", "", 403},
		{"POST", "/s/secret/api/cancel", a.host, "http://evil.example", "1", 403},
		{"POST", "/s/secret/api/cancel", a.host, "", "", 403},
		{"GET", "/s/secret/api/scan", a.host, "", "", 404},
		{"POST", "/s/secret/api/cancel", a.host, "http://" + a.host, "1", 200},
		{"GET", "/s/secret/../../etc/passwd", a.host, "", "", 404},
	}
	for _, tt := range tests {
		r := httptest.NewRequest(tt.method, "http://"+tt.host+tt.path, strings.NewReader("{}"))
		r.Header.Set("Origin", tt.origin)
		r.Header.Set("X-M175-Request", tt.guard)
		w := httptest.NewRecorder()
		a.route(w, r)
		if w.Code != tt.want {
			t.Fatalf("%s %s: %d, want %d", tt.method, tt.path, w.Code, tt.want)
		}
	}
}
func TestSettingsRoundtrip(t *testing.T) {
	dir := t.TempDir()
	a, e := loadServer(dir)
	if e != nil {
		t.Fatal(e)
	}
	s := a.settings
	s.Paper = "A4"
	s.Prefix = "Résumé"
	s.Output = t.TempDir()
	if e = atomicJSON(filepath.Join(dir, "settings.json"), s); e != nil {
		t.Fatal(e)
	}
	a, e = loadServer(dir)
	if e != nil || a.settings.Prefix != "Résumé" || a.settings.Paper != "A4" {
		t.Fatal(e, a.settings)
	}
}
func TestCancelBeforeNetwork(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := client{ctx: ctx}
	start := time.Now()
	if e := c.pause(5 * time.Second); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if time.Since(start) > time.Second {
		t.Fatal("cancel took too long")
	}
}
func TestBusyRejectsDuplicate(t *testing.T) {
	a := &appServer{job: Job{Busy: true}}
	if e := a.start("scan"); e == nil {
		t.Fatal("duplicate accepted")
	}
}
func TestStateJSONHasNoSecret(t *testing.T) {
	a, e := loadServer(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	a.token = "DO-NOT-EXPOSE"
	b, e := json.Marshal(a.snapshot())
	if e != nil || bytes.Contains(b, []byte(a.token)) {
		t.Fatal(string(b), e)
	}
}
func TestHTTPOnlyGETIsReadOnly(t *testing.T) {
	a, e := loadServer(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	a.host = "127.0.0.1"
	a.token = "x"
	r := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/s/x/api/cancel", nil)
	w := httptest.NewRecorder()
	a.route(w, r)
	if w.Code != 404 {
		t.Fatal(w.Code)
	}
}
