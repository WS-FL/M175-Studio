# M175 Studio 0.3.3 beta

Fix truncated device details in the lower-left sidebar. The connection status and IP address now occupy separate lines, so the complete IPv4 address remains visible even at the minimum window width. IPv6 addresses wrap as needed. Normal device names wrap instead of using an ellipsis; unusually long advertised names can scroll within the name area while the status, address and dropdown arrow stay visible.

Preserves the existing sidebar width, colors and scan workspace. Retains the 0.3.2 campus/office address fix. Scan protocol and settings are unchanged.

Validation: simulated scan regressions, browser layout checks at normal/minimum widths and minimum height, long English/Chinese names, full IPv4 and wrapped IPv6 addresses, existing rename/history tests, universal Apple Silicon/Intel build, and ad-hoc signature verification. No physical scan is required or started for this display fix. Intel native runtime remains unverified. Not Apple-notarized. This test build has not been published as a GitHub Release.
