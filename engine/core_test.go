package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

func sampleJPEG(t *testing.T) []byte {
	t.Helper()
	im := image.NewRGBA(image.Rect(0, 0, 300, 420))
	for y := 0; y < 420; y++ {
		for x := 0; x < 300; x++ {
			c := color.RGBA{240, 240, 240, 255}
			if y < 90 {
				c = color.RGBA{30, 80, 180, 255}
			}
			if x < 20 {
				c = color.RGBA{180, 30, 30, 255}
			}
			im.Set(x, y, c)
		}
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, im, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func record(flags byte, typ string, payload []byte) []byte {
	var b bytes.Buffer
	h := make([]byte, 12)
	h[0] = 8 | flags
	h[1] = 16
	binary.BigEndian.PutUint16(h[6:8], uint16(len(typ)))
	binary.BigEndian.PutUint32(h[8:12], uint32(len(payload)))
	b.Write(h)
	field := func(v []byte) { b.Write(v); b.Write(make([]byte, (4-len(v)%4)%4)) }
	field([]byte(typ))
	field(payload)
	return b.Bytes()
}
func dime(j []byte) []byte {
	b := record(4, "http://www.w3.org/2003/05/soap-envelope", []byte(envelope(`<RetrieveImageRequestResponse><Response href="id1"/></RetrieveImageRequestResponse>`)))
	cut := len(j) / 2
	b = append(b, record(1, "image/jpeg", j[:cut])...)
	return append(b, record(2, "", j[cut:])...)
}
func TestDIMEChunkReassembly(t *testing.T) {
	j := sampleJPEG(t)
	out, err := extractJPEG(dime(j))
	if err != nil || !bytes.Equal(out, j) {
		t.Fatalf("roundtrip: %v", err)
	}
}
func TestDIMERejectTruncated(t *testing.T) {
	b := dime(sampleJPEG(t))
	for _, n := range []int{0, 1, 8, 12, len(b) - 1} {
		if _, err := extractJPEG(b[:n]); err == nil {
			t.Fatalf("accepted %d-byte prefix", n)
		}
	}
}
func TestJPEGRejectTruncated(t *testing.T) {
	j := sampleJPEG(t)
	if _, err := validateJPEG(j[:len(j)-2]); err == nil {
		t.Fatal("accepted truncated JPEG")
	}
}
func TestXMLIncomplete(t *testing.T) {
	if _, err := fields([]byte(`<a><b>BlackandWhite`)); err == nil {
		t.Fatal("accepted incomplete XML")
	}
}
func TestXML202Configuration(t *testing.T) {
	m, err := fields([]byte(envelope(`<wscn:ScanElements xmlns:wscn="` + scanNS + `"><ScannerConfiguration/><ScannerStatus><ScannerState>Idle</ScannerState></ScannerStatus></wscn:ScanElements>`)))
	if err != nil || first(m, "ScannerState") != "Idle" {
		t.Fatal(m, err)
	}
}
func TestJobRequest(t *testing.T) {
	s, err := jobRequest(config{Source: "platen", Paper: "A4", DPI: 300})
	if err != nil {
		t.Fatal(err)
	}
	f, err := fields([]byte(envelope(s)))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]string{"InputSource": "Platen", "Format": "jfif", "ScanRegionHeight": "11690", "ScanRegionWidth": "8270", "ColorProcessing": "RGB24"} {
		if first(f, k) != v {
			t.Fatalf("%s = %s", k, first(f, k))
		}
	}
}
func TestIntegration202(t *testing.T) {
	for _, source := range []string{"platen", "adf"} {
		t.Run(source, func(t *testing.T) {
			var received atomic.Int32
			j := sampleJPEG(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				f, err := fields(body)
				if err != nil {
					t.Errorf("bad request: %v", err)
					w.WriteHeader(400)
					return
				}
				w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
				switch {
				case strings.Contains(string(body), "GetScannerElements"):
					w.WriteHeader(202)
					fmt.Fprint(w, envelope(`<wscn:ScanElements xmlns:wscn="`+scanNS+`"><ScannerConfiguration/><ScannerStatus><ScannerState>Idle</ScannerState><ScanToStatus><PaperInADF>true</PaperInADF></ScanToStatus></ScannerStatus></wscn:ScanElements>`))
				case strings.Contains(string(body), "CreateScanJobRequest"):
					if source == "adf" && first(f, "InputSource") != "ADF" {
						t.Error("wrong input source")
					}
					w.WriteHeader(202)
					fmt.Fprint(w, envelope(`<wscn:CreateScanJobResponseType xmlns:wscn="`+scanNS+`"><JobId>12</JobId><JobToken>token&amp;test</JobToken></wscn:CreateScanJobResponseType>`))
				case strings.Contains(string(body), "GetJobInfo"):
					if first(f, "jobId") != "12" {
						t.Error("wrong lower-case jobId")
					}
					state := "Processing"
					if received.Load() >= 2 {
						state = "Completed"
					}
					w.WriteHeader(202)
					fmt.Fprint(w, envelope(`<JobSummaryType><JobState>`+state+`</JobState></JobSummaryType>`))
				case strings.Contains(string(body), "RetrieveImageRequest"):
					if first(f, "JobToken") != "token&test" {
						t.Error("token not escaped correctly")
					}
					received.Add(1)
					w.Header().Set("Content-Type", "application/dime")
					w.WriteHeader(202)
					w.Write(dime(j))
				default:
					t.Error("unexpected operation")
					w.WriteHeader(500)
				}
			}))
			defer server.Close()
			host, port, _ := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
			p, _ := strconv.Atoi(port)
			result, dir, err := run(config{IP: host, Port: p, Source: source, Paper: "Letter", DPI: 300, Output: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			out, err := os.ReadFile(result)
			if err != nil || !bytes.HasPrefix(out, []byte("%PDF-1.4")) {
				t.Fatal("missing PDF", err)
			}
			count := int32(1)
			if source == "adf" {
				count = 2
			}
			if received.Load() != count {
				t.Fatal("wrong page count")
			}
			diagnostics, _ := filepath.Glob(filepath.Join(dir, "diagnostics", "*-response.xml"))
			if len(diagnostics) < 2 {
				t.Fatal("diagnostics missing")
			}
		})
	}
}
func TestStatusOnlyDoesNotScan(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		b, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(b), "GetScannerElements") {
			t.Error("unexpected scan")
		}
		w.WriteHeader(202)
		fmt.Fprint(w, envelope(`<ScanElements><ScannerConfiguration/></ScanElements>`))
	}))
	defer server.Close()
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	p, _ := strconv.Atoi(port)
	_, _, err := run(config{IP: host, Port: p, Source: "platen", Paper: "Letter", DPI: 300, Output: t.TempDir(), Status: true})
	if err != nil || calls.Load() != 1 {
		t.Fatal(err)
	}
}
func TestRejectSOAPFault202(t *testing.T) {
	d := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(202)
		fmt.Fprint(w, envelope(`<SOAP-ENV:Fault><Code><Value>SOAP-ENV:Sender</Value><Subcode><Value>wscn:ClientErrorNoImagesAvailable</Value></Subcode></Code><Reason><Text>No image</Text></Reason></SOAP-ENV:Fault>`))
	}))
	defer server.Close()
	c := &client{endpoint: server.URL, dir: d, http: server.Client(), logger: testLogger()}
	_, _, err := c.post("test", "<test/>")
	if err == nil || !strings.Contains(err.Error(), "ClientErrorNoImagesAvailable") {
		t.Fatal(err)
	}
}
func TestPDFExample(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "page.jpg")
	if err := os.WriteFile(p, sampleJPEG(t), 0600); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(d, "test.pdf")
	if requested := os.Getenv("M175_TEST_PDF"); requested != "" {
		dest = requested
		_ = os.Remove(dest)
	}
	if err := writePDF(dest, []string{p, p}, 100); err != nil {
		t.Fatal(err)
	}
}

func testLogger() *log.Logger { return log.New(io.Discard, "", 0) }
