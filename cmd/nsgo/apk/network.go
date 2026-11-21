package apk

import (
	"encoding/xml"
	"strconv"
	"strings"
)

const (
	networkSecurityName = "network_security_config"
	networkSecurityFile = "res/xml/" + networkSecurityName + ".xml"
	cleartextAttribute  = "cleartextTrafficPermitted"
)

// networkSecurityID follows the optional launcher icon and app names in the resource table.
func networkSecurityID(m Manifest) uint32 {
	id := nameResourceID(m.Icon != nil)
	if len(m.Names) > 0 {
		id += 1 << 16
	}
	return id
}

func parseResourceID(value string) (uint32, error) {
	id, err := strconv.ParseUint(strings.TrimPrefix(value, "@"), 0, 32)
	return uint32(id), err
}

// networkSecurityConfig allows HTTP for the exact loopback host names that
// Android clients pass to the policy, including both forms of the IPv6 literal.
func networkSecurityConfig() *node {
	base := &node{name: xml.Name{Local: "base-config"}}
	setAttr(base, "", cleartextAttribute, "false")
	loopback := &node{name: xml.Name{Local: "domain-config"}}
	setAttr(loopback, "", cleartextAttribute, "true")
	for _, host := range []string{"localhost", "127.0.0.1", "::1", "[::1]"} {
		loopback.kids = append(loopback.kids, &node{name: xml.Name{Local: "domain"}, text: host})
	}
	return &node{
		name: xml.Name{Local: "network-security-config"},
		kids: []*node{base, loopback},
	}
}
