# M175 Studio 0.3.7 — File clipboard fix

Fixes Copy reporting success even though Finder and other apps receive no files. The failure was reproduced through the running 0.3.6 app: its copy endpoint returned success while an independent clipboard reader found no file URLs.

- Copy now writes file URLs from the native GUI process. The backend validates the selected history entries and returns their file paths; it no longer launches a clipboard helper.
- The native app reads back the clipboard contents before acknowledging success. The interface waits for this acknowledgment and reports errors or a timeout instead of displaying a false success message.
- Single and multiple selection, Command-C, Edit > Copy, toolbar and context-menu copying use the same path. PDF scans copy the main PDF; JPEG-only scans copy the original pages.
- Adds a repeatable macOS regression test covering actual GUI copy, an independent clipboard reader, and Finder paste with filename and byte-for-byte checks.

Validation: Go tests and race tests; browser UI regression for copy acknowledgment, failure, timeout/late acknowledgment, keyboard handling, rename, multi-selection and deletion; native file validation; actual single/batch Finder paste of Chinese/space filenames on Apple Silicon. WeChat accepted both copied files as attachment cards in File Transfer; the unsent test draft was cleared. Existing settings and scan history are preserved. The scan core is unchanged.

Universal Apple Silicon/Intel build, macOS 12 or later. Intel execution still needs verification. No new physical-printer scan was performed. Ad-hoc signed; not Apple-notarized.
