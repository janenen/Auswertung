package main

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"os"
	"testing"
)

// resources walks a resource directory tree and returns the data of every
// leaf by type and ID. base is the section's address, which the data
// entries' addresses include.
func resources(t *testing.T, sec []byte, base uint32) map[[2]uint32][]byte {
	t.Helper()
	out := map[[2]uint32][]byte{}
	u32 := func(off uint32) uint32 { return binary.LittleEndian.Uint32(sec[off:]) }
	entries := func(off uint32) [][2]uint32 {
		n := uint32(binary.LittleEndian.Uint16(sec[off+12:])) + uint32(binary.LittleEndian.Uint16(sec[off+14:]))
		var es [][2]uint32
		for i := range n {
			e := off + 16 + 8*i
			es = append(es, [2]uint32{u32(e), u32(e + 4)})
		}
		return es
	}
	for _, typ := range entries(0) {
		for _, id := range entries(typ[1] &^ 0x80000000) {
			for _, lang := range entries(id[1] &^ 0x80000000) {
				if lang[1]&0x80000000 != 0 {
					t.Fatal("language entry points to a directory")
				}
				addr, size := u32(lang[1]), u32(lang[1]+4)
				out[[2]uint32{typ[0], id[0]}] = sec[addr-base : addr-base+size]
			}
		}
	}
	return out
}

func TestObject(t *testing.T) {
	data, err := os.ReadFile("../../web/public/favicon.ico")
	if err != nil {
		t.Fatal(err)
	}
	ico, err := parseICO(data)
	if err != nil {
		t.Fatal(err)
	}
	for arch, m := range machines {
		obj, err := object(arch, ico)
		if err != nil {
			t.Fatal(err)
		}
		f, err := pe.NewFile(bytes.NewReader(obj))
		if err != nil {
			t.Fatalf("%s: %v", arch, err)
		}
		if f.Machine != m.machine || len(f.Sections) != 1 || f.Sections[0].Name != ".rsrc" || len(f.Symbols) != 1 {
			t.Fatalf("%s: machine %x, sections %v, symbols %v", arch, f.Machine, f.Sections, f.Symbols)
		}
		s := f.Sections[0]
		if len(s.Relocs) != len(ico)+1 {
			t.Errorf("%s: %d relocations", arch, len(s.Relocs))
		}
		for _, r := range s.Relocs {
			if r.Type != m.reloc || r.SymbolTableIndex != 0 {
				t.Errorf("%s: relocation %+v", arch, r)
			}
		}
		sec, _ := s.Data()
		res := resources(t, sec, 0)
		if len(res) != len(ico)+1 {
			t.Fatalf("%s: %d resources", arch, len(res))
		}
		for i, img := range ico {
			if !bytes.Equal(res[[2]uint32{rtIcon, uint32(i + 1)}], img.data) {
				t.Errorf("%s: icon %d differs", arch, i+1)
			}
		}
		if !bytes.Equal(res[[2]uint32{rtGroupIcon, 1}], groupIcon(ico)) {
			t.Errorf("%s: group icon differs", arch)
		}
	}
}

// TestCommitted checks that the committed .syso files are up to date.
func TestCommitted(t *testing.T) {
	data, err := os.ReadFile("../../web/public/favicon.ico")
	if err != nil {
		t.Fatal(err)
	}
	ico, _ := parseICO(data)
	for arch := range machines {
		want, _ := object(arch, ico)
		got, err := os.ReadFile("../../cmd/auswertung/rsrc_windows_" + arch + ".syso")
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("cmd/auswertung/rsrc_windows_%s.syso is missing or outdated (%v): run go generate ./cmd/auswertung", arch, err)
		}
	}
}

func TestParseICOErrors(t *testing.T) {
	for _, data := range [][]byte{nil, []byte("PNG..."), {0, 0, 1, 0, 1, 0}, {0, 0, 1, 0, 0, 0}} {
		if _, err := parseICO(data); err == nil {
			t.Errorf("parseICO(%q) succeeded", data)
		}
	}
}
