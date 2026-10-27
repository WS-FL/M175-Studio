# M175 Studio

An independent Mac scanning app for the **HP LaserJet 100 colorMFP M175nw**. It connects directly to the printer's legacy scan interface over your local network, without VueScan or legacy HP Mac drivers. Scan, Files, Settings and Diagnostics are available in one window. Documents stay on your Mac.

M175 Studio is not affiliated with HP or Apple and does not claim official support.

## Install

1. For this 0.3.6 test build, use the supplied universal Mac ZIP. Published versions are available under [Releases](https://github.com/WS-FL/M175-Studio/releases).
2. Quit any running copy, unzip the download and move the entire **M175 Studio.app** to **Applications**. Replace the old app if prompted.
3. Open the app and allow automatic discovery to finish. Click the lower-left device area to choose a supported scanner or refresh discovery. If necessary, use **Settings > Advanced / Manual Connection** to enter a local IP and port (default `8289`).
4. Your Mac must be able to reach the printer on your network. Allow local network access when macOS asks, then click **Check Connection**.
5. Place a document face down on the glass and click **Start Scan**.

Requires macOS 12 or later. The app includes Apple Silicon and Intel executables. Normal use does not require Go, Python, Xcode, Homebrew or Terminal. Existing settings and scanned documents are preserved. A filename prefix previously saved by the user is preserved; new installations default to `Scan`.

## Features and limitations

- Flatbed and single-sided ADF controls; Letter and A4; 150, 200, 300 and 600 dpi options.
- Color JPEG originals, combined PDFs, and optional local grayscale PDF conversion.
- Scan history, page previews, persistent settings and diagnostic logs. In Files, Command-click toggles documents and Shift-click selects a range. Copy and Trash operate on the selected group, with a confirmation listing names before Trash. Use Up/Down to preview, Return/F2 to rename, Command-C to copy the PDF (or JPEG originals), and Delete to move a scan to Trash after confirmation. Toolbar buttons and a right-click menu provide the same actions.
- Bounded Bonjour / local subnet discovery with read-only legacy SOAP confirmation and remembered scanner selection. Unconfirmed devices are labeled as unsupported.
- Click a preview title to rename it inline. Enter or clicking elsewhere saves; Escape cancels. The main PDF reflects the name in Finder while original page names and scan folder IDs stay stable.

Manual IP entry and individually advertised Bonjour devices support campus/office networks using non-private unicast addresses. Automatic range probing remains limited to private local subnets; a saved manual endpoint is retried on launch.

See [0.3.6 multiple selection](RELEASE_NOTES_0.3.6.md), [0.3.5 Files shortcuts](RELEASE_NOTES_0.3.5.md), [0.3.4 navy theme](RELEASE_NOTES_0.3.4.md), [0.3.3 device display fix](RELEASE_NOTES_0.3.3.md), [0.3.2 network address fix](RELEASE_NOTES_0.3.2.md), [0.3.1 display fix](RELEASE_NOTES_0.3.1.md) and [0.3.0 test build notes](RELEASE_NOTES_0.3.0.md) for discovery limits and the validation scope.

- Flatbed color scanning at 300 dpi was previously verified on a real M175nw. ADF and other combinations still need hardware testing. Simulated printer tests are not evidence of ADF hardware compatibility.
- The English revision rebuilds the available source using the real macOS SDK. Its scanner logic derives from the available 0.2.0 source snapshot. It preserves the compact scan header introduced in 0.2.2. It is not a byte-for-byte reconstruction of the earlier Chinese 0.2.2 package.
- The app is ad-hoc signed, without Apple Developer ID signing or notarization. Review any macOS security message carefully; do not disable system security protections.
- No OCR, automatic cropping or hardware duplex scanning. Other printer models are unverified.

All project documentation, app interface text and code comments are in English. User-authored filenames, document titles, saved preferences and operating-system messages can retain their original language.

## Build and verification

The English source used to build the English app is included: `native/` contains the AppKit/WKWebView host, `engine/` the Go scan engine and embedded interface, and `tests/` the browser test harness. See [BUILD.md](BUILD.md) and [TESTING.md](TESTING.md).

## License and third-party notices

No license has been specified for the project itself. Public source availability does not grant an MIT, Apache or other open-source license. Keep [NOTICE.txt](NOTICE.txt) and [GO-LICENSE.txt](GO-LICENSE.txt) with distributed builds. Go runtime and standard library license terms remain unchanged.
