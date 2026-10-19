package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

func TestDiscoveryScenarios(t *testing.T) {
	valid := envelope(`<GetScannerElementsResponse xmlns="` + scanNS + `"><ScanElements><ScannerConfiguration/><ScannerStatus><ScannerState>Idle</ScannerState><PaperInADF>false</PaperInADF></ScannerStatus></ScanElements></GetScannerElementsResponse>`)
	for _, tc := range []struct {
		name      string
		bodies    []string
		supported int
	}{
		{"M175", []string{valid}, 1}, {"none", nil, 0}, {"multiple", []string{valid, valid}, 2},
		{"8289-with-wrong-SOAP", []string{envelope(`<OtherResponse/>`)}, 0},
		{"fake-elements-without-namespace", []string{envelope(`<ScanElements><ScannerConfiguration/><ScannerState>Idle</ScannerState></ScanElements>`)}, 0},
		{"HTML", []string{`<html>Printer</html>`}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidates := []Device{}
			var calls atomic.Int32
			for _, body := range tc.bodies {
				body := body
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Test actual request shape: discovery must never start a scan.
					raw := make([]byte, r.ContentLength)
					_, _ = r.Body.Read(raw)
					if !strings.Contains(string(raw), "GetScannerElements") || strings.Contains(string(raw), "CreateScanJob") {
						t.Error("non-read-only discovery")
					}
					fmt.Fprint(w, body)
				}))
				defer server.Close()
				host, port, _ := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
				n, _ := strconv.Atoi(port)
				candidates = append(candidates, Device{IP: host, Port: n, Name: "HP LaserJet M175nw"})
			}
			// Deduplicate services and saved endpoint pointing to the same scanner.
			candidates = append(candidates, candidates...)
			devices := findDevices(context.Background(), candidates, func(ctx context.Context, d Device) (Device, bool) { calls.Add(1); return probeScanner(ctx, d) })
			supported := 0
			for _, d := range devices {
				if d.Supported {
					supported++
				}
			}
			if supported != tc.supported || len(devices) != len(tc.bodies) || int(calls.Load()) != len(tc.bodies) {
				t.Fatalf("devices=%+v, calls=%d", devices, calls.Load())
			}
		})
	}
}

func TestDiscoveryRejectPublicAndCancel(t *testing.T) {
	calls := 0
	probe := func(ctx context.Context, d Device) (Device, bool) { calls++; return d, true }
	devices := findDevices(context.Background(), []Device{{IP: "8.8.8.8", Port: 8289}}, probe)
	if calls != 0 || len(devices) != 0 {
		t.Fatal("public address probed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	findDevices(ctx, []Device{{IP: "192.168.1.2", Port: 8289}}, probe)
	if calls != 0 {
		t.Fatal("probe after cancellation")
	}
}
func TestSelectRememberedDevice(t *testing.T) {
	a, e := loadServer(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	a.discovery.Devices = []Device{{IP: "192.168.1.10", Port: 8289, Supported: true}, {IP: "192.168.1.20", Port: 8289, Supported: true}, {IP: "192.168.1.30", Port: 8289}}
	if e = a.selectDevice("192.168.1.30", 8289); e == nil {
		t.Fatal("unsupported selected")
	}
	if e = a.selectDevice("192.168.1.20", 8289); e != nil {
		t.Fatal(e)
	}
	b, e := loadServer(a.support)
	if e != nil || b.settings.IP != "192.168.1.20" {
		t.Fatal("selection not persisted", e)
	}
}

func TestPreferredReconnect(t *testing.T) {
	devices := []Device{{IP: "192.168.1.10", Port: 8289, Supported: true}, {IP: "192.168.1.20", Port: 8289, Supported: true}, {IP: "192.168.1.30", Port: 8289}}
	if preferredDevice(devices, "192.168.1.20", 8289) != 1 {
		t.Fatal("last selection not preferred")
	}
	if preferredDevice(devices, "192.168.1.30", 8289) != 0 {
		t.Fatal("unsupported saved device preferred")
	}
	if preferredDevice(devices, "192.168.1.99", 8289) != 0 {
		t.Fatal("offline saved device preferred")
	}
	if preferredDevice(nil, "192.168.1.99", 8289) != -1 {
		t.Fatal("false connection")
	}
}
