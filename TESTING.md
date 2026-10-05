# Verification - English revision

## Previous hardware evidence

The earlier scan core completed a real network flatbed color scan at 300 dpi on an HP LaserJet 100 colorMFP M175nw, saving JPEG and PDF files. This does not establish hardware compatibility for ADF or other combinations.

## Current revision

- `go test -race -count=1 ./...` passed on macOS/arm64 (Go 1.23.2), including flatbed and two-page ADF mock-printer workflows, PDF generation, persistence and request/access checks.
- Built both darwin/arm64 and darwin/amd64 scan engines and a universal AppKit/WKWebView host against the installed Apple SDK.
- Ad-hoc bundle verification passed with `codesign --verify --deep --strict`.
- Started the native English app on a real Apple Silicon Mac using an isolated test profile. Inspected Scan, Files, Settings and Diagnostics; menu and runtime strings were in English. The scan-page promotional heading remained hidden.
- Checked a failed loopback connection and its English job status. No physical printer was contacted during these checks.
- Scanned source text for Chinese characters, private developer paths and common secret markers before publication. The English app is a new source rebuild, not an exact reconstruction of the original Chinese 0.2.2 binary. The promotional scan-page heading remains hidden.

## Known limits

A simulated printer cannot verify physical ADF feeding, scanner resolution support, sleep/wake behavior or real-printer reliability. Ad-hoc signature verification establishes integrity, not Apple trust or notarization. Intel runtime behavior requires an Intel Mac. Native startup checks do not establish successful scanning on a printer.
