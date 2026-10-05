# Source layout

- `native/Studio.m`: AppKit window, WKWebView, menus, native bridge and engine lifetime.
- `native/Info.plist`: app metadata and local-network permission explanation.
- `native/MacABI.h`: minimal declarations for historical cross-builds; normal Mac builds use installed Apple headers.
- `engine/core.go`: SOAP/DIME scanning, context cancellation and PDF generation.
- `engine/server.go`: loopback UI service, settings, scan jobs, history and access checks.
- `engine/ui/`: English HTML, CSS and JavaScript interface.
- `engine/*_test.go`: protocol, persistence and conversion tests.
- `tests/ui_test.py`: historical Linux browser/mock-printer component harness, adapted to English text; not a native WKWebView test.
- `build-macos.sh`: build using the real Apple SDK and ad-hoc codesign.
- `sign_macho.py`: inserts binary UUIDs needed by the historical Go build.
- `build_stubs.py` and `strip_object_platform.py`: historical cross-build helpers, not needed for the native Mac recipe.

The English revision was derived from the recovered 0.2.0 source, translated and updated to retain the compact scan header from 0.2.2. The original 0.2.1/0.2.2 source patches were not recovered. Building with installed Apple headers avoids the old minimal NSObject cross-build layout issue. Native/MacABI.h now includes the missing object-header field for consistency, but the native build does not use that fallback.

Normal app users do not need build tools. See BUILD.md for reproducible source build instructions and TESTING.md for the actual verification scope.
