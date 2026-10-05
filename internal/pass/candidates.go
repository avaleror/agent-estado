package pass

import (
	"net"
	"net/netip"
)

type cand struct {
	Proto string `json:"proto"`
	IP    string `json:"ip"`
	Port  int    `json:"port"`
}

func allowAdvertise(ip netip.Addr) bool {
	if !ip.IsValid() || !ip.IsGlobalUnicast() {
		return false
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	return true
}

func allowDial(ip netip.Addr) bool {
	if !allowAdvertise(ip) {
		return false
	}
	if ip.IsPrivate() {
		return onLink(ip)
	}
	return true
}

func onLink(ip netip.Addr) bool {
	ifaces, err := net.Interfaces()
	if err != nil {
		return false
	}
	raw := ip.AsSlice()
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			n, ok := a.(*net.IPNet)
			if ok && n.Contains(raw) {
				return true
			}
		}
	}
	return false
}

func localCandidates(port int) []cand {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []cand
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			n, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip, ok := netip.AddrFromSlice(n.IP)
			if !ok {
				continue
			}
			ip = ip.Unmap()
			if !allowAdvertise(ip) {
				continue
			}
			out = append(out, cand{Proto: "tcp", IP: ip.String(), Port: port})
			if len(out) == 8 {
				return out
			}
		}
	}
	return out
}

func classOf(c cand) byte {
	if c.Proto == "udp" {
		return classPunch
	}
	ip, err := netip.ParseAddr(c.IP)
	if err == nil && ip.IsPrivate() {
		return classLAN
	}
	return classMap
}

func validCand(c cand) bool {
	if c.Proto != "tcp" && c.Proto != "udp" {
		return false
	}
	if c.Port < 1 || c.Port > 65535 {
		return false
	}
	ip, err := netip.ParseAddr(c.IP)
	if err != nil || ip.Zone() != "" {
		return false
	}
	return allowAdvertise(ip)
}

func dialable(c cand) bool {
	if c.Proto != "tcp" {
		return false
	}
	ip, err := netip.ParseAddr(c.IP)
	if err != nil {
		return false
	}
	return allowDial(ip)
}
