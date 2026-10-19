# M175 Studio 0.3.0 beta

Test build; no GitHub Release has been published for this version.

## Changes

- Automatically discover local network printers/scanners at launch and with Refresh / Auto Discover. Bonjour browses scanner, uscan, uscans, ipp, ipps, http and pdl-datastream services. A paced fallback checks only port 8289 within the adjacent /24 of up to two active physical interfaces, respecting narrower subnet masks. Each pass has a 45-second deadline and at most eight simultaneous probes. Refresh has a 10-second cooldown; there is no background polling of the LAN.
- Confirm scanner support using a read-only GetScannerElements SOAP request. An open port or unrelated printer announcement is not treated as scanner compatibility. Unconfirmed devices are shown as scanning not supported.
- Select supported scanners from the existing lower-left device area. Remember the selected endpoint and prefer it on the next successful discovery. Manual IP/port entry remains in Settings > Advanced / Manual Connection. Check Connection uses the selected endpoint outside Settings; Settings can save and check a manual endpoint.
- Rename the current document directly in the preview title: click to edit, Enter to save, Escape to cancel, or click elsewhere to save. Includes a subtle hover affordance, keyboard access, and Chinese IME handling.
- Store the display title in existing metadata and rename the main PDF to the same safe title. Scan directory IDs and original page names remain stable to preserve history and references. Unsafe characters become spaces, blank titles become Untitled scan, and duplicate titles receive a numeric suffix. Titles are limited to 60 Unicode characters. Existing PDFs are never overwritten.
- Preserve the compact 0.2.2 layout, existing scanner core and flatbed 300 dpi request path.

## Installation

Quit the previous copy. Unzip the package and drag the complete M175 Studio.app into Applications. Replace the previous app, then launch it. Your existing settings and scan folders remain in place. Allow local network access if macOS prompts. If discovery cannot find a device, open Diagnostics for permission/network guidance or enter its local IP in Settings.

The ZIP contains a universal native host and separate Apple Silicon / Intel engines. Requires macOS 12 or later. The app is ad-hoc signed; it is not Developer ID signed or Apple-notarized.

## Verification and limits

Automated Go tests cover existing simulated flatbed/ADF scan behavior, discovery with M175-compatible SOAP, no devices, multiple devices, a reachable port with incompatible SOAP, unsupported-device rejection, saved selection/reconnect choice, safe rename, English/Chinese names, illegal characters, blank names, collisions, case changes, preserved JPEG bytes, main-PDF metadata and legacy history loading. Race-enabled tests passed.

A Chromium UI harness checks Enter/blur save, Escape cancel, Chinese titles, IME Enter, polling during editing, library updates, reload, searching/no-device/multiple/unsupported device states, selection and small-window layout. UI discovery responses in that harness are fixtures; actual HTTP protocol probes are tested separately in Go.

Apple Silicon native AppKit/WKWebView startup and the device menu were checked on a Mac. Existing scan history loaded successfully. The native smoke test used an isolated test profile with discovery disabled and did not perform a physical scan.

Physical M175nw scanning, Bonjour on a real printer, local-network permission grant/denial, and Intel Mac execution still require user hardware testing. Earlier flatbed color 300 dpi validation belongs to the existing scan core; this version has not been physically rescanned. ADF, other resolutions and other printer models remain unverified. This continues the published English rebuild lineage; it is not an exact reconstruction of the earlier Chinese 0.2.2 source.
