# M175 Studio 0.3.5 — Files shortcuts

This update adds everyday file operations to the Files page while preserving the navy and cool-gray interface and existing scanning workflow.

## File operations

- Select a scan and use Up/Down to preview neighboring documents in the current search results.
- Rename with the toolbar, right-click menu, Return/F2 on a selected row, or the preview title. Enter or clicking elsewhere saves; Escape cancels. The PDF filename updates while original JPEG filenames and internal folder IDs stay stable.
- Copy with Command-C, Edit > Copy, the toolbar or right-click menu. PDF scans copy the main PDF as a real Mac file; JPEG-only scans copy all original pages. Paste into Finder or applications that accept files. Normal text copying remains available while editing or selecting text.
- Delete/Backspace or Move to Trash opens an in-window confirmation with Cancel focused by default. Confirming moves the complete scan folder (PDF, originals and scan record) to macOS Trash. The next document is selected after removal. No permanent deletion is performed by the app.
- File operations are blocked while a scan or connection job is running. Failures are shown inside the window, and unsuccessful deletions retain the history entry.

## Validation

- Go unit/integration tests and race checks cover PDF/JPEG selection, renamed files, confirmation requirements, invalid/stale targets, symlinks, job-busy protection, failure preservation and HTTP request guards, alongside existing discovery, rename and mock-scanner regressions.
- Browser tests use disposable documents and cover keyboard navigation, filtered results, inline editing, Command-C routing, right-click actions, confirmation/cancel, failure states, next-preview selection, history reload and compact-window layout.
- A native macOS test uses a separate named clipboard and disposable files to verify real file URLs, multi-JPEG copying and a recoverable Trash move. It does not access the user's clipboard or scan history.
- Universal macOS build includes Apple Silicon and Intel executables and passes ad-hoc signature checks. Intel execution and paste behavior in individual third-party apps still require user testing.

The scan core and previously verified flatbed color 300 dpi path are unchanged. No new physical-printer scan was performed for this update. ADF and other resolutions remain unverified on hardware. macOS 12 or later is required; the app is ad-hoc signed and is not Apple-notarized.
