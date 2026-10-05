// M175 Studio: a loopback-only application service. No third-party Go packages.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

//go:embed ui/*
var assets embed.FS

type Settings struct {
	IP     string `json:"ip"`
	Port   int    `json:"port"`
	Output string `json:"output"`
	Source string `json:"source"`
	Paper  string `json:"paper"`
	DPI    int    `json:"dpi"`
	Mode   string `json:"mode"`
	Format string `json:"format"`
	Prefix string `json:"prefix"`
}
type Document struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Created string   `json:"created"`
	Pages   []string `json:"pages"`
	PDF     string   `json:"pdf"`
	DPI     int      `json:"dpi"`
	Source  string   `json:"source"`
	Paper   string   `json:"paper"`
	Mode    string   `json:"mode"`
	Partial bool     `json:"partial"`
	Bytes   int64    `json:"bytes"`
}
type Job struct {
	Kind       string   `json:"kind"`
	Busy       bool     `json:"busy"`
	Stage      string   `json:"stage"`
	Message    string   `json:"message"`
	Started    string   `json:"started"`
	Ended      string   `json:"ended"`
	Pages      int      `json:"pages"`
	DocumentID string   `json:"documentID"`
	Error      string   `json:"error"`
	Hint       string   `json:"hint"`
	Logs       []string `json:"logs"`
	Dir        string   `json:"dir"`
}
type Connection struct {
	Status       string `json:"status"`
	ScannerState string `json:"scannerState"`
	PaperInADF   string `json:"paperInADF"`
	Checked      string `json:"checked"`
	IP           string `json:"ip"`
}
type appServer struct {
	mu                   sync.Mutex
	settings             Settings
	docs                 []Document
	job                  Job
	connection           Connection
	cancel               context.CancelFunc
	support, token, host string
	closeFn              context.CancelFunc
}

func defaults() Settings {
	home, _ := os.UserHomeDir()
	return Settings{IP: "192.168.5.44", Port: 8289, Output: filepath.Join(home, "Pictures", "M175 Scans"), Source: "platen", Paper: "Letter", DPI: 300, Mode: "color", Format: "pdf", Prefix: "Scan"}
}
func cleanSettings(s Settings) (Settings, error) {
	s.IP = strings.TrimSpace(s.IP)
	ip := net.ParseIP(s.IP)
	if ip == nil || (!ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast()) {
		return s, errors.New("Enter a local network IP address, such as 192.168.5.44")
	}
	if s.Port < 1 || s.Port > 65535 {
		return s, errors.New("Port must be between 1 and 65535")
	}
	if s.Source != "platen" && s.Source != "adf" {
		return s, errors.New("Invalid scan source")
	}
	if s.Paper != "Letter" && s.Paper != "A4" {
		return s, errors.New("Invalid paper size")
	}
	if s.DPI != 150 && s.DPI != 200 && s.DPI != 300 && s.DPI != 600 {
		return s, errors.New("Invalid resolution")
	}
	if s.Mode != "color" && s.Mode != "gray" {
		return s, errors.New("Invalid color mode")
	}
	if s.Format != "pdf" && s.Format != "jpeg" {
		return s, errors.New("Invalid output format")
	}
	if s.Format == "jpeg" && s.Mode == "gray" {
		return s, errors.New("Select PDF for grayscale output; original JPEGs remain in color")
	}
	s.Output = strings.TrimSpace(s.Output)
	if strings.HasPrefix(s.Output, "~/") {
		home, _ := os.UserHomeDir()
		s.Output = filepath.Join(home, s.Output[2:])
	}
	if !filepath.IsAbs(s.Output) {
		return s, errors.New("Output must be an absolute path or start with ~/")
	}
	s.Output = filepath.Clean(s.Output)
	if s.Output == string(filepath.Separator) {
		return s, errors.New("The system root cannot be used as the output folder")
	}
	s.Prefix = strings.TrimSpace(s.Prefix)
	if s.Prefix == "" {
		s.Prefix = "Scan"
	}
	if len([]rune(s.Prefix)) > 60 || strings.ContainsAny(s.Prefix, "/\\:\x00\r\n") || s.Prefix == "." || s.Prefix == ".." {
		return s, errors.New("Filename prefix must not contain slashes, colons or newlines; maximum 60 characters")
	}
	return s, nil
}
func (s Settings) core() config {
	return config{IP: s.IP, Port: s.Port, Output: s.Output, Source: s.Source, Paper: s.Paper, DPI: s.DPI, Mode: s.Mode, Format: s.Format}
}
func atomicJSON(path string, value any) error {
	raw, e := json.MarshalIndent(value, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".m175-write-")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, e = f.Write(raw); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(tmp, path)
}
func loadServer(support string) (*appServer, error) {
	if e := os.MkdirAll(support, 0700); e != nil {
		return nil, e
	}
	a := &appServer{settings: defaults(), support: support, docs: []Document{}, job: Job{Stage: "idle", Message: "Place your document to begin", Logs: []string{}}, connection: Connection{Status: "unknown"}}
	if b, e := os.ReadFile(filepath.Join(support, "settings.json")); e == nil {
		var ss Settings
		if json.Unmarshal(b, &ss) == nil {
			if ss, e = cleanSettings(ss); e == nil {
				a.settings = ss
			}
		}
	}
	a.docs = listDocuments(a.settings.Output)
	return a, nil
}

func listDocuments(root string) []Document {
	entries, e := os.ReadDir(root)
	if e != nil {
		return []Document{}
	}
	var docs []Document
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.HasPrefix(entry.Name(), "scan-") {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		pages, _ := filepath.Glob(filepath.Join(dir, "page-*.jpg"))
		if len(pages) == 0 {
			continue
		}
		info, e := entry.Info()
		if e != nil {
			continue
		}
		doc := Document{ID: entry.Name(), Title: "Scan - " + info.ModTime().Format("Jan 02 15:04"), Created: info.ModTime().Format(time.RFC3339), DPI: 300, Source: "platen", Paper: "Letter", Mode: "color", Pages: []string{}}
		// Metadata is local application data. Paths are never trusted from metadata.
		if b, e := os.ReadFile(filepath.Join(dir, "m175-studio.json")); e == nil {
			var m Document
			if json.Unmarshal(b, &m) == nil {
				doc.Title = m.Title
				doc.Created = m.Created
				doc.DPI = m.DPI
				doc.Source = m.Source
				doc.Paper = m.Paper
				doc.Mode = m.Mode
				doc.Partial = m.Partial
			}
		}
		for _, p := range pages {
			st, e := os.Lstat(p)
			if e != nil || !st.Mode().IsRegular() {
				continue
			}
			name := filepath.Base(p)
			if doc.Mode == "gray" {
				if st, e := os.Lstat(filepath.Join(dir, "gray-"+name)); e == nil && st.Mode().IsRegular() {
					name = "gray-" + name
				}
			}
			doc.Pages = append(doc.Pages, name)
			doc.Bytes += st.Size()
		}
		if len(doc.Pages) == 0 {
			continue
		}
		if pf, _ := filepath.Glob(filepath.Join(dir, "*.pdf")); len(pf) > 0 {
			sort.Strings(pf)
			for _, p := range pf {
				if st, e := os.Lstat(p); e == nil && st.Mode().IsRegular() {
					doc.PDF = filepath.Base(p)
					doc.Bytes += st.Size()
					if strings.HasPrefix(doc.PDF, "partial-") {
						doc.Partial = true
					}
					break
				}
			}
		}
		docs = append(docs, doc)
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].Created > docs[j].Created })
	if len(docs) > 200 {
		docs = docs[:200]
	}
	if docs == nil {
		return []Document{}
	}
	return docs
}
func (a *appServer) snapshot() any {
	a.mu.Lock()
	defer a.mu.Unlock()
	job := a.job
	job.Logs = append([]string{}, a.job.Logs...)
	docs := append([]Document{}, a.docs...)
	return struct {
		Version    string     `json:"version"`
		Settings   Settings   `json:"settings"`
		Job        Job        `json:"job"`
		Connection Connection `json:"connection"`
		Documents  []Document `json:"documents"`
		Arch       string     `json:"arch"`
	}{version, a.settings, job, a.connection, docs, runtime.GOARCH}
}
func (a *appServer) Write(b []byte) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line == "" {
			continue
		}
		a.job.Logs = append(a.job.Logs, line)
		if len(a.job.Logs) > 500 {
			a.job.Logs = a.job.Logs[len(a.job.Logs)-500:]
		}
		switch {
		case strings.Contains(line, "GetScannerElements"):
			a.job.Stage = "connecting"
			a.job.Message = "Connecting to scanner..."
		case strings.Contains(line, "ScannerState="):
			value := strings.SplitN(line, "ScannerState=", 2)[1]
			a.connection.ScannerState = strings.Split(value, ";")[0]
			if x := strings.SplitN(line, "PaperInADF=", 2); len(x) == 2 {
				a.connection.PaperInADF = strings.TrimSpace(x[1])
			}
			a.connection.Status = "online"
			a.connection.Checked = time.Now().Format(time.RFC3339)
			a.connection.IP = a.settings.IP
		case strings.Contains(line, "CreateScanJobRequest"):
			a.job.Stage = "preparing"
			a.job.Message = "Connected. Creating scan job..."
		case strings.Contains(line, "JobState="):
			a.job.Stage = "scanning"
			a.job.Message = "Scanner is preparing a page..."
		case strings.Contains(line, "RetrieveImageRequest"):
			a.job.Stage = "receiving"
			a.job.Message = fmt.Sprintf("Scanning and receiving page %d...", a.job.Pages+1)
		case strings.Contains(line, "Saved "):
			a.job.Pages++
			a.job.Stage = "saving"
			a.job.Message = fmt.Sprintf("Received %d pages. Saving...", a.job.Pages)
		}
	}
	return len(b), nil
}
func (a *appServer) start(kind string) error {
	a.mu.Lock()
	if a.job.Busy {
		a.mu.Unlock()
		return errors.New("A job is already running. Wait for it to finish")
	}
	ss := a.settings
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.job = Job{Kind: kind, Busy: true, Stage: "connecting", Message: "Connecting to scanner...", Started: time.Now().Format(time.RFC3339), Logs: []string{}}
	a.mu.Unlock()
	go func() {
		cfg := ss.core()
		if kind == "check" {
			cfg.Status = true
			cfg.Output = filepath.Join(a.support, "Connection Checks")
		}
		result, dir, err := runWith(ctx, cfg, a)
		var docID string
		if kind == "scan" && dir != "" {
			pages, _ := filepath.Glob(filepath.Join(dir, "page-*.jpg"))
			if len(pages) > 0 {
				docID = filepath.Base(dir)
				title := ss.Prefix + " · " + time.Now().Format("2006-01-02 15.04.05")
				// Rename a completed PDF to the chosen prefix without overwriting any file.
				if result != "" && strings.HasSuffix(result, ".pdf") {
					name := ss.Prefix + "-" + time.Now().Format("20060102-150405") + ".pdf"
					if err != nil {
						name = "partial-" + name
					}
					dest := filepath.Join(dir, name)
					if _, e := os.Lstat(dest); os.IsNotExist(e) {
						if e = os.Rename(result, dest); e == nil {
							result = dest
						}
					}
				}
				meta := Document{ID: docID, Title: title, Created: time.Now().Format(time.RFC3339), DPI: ss.DPI, Source: ss.Source, Paper: ss.Paper, Mode: ss.Mode, Partial: err != nil}
				if e := atomicJSON(filepath.Join(dir, "m175-studio.json"), meta); e != nil {
					a.Write([]byte("Metadata warning: " + e.Error()))
				}
			}
		}
		docs := listDocuments(ss.Output)
		a.mu.Lock()
		defer a.mu.Unlock()
		cancel()
		a.cancel = nil
		a.docs = docs
		a.job.Busy = false
		a.job.Ended = time.Now().Format(time.RFC3339)
		a.job.Dir = dir
		a.job.DocumentID = docID
		if err != nil {
			a.job.Error = err.Error()
			a.job.Stage = "error"
			a.job.Message = "The job did not finish"
			a.job.Hint = errorHint(err)
			if errors.Is(err, context.Canceled) {
				a.job.Stage = "stopped"
				a.job.Message = "Reception stopped. Received pages have been kept"
			}
			if strings.Contains(err.Error(), "GetScannerElements") {
				a.connection.Status = "error"
				a.connection.Checked = time.Now().Format(time.RFC3339)
			}
		} else if kind == "check" {
			a.job.Stage = "done"
			a.job.Message = "Connected. Scanner responded"
		} else {
			a.job.Stage = "done"
			a.job.Message = fmt.Sprintf("Scan complete - %d pages saved", a.job.Pages)
		}
	}()
	return nil
}
func errorHint(err error) string {
	s := strings.ToLower(err.Error())
	switch {
	case errors.Is(err, context.Canceled):
		return "This only stops reception in the app; the printer may still be scanning. Press Cancel on the printer and wait until it is idle before starting again."
	case strings.Contains(s, "no route to host"), strings.Contains(s, "operation not permitted"), strings.Contains(s, "permission denied"):
		return "Allow M175 Studio in System Settings > Privacy & Security > Local Network. Keep the logs if access is already allowed and the error persists. Also check the printer IP, Wi-Fi and VPN."
	case strings.Contains(s, "no paper"):
		return "No paper detected in the ADF. Load the document feeder or use the flatbed."
	case strings.Contains(s, "timeout"), strings.Contains(s, "deadline"):
		return "Connection or read timed out. Wake the printer, check its IP and make sure both devices are on the same local network. If scanning has started, press Cancel on the printer before trying again."
	case strings.Contains(s, "refused"):
		return "The scan port refused the connection. Check the address and port 8289, wake the printer and try again."
	default:
		return "Received original JPEGs will be kept. View the full log in Diagnostics and wait until the printer is idle before scanning again."
	}
}
func grayPages(pages []string) ([]string, error) {
	out := []string{}
	for _, p := range pages {
		f, e := os.Open(p)
		if e != nil {
			return out, e
		}
		im, e := jpeg.Decode(f)
		f.Close()
		if e != nil {
			return out, e
		}
		g := image.NewGray(im.Bounds())
		draw.Draw(g, g.Bounds(), im, im.Bounds().Min, draw.Src)
		path := filepath.Join(filepath.Dir(p), "gray-"+filepath.Base(p))
		var b bytes.Buffer
		if e = jpeg.Encode(&b, g, &jpeg.Options{Quality: 95}); e != nil {
			return out, e
		}
		if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
			return out, e
		}
		out = append(out, path)
	}
	return out, nil
}

func jsonOut(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	jsonOut(w, map[string]string{"error": err.Error()})
}
func decode(r *http.Request, v any) error {
	d := json.NewDecoder(io.LimitReader(r.Body, 65537))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return errors.New("Invalid request format")
	}
	var extra any
	if e := d.Decode(&extra); e != io.EOF {
		return errors.New("Invalid request format")
	}
	return nil
}
func (a *appServer) route(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; frame-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'; form-action 'none'")
	// Host + unguessable session URL + Origin checks defend the local service.
	if r.Host != a.host {
		http.Error(w, "Invalid Host", 403)
		return
	}
	base := "/s/" + a.token + "/"
	if !strings.HasPrefix(r.URL.Path, base) {
		http.NotFound(w, r)
		return
	}
	rel := strings.TrimPrefix(r.URL.Path, base)
	if r.Method != "GET" && r.Method != "HEAD" {
		origin := r.Header.Get("Origin")
		if origin != "" && origin != "http://"+a.host {
			http.Error(w, "Origin refused", 403)
			return
		}
		if r.Header.Get("X-M175-Request") != "1" {
			http.Error(w, "Missing request guard", 403)
			return
		}
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", 405)
			return
		}
	}
	if rel == "api/state" && r.Method == "GET" {
		jsonOut(w, a.snapshot())
		return
	}
	if strings.HasPrefix(rel, "asset/") && r.Method == "GET" {
		p := strings.Split(strings.TrimPrefix(rel, "asset/"), "/")
		if len(p) != 2 {
			http.NotFound(w, r)
			return
		}
		path, e := a.documentAsset(p[0], p[1])
		if e != nil {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, path)
		return
	}
	if r.Method == "POST" {
		switch rel {
		case "api/settings":
			var ss Settings
			if e := decode(r, &ss); e != nil {
				fail(w, 400, e)
				return
			}
			ss, e := cleanSettings(ss)
			if e != nil {
				fail(w, 400, e)
				return
			}
			a.mu.Lock()
			if a.job.Busy {
				a.mu.Unlock()
				fail(w, 409, errors.New("Settings cannot be changed while a job is running"))
				return
			}
			e = atomicJSON(filepath.Join(a.support, "settings.json"), ss)
			if e == nil {
				if a.settings.IP != ss.IP || a.settings.Port != ss.Port {
					a.connection = Connection{Status: "unknown"}
				}
				a.settings = ss
				a.docs = listDocuments(ss.Output)
			}
			a.mu.Unlock()
			if e != nil {
				fail(w, 500, e)
				return
			}
			jsonOut(w, map[string]bool{"ok": true})
			return
		case "api/scan", "api/check":
			kind := "scan"
			if rel == "api/check" {
				kind = "check"
			}
			if e := a.start(kind); e != nil {
				fail(w, 409, e)
				return
			}
			jsonOut(w, map[string]bool{"ok": true})
			return
		case "api/cancel":
			a.mu.Lock()
			if a.cancel != nil {
				a.cancel()
			}
			a.mu.Unlock()
			jsonOut(w, map[string]bool{"ok": true})
			return
		case "api/refresh":
			a.mu.Lock()
			a.docs = listDocuments(a.settings.Output)
			a.mu.Unlock()
			jsonOut(w, map[string]bool{"ok": true})
			return
		case "api/open":
			var req struct {
				ID     string `json:"id"`
				Action string `json:"action"`
			}
			if e := decode(r, &req); e != nil {
				fail(w, 400, e)
				return
			}
			path := ""
			reveal := false
			if req.Action == "output" {
				a.mu.Lock()
				path = a.settings.Output
				a.mu.Unlock()
				_ = os.MkdirAll(path, 0700)
			} else if req.Action == "diagnostics" {
				a.mu.Lock()
				path = a.job.Dir
				a.mu.Unlock()
				if path != "" {
					path = filepath.Join(path, "diagnostics")
				}
			} else {
				a.mu.Lock()
				var doc *Document
				for i := range a.docs {
					if a.docs[i].ID == req.ID {
						d := a.docs[i]
						doc = &d
						break
					}
				}
				a.mu.Unlock()
				if doc == nil {
					fail(w, 404, errors.New("Scan record not found"))
					return
				}
				name := doc.PDF
				if name == "" && len(doc.Pages) > 0 {
					name = doc.Pages[0]
				}
				var e error
				path, e = a.documentAsset(req.ID, name)
				if e != nil {
					fail(w, 404, e)
					return
				}
				reveal = req.Action == "reveal"
			}
			if path == "" {
				fail(w, 400, errors.New("No result is available to open"))
				return
			}
			if runtime.GOOS == "darwin" {
				args := []string{path}
				if reveal {
					args = []string{"-R", path}
				}
				if e := exec.Command("/usr/bin/open", args...).Run(); e != nil {
					fail(w, 500, e)
					return
				}
			}
			jsonOut(w, map[string]bool{"ok": true})
			return
		case "api/shutdown":
			if a.closeFn != nil {
				go a.closeFn()
			}
			jsonOut(w, map[string]bool{"ok": true})
			return
		}
		http.NotFound(w, r)
		return
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	if rel == "" {
		rel = "index.html"
	}
	if rel != "index.html" && rel != "style.css" && rel != "app.js" {
		http.NotFound(w, r)
		return
	}
	content, e := fs.ReadFile(assets, "ui/"+rel)
	if e != nil {
		http.NotFound(w, r)
		return
	}
	switch rel {
	case "index.html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case "style.css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case "app.js":
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	}
	w.Write(content)
}
func (a *appServer) documentAsset(id, name string) (string, error) {
	a.mu.Lock()
	root := a.settings.Output
	var doc *Document
	for i := range a.docs {
		if a.docs[i].ID == id {
			d := a.docs[i]
			doc = &d
			break
		}
	}
	a.mu.Unlock()
	if doc == nil || filepath.Base(id) != id || filepath.Base(name) != name {
		return "", errors.New("File does not exist")
	}
	allowed := name == doc.PDF && name != ""
	for _, p := range doc.Pages {
		if p == name {
			allowed = true
		}
	}
	if !allowed {
		return "", errors.New("Access to this file is not allowed")
	}
	path := filepath.Join(root, id, name)
	rootReal, e := filepath.EvalSymlinks(root)
	if e != nil {
		return "", e
	}
	real, e := filepath.EvalSymlinks(path)
	if e != nil {
		return "", e
	}
	rel, e := filepath.Rel(rootReal, real)
	if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("File is outside the output folder")
	}
	if st, e := os.Stat(real); e != nil || !st.Mode().IsRegular() {
		return "", errors.New("Invalid file")
	}
	return real, nil
}

func main() {
	ready := flag.String("ready-file", "", "write loopback UI URL to this private file")
	supportFlag := flag.String("support-dir", "", "settings directory (testing override)")
	watch := flag.Bool("watch-stdin", false, "exit when native parent closes its pipe")
	ver := flag.Bool("version", false, "print version")
	flag.Parse()
	if *ver {
		fmt.Println("M175 Studio", version)
		return
	}
	home, _ := os.UserHomeDir()
	support := *supportFlag
	if support == "" {
		support = filepath.Join(home, "Library", "Application Support", "M175 Studio")
	}
	a, e := loadServer(support)
	if e != nil {
		log.Fatal(e)
	}
	lock, e := os.OpenFile(filepath.Join(support, "instance.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		log.Fatal(e)
	}
	defer lock.Close()
	if e = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		log.Fatal("M175 Studio is already running. Close the other instance and try again")
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	var token [24]byte
	if _, e = rand.Read(token[:]); e != nil {
		log.Fatal(e)
	}
	a.token = hex.EncodeToString(token[:])
	listener, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		log.Fatal(e)
	}
	a.host = listener.Addr().String()
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	a.closeFn = cancel
	server := &http.Server{Handler: http.HandlerFunc(a.route), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16384}
	if *watch {
		go func() { _, _ = io.Copy(io.Discard, os.Stdin); cancel() }()
	}
	go func() {
		<-ctx.Done()
		a.mu.Lock()
		if a.cancel != nil {
			a.cancel()
		}
		a.mu.Unlock()
		c, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(c)
	}()
	url := "http://" + a.host + "/s/" + a.token + "/"
	if *ready != "" {
		if e = atomicJSON(*ready, map[string]string{"url": url, "version": version}); e != nil {
			log.Fatal(e)
		}
		defer os.Remove(*ready)
	} else {
		fmt.Println(url)
	}
	if e = server.Serve(listener); e != nil && !errors.Is(e, http.ErrServerClosed) {
		log.Fatal(e)
	}
}

// Compile-time references ensure these packages stay audited and standard-library-only.
var _ color.Model = color.GrayModel
var _ = strconv.Itoa
