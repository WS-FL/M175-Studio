# M175 Studio 0.3.2 beta

Fix manual printer connections on networks that assign non-private IP addresses. Previous versions incorrectly treated private address ranges as the only valid LAN endpoints, rejecting valid campus/office printer addresses before attempting a connection.

- Use the same unicast IP validation for settings and the scan engine. Save and Check Connection, scanning, and settings reload now accept these endpoints.
- Retry the saved endpoint at launch, including non-private addresses. Individually advertised Bonjour IPv4 endpoints can also be confirmed using read-only legacy SOAP requests.
- Keep subnet range probing private-only, bounded and paced. No public-address range sweeps are added. Multicast, broadcast, unspecified addresses, URLs and hostnames remain invalid.
- Preserve the existing interface, documents, rename behavior and flatbed 300 dpi scan path.

Universal Apple Silicon / Intel app for macOS 12 or later. Ad-hoc signed, not Apple-notarized. Automated tests cover address validation, persistence, explicit discovery versus range restrictions, status transport cancellation, existing simulated scan/rename flows, and browser Save and Check Connection. Real scanning and Intel runtime still require hardware testing. This test build has not been published as a GitHub Release.
