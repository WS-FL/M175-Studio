package main

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Device struct {
	IP        string `json:"ip"`
	Port      int    `json:"port"`
	Name      string `json:"name"`
	Supported bool   `json:"supported"`
	State     string `json:"state"`
	ADF       string `json:"adf"`
}
type Discovery struct {
	Busy    bool     `json:"busy"`
	Devices []Device `json:"devices"`
	Message string   `json:"message"`
	Checked string   `json:"checked"`
}

// Discovery never creates a scan job. A listening port alone is not support.
func probeScanner(ctx context.Context, d Device) (Device, bool) {
	endpoint := "http://" + net.JoinHostPort(d.IP, strconv.Itoa(d.Port)) + "/"
	req, e := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(envelope(`<GetScannerElements xmlns="`+scanNS+`"/>`)))
	if e != nil {
		return d, false
	}
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	tr := &http.Transport{Proxy: nil, DisableKeepAlives: true, DialContext: (&net.Dialer{Timeout: 450 * time.Millisecond}).DialContext, ResponseHeaderTimeout: 1200 * time.Millisecond}
	defer tr.CloseIdleConnections()
	hc := &http.Client{Transport: tr, Timeout: 1500 * time.Millisecond, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect refused") }}
	resp, e := hc.Do(req)
	if e != nil {
		return d, false
	}
	defer resp.Body.Close()
	raw, e := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if e != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return d, true
	}
	// Require a SOAP envelope, response from the legacy namespace, ScanElements,
	// ScannerConfiguration, and a nonempty state; reject unrelated SOAP/HTML.
	dec := xml.NewDecoder(bytes.NewReader(raw))
	soap, legacy := false, false
	for {
		tok, e := dec.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return d, true
		}
		if t, ok := tok.(xml.StartElement); ok {
			if t.Name.Local == "Envelope" && (t.Name.Space == "http://www.w3.org/2003/05/soap-envelope" || t.Name.Space == "http://schemas.xmlsoap.org/soap/envelope/") {
				soap = true
			}
			if t.Name.Space == scanNS {
				legacy = true
			}
		}
	}
	f, e := fields(raw)
	if e != nil {
		return d, true
	}
	_, elements := f["ScanElements"]
	_, configuration := f["ScannerConfiguration"]
	_, fault := f["Fault"]
	d.Supported = soap && legacy && elements && configuration && !fault && first(f, "ScannerState") != ""
	if d.Supported {
		d.State = first(f, "ScannerState")
		d.ADF = first(f, "PaperInADF")
		if d.Name == "" {
			d.Name = "HP legacy scanner"
		}
	}
	return d, true
}

// Browse with macOS's Bonjour client. Each one-shot subprocess is bounded;
// dns-sd uses the system multicast responder rather than scanning extra ports.
func dnsOutput(ctx context.Context, args ...string) string {
	cmd := exec.CommandContext(ctx, "/usr/bin/dns-sd", args...)
	var b bytes.Buffer
	cmd.Stdout = &b
	_ = cmd.Run()
	return b.String()
}
func bonjourCandidates(ctx context.Context) []Device {
	services := []string{"_scanner._tcp", "_uscan._tcp", "_uscans._tcp", "_ipp._tcp", "_ipps._tcp", "_http._tcp", "_pdl-datastream._tcp"}
	type instance struct{ name, service string }
	ch := make(chan instance, 224)
	var wg sync.WaitGroup
	for _, service := range services {
		wg.Add(1)
		go func(service string) {
			defer wg.Done()
			c, cancel := context.WithTimeout(ctx, 2200*time.Millisecond)
			defer cancel()
			out := dnsOutput(c, "-B", service, "local.")
			count := 0
			for _, line := range strings.Split(out, "\n") {
				f := strings.Fields(line)
				// Timestamp A/R Flags if Domain Service Instance Name
				if len(f) >= 7 && f[1] == "Add" && strings.TrimSuffix(f[4], ".") == "local" {
					ch <- instance{strings.Join(f[6:], " "), service}
					count++
					if count >= 32 {
						break
					}
				}
			}
		}(service)
	}
	wg.Wait()
	close(ch)
	out := []Device{}
	var mu sync.Mutex
	sem := make(chan struct{}, 8)
	seen := map[string]bool{}
	for i := range ch {
		key := i.service + "/" + i.name
		if seen[key] {
			continue
		}
		seen[key] = true
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			break
		}
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func(i instance) {
			defer wg.Done()
			defer func() { <-sem }()
			c, cancel := context.WithTimeout(ctx, 2200*time.Millisecond)
			defer cancel()
			result := dnsOutput(c, "-L", i.name, i.service, "local.")
			for _, line := range strings.Split(result, "\n") {
				const marker = "can be reached at "
				idx := strings.Index(line, marker)
				if idx < 0 {
					continue
				}
				fields := strings.Fields(line[idx+len(marker):])
				if len(fields) == 0 {
					continue
				}
				host, _, e := net.SplitHostPort(strings.TrimSuffix(fields[0], "."))
				if e != nil {
					continue
				}
				resolveCtx, resolveCancel := context.WithTimeout(ctx, 1500*time.Millisecond)
				ips, e := net.DefaultResolver.LookupIPAddr(resolveCtx, host)
				resolveCancel()
				if e != nil {
					continue
				}
				for _, addr := range ips {
					ip := addr.IP.To4()
					if ip == nil || !(ip.IsPrivate() || ip.IsLinkLocalUnicast()) {
						continue
					}
					mu.Lock()
					out = append(out, Device{IP: ip.String(), Port: 8289, Name: i.name})
					mu.Unlock()
				}
			}
		}(i)
	}
	wg.Wait()
	return out
}

// At most two active physical interfaces, one adjacent /24 per interface even
// on a larger LAN; honor narrower masks. No VPN, public ranges, or IPv6 sweep.
func subnetCandidates() []Device {
	interfaces, _ := net.Interfaces()
	out := []Device{}
	used := map[string]bool{}
	networks := 0
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 || !strings.HasPrefix(iface.Name, "en") {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			n, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := n.IP.To4()
			if ip == nil || !ip.IsPrivate() {
				continue
			}
			prefix := fmt.Sprintf("%d.%d.%d.", ip[0], ip[1], ip[2])
			if used[prefix] {
				continue
			}
			used[prefix] = true
			networks++
			for x := 1; x < 255; x++ {
				candidate := net.ParseIP(prefix + strconv.Itoa(x))
				if n.Contains(candidate) && !candidate.Equal(ip) {
					out = append(out, Device{IP: candidate.String(), Port: 8289})
				}
			}
			if networks >= 2 {
				return out
			}
		}
	}
	return out
}

// Inject candidates/probes in tests; only eight concurrent endpoints, paced
// starts, and one read-only request per endpoint per explicit discovery pass.
func findDevices(ctx context.Context, candidates []Device, probe func(context.Context, Device) (Device, bool)) []Device {
	unique := map[string]Device{}
	for _, d := range candidates {
		ip := net.ParseIP(d.IP)
		if ip == nil || !(ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()) {
			continue
		}
		key := net.JoinHostPort(d.IP, strconv.Itoa(d.Port))
		old := unique[key]
		if old.Name != "" {
			d.Name = old.Name
		}
		unique[key] = d
	}
	keys := make([]string, 0, len(unique))
	for k := range unique {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := []Device{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for _, key := range keys {
		select {
		case <-ctx.Done():
			wg.Wait()
			return out
		case <-ticker.C:
		}
		select {
		case <-ctx.Done():
			wg.Wait()
			return out
		case sem <- struct{}{}:
		}
		d := unique[key]
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			checked, responded := probe(ctx, d)
			if checked.Supported || responded || d.Name != "" {
				if checked.Name == "" {
					checked.Name = "Network device"
				}
				mu.Lock()
				out = append(out, checked)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	sort.Slice(out, func(i, j int) bool {
		if out[i].Supported != out[j].Supported {
			return out[i].Supported
		}
		return out[i].IP < out[j].IP
	})
	return out
}
func (a *appServer) discover() error {
	a.mu.Lock()
	if a.job.Busy || a.discovery.Busy {
		a.mu.Unlock()
		return errors.New("Wait for the current scan or search to finish")
	}
	if a.discovery.Checked != "" {
		if t, e := time.Parse(time.RFC3339, a.discovery.Checked); e == nil && time.Since(t) < 10*time.Second {
			a.mu.Unlock()
			return errors.New("Wait a few seconds before refreshing devices")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	a.discoveryCancel = cancel
	a.discovery.Busy = true
	a.discovery.Message = "Searching…"
	ss := a.settings
	a.mu.Unlock()
	go func() {
		defer cancel()
		candidates := []Device{{IP: ss.IP, Port: ss.Port}}
		candidates = append(candidates, bonjourCandidates(ctx)...)
		candidates = append(candidates, subnetCandidates()...)
		devices := findDevices(ctx, candidates, probeScanner)
		a.mu.Lock()
		defer a.mu.Unlock()
		a.discoveryCancel = nil
		a.discovery = Discovery{Devices: devices, Checked: time.Now().Format(time.RFC3339), Message: "No scanners found. Check Local Network permission or use Settings > Advanced / Manual Connection."}
		// Do not overwrite a manual setting or selected device changed during search.
		if a.job.Busy || a.settings.IP != ss.IP || a.settings.Port != ss.Port {
			return
		}
		a.connection = Connection{Status: "unknown", IP: ss.IP}
		chosen := preferredDevice(devices, ss.IP, ss.Port)
		if chosen >= 0 {
			d := devices[chosen]
			next := a.settings
			next.IP = d.IP
			next.Port = d.Port
			if e := atomicJSON(a.support+"/settings.json", next); e != nil {
				a.discovery.Message = "Scanner found, but connection could not be saved: " + e.Error()
				return
			}
			a.settings = next
			a.connection = Connection{Status: "online", IP: d.IP, ScannerState: d.State, PaperInADF: d.ADF, Checked: a.discovery.Checked}
			a.discovery.Message = "Connected"
		}
	}()
	return nil
}
func (a *appServer) selectDevice(ip string, port int) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.job.Busy || a.discovery.Busy {
		return errors.New("Wait for the current job or search")
	}
	for _, d := range a.discovery.Devices {
		if d.IP == ip && d.Port == port && d.Supported {
			ss := a.settings
			ss.IP = d.IP
			ss.Port = d.Port
			if e := atomicJSON(a.support+"/settings.json", ss); e != nil {
				return e
			}
			a.settings = ss
			a.connection = Connection{Status: "online", IP: d.IP, ScannerState: d.State, PaperInADF: d.ADF, Checked: a.discovery.Checked}
			return nil
		}
	}
	return errors.New("This device has not been confirmed as a supported scanner")
}

func preferredDevice(devices []Device, ip string, port int) int {
	chosen := -1
	for i, d := range devices {
		if d.Supported {
			if chosen < 0 {
				chosen = i
			}
			if d.IP == ip && d.Port == port {
				return i
			}
		}
	}
	return chosen
}
