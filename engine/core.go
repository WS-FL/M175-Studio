// M175 Scan: a local-only experimental scanner client.
// Protocol reference (not an upstream binary or HP product):
// https://gitlab.com/sijisu/hpsimplescan (SOAP operation names and wire fields).
// Only the Go standard library is used. See README.md for test limitations.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"image/color"
	"image/jpeg"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const version = "0.2.2 beta (English)"
const scanNS = "http://tempuri.org/wscn.xsd"
const maxResponse = 128 << 20

// Keep local defaults explicit; no discovery, cloud services, proxy, or listener.
type config struct {
	IP, Source, Paper, Output string
	Mode, Format              string
	Port, DPI                 int
	Status                    bool
}
type client struct {
	endpoint, dir string
	http          *http.Client
	logger        *log.Logger
	request       int
	ctx           context.Context
}
type soapError struct {
	Status int
	Text   string
}

func (e *soapError) Error() string { return fmt.Sprintf("HTTP %d: %s", e.Status, e.Text) }

func envelope(body string) string {
	return `<SOAP-ENV:Envelope xmlns:SOAP-ENV="http://www.w3.org/2003/05/soap-envelope" xmlns:SOAP-ENC="http://www.w3.org/2003/05/soap-encoding" xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" SOAP-ENV:encodingStyle="http://www.w3.org/2003/05/soap-encoding"><SOAP-ENV:Body>` + body + `</SOAP-ENV:Body></SOAP-ENV:Envelope>`
}
func escape(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
func typed(name, typ, value string) string {
	return `<` + name + ` xsi:type="` + typ + `">` + escape(value) + `</` + name + `>`
}
func group(name, typ, body string) string {
	return `<` + name + ` xmlns="` + scanNS + `" xsi:type="` + typ + `">` + body + `</` + name + `>`
}
func integer(name string, n int) string { return typed(name, "xsd:int", strconv.Itoa(n)) }

// Namespace-aware XML parser. Do not guess JobId/JobToken from incomplete XML.
func fields(data []byte) (map[string][]string, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	type node struct {
		name string
		text strings.Builder
	}
	stack := []*node{}
	out := map[string][]string{}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			stack = append(stack, &node{name: t.Name.Local})
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].text.Write(t)
			}
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, errors.New("unexpected XML closing tag")
			}
			n := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			out[n.name] = append(out[n.name], strings.TrimSpace(n.text.String()))
		}
	}
	if len(stack) != 0 || len(out) == 0 {
		return nil, errors.New("incomplete or empty XML")
	}
	return out, nil
}
func first(m map[string][]string, key string) string {
	if len(m[key]) > 0 {
		return m[key][0]
	}
	return ""
}

func (c *client) post(op, body string) ([]byte, string, error) {
	c.request++
	prefix := fmt.Sprintf("%03d-%s", c.request, op)
	reqBody := []byte(envelope(body))
	if err := os.WriteFile(filepath.Join(c.dir, prefix+"-request.xml"), reqBody, 0600); err != nil {
		return nil, "", err
	}
	c.logger.Printf("%s", op)
	ctx := c.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("User-Agent", "M175-Local-Scan/0.1")
	req.Close = true
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("%s: %w", op, err)
	}
	defer resp.Body.Close()
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	ct := resp.Header.Get("Content-Type")
	c.logger.Printf("HTTP %d; %s; %d bytes", resp.StatusCode, ct, len(raw))
	// Diagnostics intentionally exclude successful image bodies (document privacy).
	if strings.Contains(strings.ToLower(ct), "xml") || resp.StatusCode >= 400 || (len(raw) > 0 && raw[0] == '<') {
		if saveErr := os.WriteFile(filepath.Join(c.dir, prefix+"-response.xml"), raw, 0600); saveErr != nil {
			return nil, ct, saveErr
		}
	}
	if readErr != nil {
		return nil, ct, fmt.Errorf("%s response incomplete: %w", op, readErr)
	}
	if len(raw) > maxResponse {
		return nil, ct, errors.New("response exceeds 128 MiB safety limit")
	}
	f, xmlErr := fields(raw)
	_, fault := f["Fault"]
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || (xmlErr == nil && fault) {
		msg := strings.Join(f["Value"], " / ") + " " + strings.Join(f["Text"], " / ")
		if strings.TrimSpace(msg) == "" {
			msg = fmt.Sprintf("unexpected response; see %s-response.xml", prefix)
		}
		return nil, ct, &soapError{resp.StatusCode, strings.TrimSpace(msg)}
	}
	return raw, ct, nil
}

func (c *client) status() (map[string][]string, error) {
	raw, _, err := c.post("GetScannerElements", `<GetScannerElements xmlns="`+scanNS+`"/>`)
	if err != nil {
		return nil, err
	}
	f, err := fields(raw)
	if err != nil {
		return nil, fmt.Errorf("scanner configuration XML is incomplete/invalid: %w; complete received bytes saved in diagnostics", err)
	}
	if _, ok := f["ScanElements"]; !ok {
		return nil, errors.New("response does not contain ScanElements")
	}
	c.logger.Printf("ScannerState=%s; PaperInADF=%s", first(f, "ScannerState"), first(f, "PaperInADF"))
	return f, nil
}

func jobRequest(cfg config) (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	id[6] = (id[6] & 15) | 64
	id[8] = (id[8] & 63) | 128
	uuid := fmt.Sprintf("%x-%x-%x-%x-%x", id[0:4], id[4:6], id[6:8], id[8:10], id[10:16])
	w, h := 8500, 11000
	if cfg.Paper == "A4" {
		w, h = 8270, 11690
	}
	src := "Platen"
	if cfg.Source == "adf" {
		src = "ADF"
	}
	desc := group("JobDescription", "JobDescriptionType", typed("JobOriginatingUserName", "xsd:string", "Local user")+typed("JobName", "xsd:string", "M175 local scan"))
	exposure := group("Exposure", "ScanExposureType", typed("AutoExposure", "xsd:boolean", "true")+group("ExposureSettings", "ExposureSettingsOverrideType", integer("Contrast", 0)))
	// Retain the reference client's maximum flatbed input size; select the crop separately.
	input := group("InputSize", "DocumentInputSizeType", group("InputMediaSize", "DimensionsType", integer("Width", 8500)+integer("Height", 11690)))
	region := integer("ScanRegionXOffset", 0) + integer("ScanRegionYOffset", 0) + integer("ScanRegionHeight", h) + integer("ScanRegionWidth", w)
	front := group("Resolution", "MediaSideOverrideType", integer("Width", cfg.DPI)+integer("Height", cfg.DPI)) + group("ScanRegion", "ScanRegionType", region) + typed("ColorProcessing", "ColorEntryType", "RGB24")
	media := group("MediaSides", "MediaSideOverrideType", group("MediaFront", "MediaSideOverrideType", front))
	params := group("Format", "ScanDocumentFormat", "jfif") + `<ImagesToTransfer>0</ImagesToTransfer>` + exposure + typed("ContentType", "ContentType", "Auto") + integer("CompressionQualityFactor", 0) + typed("InputSource", "ScanInputSource", src) + input + media
	ticket := group("ScanTicket", "ScanTicketType", desc+group("DocumentParameters", "DocumentParametersType", params)+integer("RetrieveImageTimeout", 1800))
	return `<m:CreateScanJobRequest xmlns:m="` + scanNS + `">` + typed("ScanIdentifier", "xsd:string", uuid) + ticket + `</m:CreateScanJobRequest>`, nil
}

// DIME: 12-byte big-endian headers; each field padded to a 4-byte boundary.
// Reassemble chunked JPEG records, not a byte search through mixed SOAP/image data.
func extractJPEG(data []byte) ([]byte, error) {
	if bytes.HasPrefix(data, []byte{0xff, 0xd8}) {
		return validateJPEG(data)
	}
	pos := 0
	records := 0
	ended := false
	continuation := false
	isImage := false
	var image bytes.Buffer
	for pos < len(data) {
		if len(data)-pos < 12 {
			return nil, errors.New("truncated DIME header")
		}
		h := data[pos : pos+12]
		pos += 12
		records++
		if h[0]>>3 != 1 {
			return nil, fmt.Errorf("unsupported DIME version %d", h[0]>>3)
		}
		if records == 1 && h[0]&4 == 0 {
			return nil, errors.New("DIME message-begin flag missing")
		}
		if records > 1 && h[0]&4 != 0 {
			return nil, errors.New("unexpected DIME message-begin flag")
		}
		optionLen := int(binary.BigEndian.Uint16(h[2:4]))
		idLen := int(binary.BigEndian.Uint16(h[4:6]))
		typeLen := int(binary.BigEndian.Uint16(h[6:8]))
		dataLen := int(binary.BigEndian.Uint32(h[8:12]))
		readField := func(n int) ([]byte, error) {
			pad := (n + 3) &^ 3
			if n < 0 || pad > len(data)-pos {
				return nil, errors.New("truncated DIME record")
			}
			v := data[pos : pos+n]
			pos += pad
			return v, nil
		}
		if _, err := readField(optionLen); err != nil {
			return nil, err
		}
		if _, err := readField(idLen); err != nil {
			return nil, err
		}
		typ, err := readField(typeLen)
		if err != nil {
			return nil, err
		}
		payload, err := readField(dataLen)
		if err != nil {
			return nil, err
		}
		if !continuation {
			t := strings.ToLower(string(typ))
			isImage = strings.Contains(t, "jpeg") || strings.Contains(t, "jpg") || bytes.HasPrefix(payload, []byte{0xff, 0xd8})
			if !isImage && (strings.Contains(t, "xml") || bytes.HasPrefix(payload, []byte("<"))) {
				if f, e := fields(payload); e == nil {
					if _, ok := f["Fault"]; ok {
						return nil, &soapError{200, strings.Join(f["Value"], " / ") + " " + strings.Join(f["Text"], " / ")}
					}
				}
			}
		}
		if isImage {
			image.Write(payload)
		}
		continuation = h[0]&1 != 0
		if h[0]&2 != 0 {
			if continuation {
				return nil, errors.New("DIME ends with an unfinished chunk")
			}
			ended = true
			break
		}
	}
	if !ended {
		return nil, errors.New("DIME message-end flag missing")
	}
	if pos != len(data) {
		return nil, errors.New("unexpected trailing bytes after DIME")
	}
	if image.Len() == 0 {
		return nil, errors.New("no JPEG image in response")
	}
	return validateJPEG(image.Bytes())
}
func validateJPEG(data []byte) ([]byte, error) {
	if len(data) < 4 || !bytes.HasPrefix(data, []byte{0xff, 0xd8}) || !bytes.HasSuffix(data, []byte{0xff, 0xd9}) {
		return nil, errors.New("JPEG markers missing: image may be truncated")
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("invalid JPEG: %w", err)
	}
	if cfg.Width < 1 || cfg.Height < 1 || int64(cfg.Width)*int64(cfg.Height) > 150_000_000 {
		return nil, errors.New("unreasonable JPEG dimensions")
	}
	if _, err = jpeg.Decode(bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("JPEG data incomplete: %w", err)
	}
	return data, nil
}

func (c *client) pause(d time.Duration) error {
	ctx := c.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *client) scan(cfg config) ([]string, error) {
	f, err := c.status()
	if err != nil {
		return nil, err
	}
	if cfg.Source == "adf" && strings.EqualFold(first(f, "PaperInADF"), "false") {
		return nil, errors.New("ADF reports no paper; load the feeder before scanning")
	}
	body, err := jobRequest(cfg)
	if err != nil {
		return nil, err
	}
	raw, _, err := c.post("CreateScanJobRequest", body)
	if err != nil {
		return nil, err
	}
	f, err = fields(raw)
	if err != nil {
		return nil, err
	}
	id, token := first(f, "JobId"), first(f, "JobToken")
	if id == "" || token == "" {
		return nil, errors.New("CreateScanJob response has no JobId/JobToken; see diagnostics")
	}
	if _, err = strconv.Atoi(id); err != nil {
		return nil, errors.New("invalid JobId")
	}
	var pages []string
	for page := 1; page <= 100; page++ {
		if err := c.pause(time.Second); err != nil {
			return pages, err
		}
		body = `<m:GetJobInfo xmlns:m="` + scanNS + `">` + typed("jobId", "xsd:String", id) + `</m:GetJobInfo>`
		raw, _, err = c.post("GetJobInfo", body)
		if err != nil {
			return pages, err
		}
		f, err = fields(raw)
		if err != nil {
			return pages, err
		}
		state := first(f, "JobState")
		c.logger.Printf("Page %d; JobState=%s", page, state)
		if strings.EqualFold(state, "Canceled") || strings.EqualFold(state, "Aborted") {
			return pages, fmt.Errorf("scan job %s: %s", state, first(f, "JobStateReasons"))
		}
		// Match the reference ADF workflow. Single-page retrieval also accepts Completed.
		if cfg.Source == "adf" && len(pages) > 0 && state != "Pending" && state != "Processing" {
			if !strings.EqualFold(state, "Completed") {
				return pages, fmt.Errorf("unexpected ADF final state %q", state)
			}
			break
		}
		if err := c.pause(time.Second); err != nil {
			return pages, err
		}
		body = `<m:RetrieveImageRequest xmlns:m="` + scanNS + `">` + typed("JobId", "xsd:int", id) + typed("JobToken", "xsd:string", token) + `</m:RetrieveImageRequest>`
		raw, _, err = c.post("RetrieveImageRequest", body)
		if err != nil {
			if cfg.Source == "adf" && len(pages) > 0 && strings.Contains(err.Error(), "ClientErrorNoImagesAvailable") {
				break
			}
			return pages, err
		}
		image, err := extractJPEG(raw)
		if err != nil {
			if cfg.Source == "adf" && len(pages) > 0 && strings.Contains(err.Error(), "ClientErrorNoImagesAvailable") {
				break
			}
			return pages, err
		}
		dest := filepath.Join(filepath.Dir(c.dir), fmt.Sprintf("page-%03d.jpg", page))
		if err = os.WriteFile(dest, image, 0600); err != nil {
			return pages, err
		}
		pages = append(pages, dest)
		c.logger.Printf("Saved %s", dest)
		if cfg.Source == "platen" {
			break
		}
		if page == 100 {
			return pages, errors.New("stopped at the 100-page safety limit; check the scanner")
		}
	}
	if len(pages) == 0 {
		return nil, errors.New("no pages returned")
	}
	return pages, nil
}

// Lossless JPEG embedding in a simple image-only PDF. No OCR, annotations, watermarks,
// JavaScript, external references, extra compression, or fonts are embedded.
func writePDF(path string, pages []string, dpi int) error {
	if dpi < 1 || dpi > 2400 {
		return errors.New("invalid PDF DPI")
	}
	if len(pages) == 0 {
		return errors.New("no pages for PDF")
	}
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	success := false
	defer func() {
		_ = out.Close()
		if !success {
			_ = os.Remove(path)
		}
	}()
	if _, err = io.WriteString(out, "%PDF-1.4\n%\xe2\xe3\xcf\xd3\n"); err != nil {
		return err
	}
	offsets := make([]int64, 3+3*len(pages))
	obj := func(n int, body []byte) error {
		offset, e := out.Seek(0, io.SeekCurrent)
		if e != nil {
			return e
		}
		offsets[n] = offset
		if _, e = fmt.Fprintf(out, "%d 0 obj\n", n); e != nil {
			return e
		}
		if _, e = out.Write(body); e != nil {
			return e
		}
		_, e = io.WriteString(out, "\nendobj\n")
		return e
	}
	stream := func(n int, dict string, data []byte) error {
		var b bytes.Buffer
		fmt.Fprintf(&b, "<< %s /Length %d >>\nstream\n", dict, len(data))
		b.Write(data)
		b.WriteString("\nendstream")
		return obj(n, b.Bytes())
	}
	if err = obj(1, []byte("<< /Type /Catalog /Pages 2 0 R >>")); err != nil {
		return err
	}
	var kids strings.Builder
	for i := range pages {
		fmt.Fprintf(&kids, "%d 0 R ", 3+i*3)
	}
	if err = obj(2, []byte(fmt.Sprintf("<< /Type /Pages /Count %d /Kids [ %s] >>", len(pages), kids.String()))); err != nil {
		return err
	}
	for i, p := range pages {
		img, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		cfg, e := jpeg.DecodeConfig(bytes.NewReader(img))
		if e != nil {
			return e
		}
		width, height := float64(cfg.Width)*72/float64(dpi), float64(cfg.Height)*72/float64(dpi)
		page, image, content := 3+3*i, 4+3*i, 5+3*i
		desc := fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.4f %.4f] /Resources << /XObject << /Im0 %d 0 R >> >> /Contents %d 0 R >>", width, height, image, content)
		if err = obj(page, []byte(desc)); err != nil {
			return err
		}
		space := "/DeviceRGB"
		if cfg.ColorModel == color.GrayModel {
			space = "/DeviceGray"
		}
		if cfg.ColorModel == color.CMYKModel {
			return errors.New("CMYK JPEG is not supported by this PDF writer")
		}
		dict := fmt.Sprintf("/Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace %s /BitsPerComponent 8 /Filter /DCTDecode", cfg.Width, cfg.Height, space)
		if err = stream(image, dict, img); err != nil {
			return err
		}
		if err = stream(content, "", []byte(fmt.Sprintf("q\n%.4f 0 0 %.4f 0 0 cm\n/Im0 Do\nQ\n", width, height))); err != nil {
			return err
		}
	}
	xref, err := out.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintf(out, "xref\n0 %d\n0000000000 65535 f \n", len(offsets)); err != nil {
		return err
	}
	for _, o := range offsets[1:] {
		if _, err = fmt.Fprintf(out, "%010d 00000 n \n", o); err != nil {
			return err
		}
	}
	if _, err = fmt.Fprintf(out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref); err != nil {
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}
	success = true
	return nil
}

func run(cfg config) (result, runDir string, err error) {
	return runWith(context.Background(), cfg, os.Stderr)
}

func runWith(ctx context.Context, cfg config, progress io.Writer) (result, runDir string, err error) {
	ip := net.ParseIP(cfg.IP)
	if ip == nil || (!ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast()) {
		return "", "", errors.New("printer must be a local/private IP address")
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return "", "", errors.New("invalid port")
	}
	if cfg.Source != "platen" && cfg.Source != "adf" {
		return "", "", errors.New("source must be platen or adf")
	}
	if cfg.Paper != "Letter" && cfg.Paper != "A4" {
		return "", "", errors.New("paper must be Letter or A4")
	}
	if cfg.DPI != 150 && cfg.DPI != 200 && cfg.DPI != 300 && cfg.DPI != 600 {
		return "", "", errors.New("DPI must be 150, 200, 300 or 600")
	}
	if err = os.MkdirAll(cfg.Output, 0700); err != nil {
		return
	}
	runDir, err = os.MkdirTemp(cfg.Output, "scan-"+time.Now().Format("20060102-150405")+"-")
	if err != nil {
		return
	}
	diag := filepath.Join(runDir, "diagnostics")
	if err = os.Mkdir(diag, 0700); err != nil {
		return
	}
	logfile, e := os.OpenFile(filepath.Join(diag, "scan.log"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		err = e
		return
	}
	defer logfile.Close()
	logger := log.New(io.MultiWriter(progress, logfile), "", log.LstdFlags)
	logger.Printf("M175 Scan %s; %s/%s; source=%s paper=%s dpi=%d", version, runtime.GOOS, runtime.GOARCH, cfg.Source, cfg.Paper, cfg.DPI)
	transport := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext, DisableKeepAlives: true, ResponseHeaderTimeout: 180 * time.Second}
	defer transport.CloseIdleConnections()
	hc := &http.Client{Transport: transport, Timeout: 180 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("printer redirect refused") }}
	c := &client{endpoint: "http://" + net.JoinHostPort(cfg.IP, strconv.Itoa(cfg.Port)) + "/", dir: diag, http: hc, logger: logger, ctx: ctx}
	if cfg.Status {
		_, err = c.status()
		result = diag
		return
	}
	pages, scanErr := c.scan(cfg)
	if len(pages) > 0 && cfg.Format != "jpeg" {
		if cfg.Mode == "gray" {
			converted, convErr := grayPages(pages)
			if convErr != nil {
				return "", runDir, fmt.Errorf("Original JPEGs saved; grayscale conversion failed: %w", convErr)
			}
			pages = converted
		}
		result = filepath.Join(runDir, "scan.pdf")
		if scanErr != nil {
			result = filepath.Join(runDir, "partial-scan.pdf")
		}
		err = writePDF(result, pages, cfg.DPI)
		if err != nil {
			err = fmt.Errorf("JPEG pages saved, but PDF conversion failed: %w", err)
			return
		}
	}
	if cfg.Format == "jpeg" {
		result = runDir
	}
	if scanErr != nil {
		err = scanErr
		logger.Printf("ERROR: %v", err)
		return
	}
	logger.Printf("Completed: %d pages; %s", len(pages), result)
	return
}
