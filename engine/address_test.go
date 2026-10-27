package main

import (
	"context"
	"io"
	"net"
	"strings"
	"testing"
)

func TestPrinterAddressValidation(t *testing.T) {
	for _, address := range []string{"198.51.100.6", "192.168.5.44", "10.1.2.3", "172.20.1.2", "169.254.1.2", "127.0.0.1", "::1", "fd00::1", "2001:db8::1"} {
		t.Run(address, func(t *testing.T) {
			s := defaults()
			s.IP = " " + address + " "
			got, err := cleanSettings(s)
			if err != nil || got.IP != address {
				t.Fatalf("settings rejected endpoint: %+v %v", got, err)
			}
			// Check the scan engine's own gate without sending any traffic.
			cfg := got.core()
			cfg.Port = 0
			_, _, err = runWith(context.Background(), cfg, io.Discard)
			if err == nil || err.Error() != "invalid port" {
				t.Fatalf("scan engine rejected IP before port check: %v", err)
			}
		})
	}
	for _, address := range []string{"", "printer.local", "http://198.51.100.6", "198.51.100.6:8289", "999.1.2.3", "0.0.0.0", "0.1.2.3", "224.0.0.1", "240.1.2.3", "255.255.255.255", "::", "ff02::1", "::ffff:224.0.0.1"} {
		if validPrinterIP(net.ParseIP(address)) {
			t.Errorf("accepted non-unicast or invalid address %q", address)
		}
	}
}

func TestExplicitCampusDiscovery(t *testing.T) {
	for _, direct := range []bool{false, true} {
		calls := 0
		candidates := []Device{{IP: "198.51.100.6", Port: 8289, direct: direct}}
		// Even explicit candidates may not target multicast/broadcast or bad ports.
		candidates = append(candidates, Device{IP: "224.0.0.1", Port: 8289, direct: true}, Device{IP: "255.255.255.255", Port: 8289, direct: true}, Device{IP: "198.51.100.6", Port: 0, direct: true})
		devices := findDevices(context.Background(), candidates, func(ctx context.Context, d Device) (Device, bool) {
			calls++
			d.Supported = true
			return d, true
		})
		want := 0
		if direct {
			want = 1
		}
		if calls != want || len(devices) != want {
			t.Fatalf("direct=%v calls=%d devices=%+v", direct, calls, devices)
		}
		if direct && preferredDevice(devices, "198.51.100.6", 8289) != 0 {
			t.Fatal("saved campus endpoint was not preferred")
		}
	}
}

func TestCampusStatusCheckReachesTransport(t *testing.T) {
	// Cancellation lets the real status-check path reach HTTP transport without
	// contacting the network or creating a scan job.
	s := defaults()
	s.IP = "198.51.100.6"
	s.Output = t.TempDir()
	cfg := s.core()
	cfg.Status = true
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := runWith(ctx, cfg, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("expected canceled HTTP request, got %v", err)
	}
}
