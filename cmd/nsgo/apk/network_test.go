package apk

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"io"
	"slices"
	"strings"
	"testing"

	androidruntime "github.com/nsgo-dev/nsgo/internal/platform/android/runtime"
)

func TestLoopbackCleartext(t *testing.T) {
	key, err := NewKey()
	if err != nil {
		t.Fatal(err)
	}
	const policyPath = "res/xml/network_security_config.xml"
	const attrName = "networkSecurityConfig"
	const frameworkID = 0x01010527
	hosts := []string{"localhost", "127.0.0.1", "::1", "[::1]"}
	for _, format := range []struct {
		name, module, manifest, table string
		write                         func(io.Writer, []byte, Manifest, []File, *Key) error
	}{
		{"apk", "", "AndroidManifest.xml", "resources.arsc", Write},
		{"bundle", "base/", "base/manifest/AndroidManifest.xml", "base/resources.pb", WriteBundle},
	} {
		t.Run(format.name, func(t *testing.T) {
			for _, tc := range []struct {
				name                string
				enable, icon, label bool
				id                  uint32
			}{
				{name: "default"},
				{name: "no other resources", enable: true, id: 0x7f010000},
				{name: "icon only", enable: true, icon: true, id: 0x7f020000},
				{name: "localized label only", enable: true, label: true, id: 0x7f020000},
				{name: "icon and localized label", enable: true, icon: true, label: true, id: 0x7f030000},
			} {
				t.Run(tc.name, func(t *testing.T) {
					m := Manifest{Package: "dev.nsgo.test.loopback", Label: "Test", VersionCode: 1, LoopbackCleartext: tc.enable}
					if tc.icon {
						m.Icon = []byte("PNG")
					}
					if tc.label {
						m.Names = map[string]string{"": "Test"}
					}
					var buf bytes.Buffer
					if err := format.write(&buf, androidruntime.Manifest, m, nil, key); err != nil {
						t.Fatal(err)
					}
					archive, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
					if err != nil {
						t.Fatal(err)
					}
					read := func(name string) []byte {
						t.Helper()
						file, err := archive.Open(name)
						if err != nil {
							t.Fatal(err)
						}
						defer file.Close()
						data, err := io.ReadAll(file)
						if err != nil {
							t.Fatal(err)
						}
						return data
					}
					manifest := read(format.manifest)
					if !tc.enable {
						for _, file := range archive.File {
							if file.Name == format.module+policyPath {
								t.Fatal("default package contains a network policy")
							}
						}
						if format.name == "bundle" {
							if bytes.Contains(manifest, []byte(attrName)) {
								t.Fatal("default bundle manifest declares a network policy")
							}
						} else {
							if slices.Contains(decodeXML(t, manifest).ids, uint32(frameworkID)) {
								t.Fatal("default APK manifest declares a network policy")
							}
						}
						return
					}
					config, table := read(format.module+policyPath), read(format.table)
					if format.name == "bundle" {
						ref := pbBytes(nil, 1, pbVarint(nil, 2, uint64(tc.id)))
						compiled := pbBytes(pbVarint(nil, 5, frameworkID), 6, ref)
						if !bytes.Contains(manifest, compiled) {
							t.Fatalf("bundle manifest lacks reference to %#x", tc.id)
						}
						typePrefix := pbBytes(nil, 1, pbVarint(nil, 1, uint64(tc.id>>16&0xff)))
						typePrefix = pbString(typePrefix, 2, "xml")
						fileItem := pbVarint(pbString(nil, 1, policyPath), 2, 3)
						if !bytes.Contains(table, typePrefix) || !bytes.Contains(table, pbBytes(nil, 5, fileItem)) {
							t.Fatal("bundle table lacks the policy XML resource")
						}
						for _, host := range hosts {
							if !bytes.Contains(config, pbString(nil, 2, host)) {
								t.Errorf("bundle XML lacks the text node for %s", host)
							}
						}
						for _, allowed := range []uint64{0, 1} {
							compiled := pbBytes(nil, 6, pbBytes(nil, 7, pbVarint(nil, 8, allowed)))
							if !bytes.Contains(config, compiled) {
								t.Errorf("bundle XML lacks the typed boolean %d", allowed)
							}
						}
						return
					}
					app := decodeXML(t, manifest).root.child(t, "application")
					if attr := app.attr(t, attrName); attr.kind != typeReference || attr.id != frameworkID || attr.data != tc.id {
						t.Errorf("manifest policy attribute = %+v, want reference to %#x", attr, tc.id)
					}
					if !strings.Contains(string(table), policyPath) {
						t.Fatal("APK table lacks the policy XML file")
					}
					root := decodeXML(t, config).root
					base, loopback := root.child(t, "base-config"), root.child(t, "domain-config")
					for _, spec := range []struct {
						node *xmlNode
						want uint32
					}{{base, 0}, {loopback, ^uint32(0)}} {
						attrs := spec.node.attrs
						if len(attrs) != 1 || attrs[0].kind != typeBool ||
							stringAt(t, spec.node, attrs[0].name) != "cleartextTrafficPermitted" || attrs[0].data != spec.want {
							t.Errorf("%s attrs = %+v, want cleartextTrafficPermitted = %#x", spec.node.name, attrs, spec.want)
						}
					}
					var domains []string
					for _, node := range loopback.kids {
						if node.name != "domain" || len(node.attrs) != 0 {
							t.Fatalf("invalid domain node: %+v", node)
						}
						domains = append(domains, node.text)
					}
					if !slices.Equal(domains, hosts) {
						t.Errorf("domains = %v, want %v", domains, hosts)
					}
					// Find the APK type spec for the manifest's reference.
					id := byte(tc.id >> 16)
					spec := binary.LittleEndian.AppendUint16(nil, tableTypeSpec)
					spec = append(spec, 16, 0, 20, 0, 0, 0, id, 0, 0, 0, 1, 0, 0, 0)
					if !bytes.Contains(table, spec) {
						t.Errorf("APK table lacks the policy type spec %#x", id)
					}
				})
			}
		})
	}
}
