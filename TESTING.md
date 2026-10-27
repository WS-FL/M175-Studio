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


## 0.3.0 feature tests

Run `go test -race -count=1 ./...` in engine/ for protocol discovery, preferred reconnect, rename and scan regressions. The tests use temporary local HTTP services and isolated document fixtures; they do not contact a physical scanner.

For the added UI harness, install Node and Playwright Chromium, build `build/m175-engine` from engine/, then run `node tests/features-ui.cjs` from the repository root. Playwright must be resolvable by Node (or NODE_PATH). Screenshots and the JSON report are written to ignored build/feature-ui/. Discovery states in the UI harness are explicit fixtures; HTTP/SOAP probes are exercised by the Go tests. `--no-discovery` is a test-only engine flag that prevents LAN traffic during isolated UI runs. Setting an absolute M175_STUDIO_TEST_ROOT for native host testing likewise isolates settings/logs and disables LAN discovery.

Hardware acceptance still requires: grant/deny Local Network permission, find the real M175nw, remember it across relaunch, refresh after disconnect, scan flatbed color 300 dpi, and open/reveal a renamed PDF and its original JPEG pages. Verify Intel execution on an Intel Mac; cross-compilation and signature checks alone do not establish runtime compatibility.


0.3.1 adds long English, Chinese and unusually long Bonjour-name layout checks at 1120px and 960px widths, including effective ellipsis, card bounds, visible dropdown arrow and full-name tooltips.


## 0.3.2 address regression

Settings and scan-engine validation now share unicast-IP rules. Tests use the documentation-only address 198.51.100.6 for campus-like networks; no test contacts that address. Coverage includes saved settings reload, direct endpoint discovery, rejection of public range candidates, invalid/broadcast/multicast endpoints, and a canceled status request reaching the HTTP transport. The browser harness exercises the real settings API with a mocked connection-check response, then verifies persistence/reload alongside all existing rename/history/layout scenarios. Set M175_TEST_BROWSER to an installed Chromium-family executable when the Playwright-managed browser is unavailable; it runs headlessly in an isolated profile.

Both architectures build and pass ad-hoc signature verification. Native launch on Apple Silicon preserves existing history. Real-printer checks in this environment were refused at the requested scan port; no physical scan was started. Intel execution and successful scanning on the new network remain unverified.


## 0.3.3 sidebar display

The browser harness checks wrapped English/Chinese names, complete IPv4/IPv6 text, a separate Connected label, visible dropdown arrow, and footer/navigation bounds at 1120/960 px widths and 665 px height. Very long names have a bounded scroll area. No personal scanned images are used.


## 0.3.5 Files operations

Run the Go race suite and `node tests/features-ui.cjs` as above. New UI cases use temporary documents and mocked copy/Trash responses, while Go tests verify file selection, mutation guards and unchanged contents after a simulated folder move. The browser suite includes filtered navigation, keyboard/toolbar/context rename, Command-C and native Copy routing, text-editing exclusions, Delete/Backspace confirmation, default cancellation, focus trapping, failure reporting, neighboring previews, empty results and reload.

Run the native integration test on macOS to exercise actual NSPasteboard file URLs and NSFileManager Trash:

```sh
mkdir -p build
xcrun clang -mmacosx-version-min=12.0 -fno-objc-arc -framework Cocoa native/FileActionsTest.m native/FileActions.m -o build/file-actions-test
./build/file-actions-test
```

It uses an independently named clipboard and its own temporary scan folder. After verifying the returned Trash item's contents, it removes only that disposable test item. It does not read or overwrite the user's clipboard or scan history. Manually verify Command-C/paste in Finder and your target app, restore a disposable scan using Trash, and repeat on Intel hardware. Existing real scans must not be used for destructive tests.
