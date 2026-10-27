# M175 Studio 0.3.6 — Multiple file selection

Files now supports selecting and operating on multiple scans while keeping the existing layout and single-document preview.

- Command-click toggles individual documents; Shift-click selects a continuous range. Command-Shift-click adds a range. Shift-Up/Down extends or shrinks the selection.
- The selected count and Copy/Trash buttons reflect the complete selection. Right-clicking a selected row keeps the group; right-clicking another row selects that document.
- Command-C, Edit > Copy, the toolbar and the context menu copy the whole group in one clipboard operation. Each PDF scan contributes its main PDF; JPEG-only scans contribute their original pages.
- Delete/Backspace and Trash show one confirmation listing the count and names. Cancel remains the default. Each confirmed scan folder moves to macOS Trash with its PDF, original images and record.
- If part of a batch cannot be moved, successful moves are reported and failed documents stay selected. Retrying only targets those remaining documents.
- Search removes hidden items from the selection. Rename remains a single-document operation.

Validation covers mixed-format batch copy, deduplication, whole-selection validation before changes, request guards, partial Trash failure/retry, Command/Shift combinations, polling, context menus, filtered selections, confirmation/cancellation and minimum-window layout. Existing single-file, rename, discovery and mock scan regression tests are retained.

Universal macOS build for Apple Silicon and Intel; macOS 12 or later. The app is ad-hoc signed and not Apple-notarized. Intel execution and paste behavior in individual third-party applications still need user testing. The scanning core is unchanged; no new physical-printer scan was performed for this update.
