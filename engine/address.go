package main

import "net"

// Private address space is not a test of LAN reachability: campus networks
// can assign globally routable addresses to printers. Explicit endpoints may
// use any unicast IP literal. Do not resolve arbitrary hostnames or URLs here.
func validPrinterIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if v4 := ip.To4(); v4 != nil && (v4[0] == 0 || v4[0] >= 224) {
		return false
	}
	return ip.IsGlobalUnicast() || ip.IsLoopback() || ip.IsLinkLocalUnicast()
}

// Range discovery remains restricted. Only a saved/manual endpoint or an
// individual Bonjour advertisement may opt in to a non-private address.
func discoveryAddressAllowed(d Device) bool {
	ip := net.ParseIP(d.IP)
	return validPrinterIP(ip) && d.Port > 0 && d.Port <= 65535 &&
		(d.direct || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast())
}
