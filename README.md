# M175 Studio

An independent Mac scanning app for the **HP LaserJet 100 colorMFP M175nw**. It connects directly to the printer's legacy scan interface over your local network, without VueScan or legacy HP Mac drivers. Scan, Files, Settings and Diagnostics are available in one window. Documents stay on your Mac.

M175 Studio is not affiliated with HP or Apple and does not claim official support.

## Install

1. Download the English Mac ZIP from [Releases](https://github.com/WS-FL/M175-Studio/releases).
2. Quit any running copy, unzip the download and move the entire **M175 Studio.app** to **Applications**. Replace the old app if prompted.
3. Open the app. In **Settings**, enter your printer's local IP address. The example default is `192.168.5.44`; change it to your device's address. The default scan port is `8289`.
4. Keep your Mac and printer on the same local network. Allow local network access when macOS asks, then click **Check Connection**.
5. Place a document face down on the glass and click **Start Scan**.

Requires macOS 12 or later. The app includes Apple Silicon and Intel executables. Normal use does not require Go, Python, Xcode, Homebrew or Terminal. Existing settings and scanned documents are preserved. A filename prefix previously saved by the user is preserved; new installations default to `Scan`.

## Features and limitations

- Flatbed and single-sided ADF controls; Letter and A4; 150, 200, 300 and 600 dpi options.
- Color JPEG originals, combined PDFs, and optional local grayscale PDF conversion.
- Scan history, page previews, persistent settings and diagnostic logs.
- Flatbed color scanning at 300 dpi was previously verified on a real M175nw. ADF and other combinations still need hardware testing. Simulated printer tests are not evidence of ADF hardware compatibility.
- The English revision rebuilds the available source using the real macOS SDK. Its scanner logic derives from the available 0.2.0 source snapshot. It preserves the compact scan header introduced in 0.2.2. It is not a byte-for-byte reconstruction of the earlier Chinese 0.2.2 package.
- The app is ad-hoc signed, without Apple Developer ID signing or notarization. Review any macOS security message carefully; do not disable system security protections.
- No OCR, automatic cropping or hardware duplex scanning. Other printer models are unverified.

All project documentation, app interface text and code comments are in English. User-authored filenames, document titles, saved preferences and operating-system messages can retain their original language.

## Build and verification

The English source used to build the English app is included: `native/` contains the AppKit/WKWebView host, `engine/` the Go scan engine and embedded interface, and `tests/` the browser test harness. See [BUILD.md](BUILD.md) and [TESTING.md](TESTING.md).

## License and third-party notices

No license has been specified for the project itself. Public source availability does not grant an MIT, Apache or other open-source license. Keep [NOTICE.txt](NOTICE.txt) and [GO-LICENSE.txt](GO-LICENSE.txt) with distributed builds. Go runtime and standard library license terms remain unchanged.
