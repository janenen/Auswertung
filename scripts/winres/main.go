// Command winres writes Windows resource objects (.syso) with the program
// icon, which the Go linker embeds when building for Windows. Run it via
// go generate in cmd/auswertung:
//
//	go run ../../scripts/winres -icon ../../web/public/favicon.ico
//
// It writes rsrc_windows_<arch>.syso for every architecture; the files are
// committed, so plain go build needs no extra tools.
package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// COFF machine types and the relocation type for an image-relative
// address (ADDR32NB) on each architecture.
var machines = map[string]struct{ machine, reloc uint16 }{
	"amd64": {0x8664, 0x3},
	"arm64": {0xaa64, 0x2},
}

const (
	rtIcon      = 3
	rtGroupIcon = 14
	langEnUS    = 0x409
)

func main() {
	icon := flag.String("icon", "", "icon file (.ico)")
	out := flag.String("out", ".", "output directory")
	flag.Parse()
	if *icon == "" {
		log.Fatal("-icon missing")
	}
	data, err := os.ReadFile(*icon)
	if err != nil {
		log.Fatal(err)
	}
	ico, err := parseICO(data)
	if err != nil {
		log.Fatalf("%s: %v", *icon, err)
	}
	for _, arch := range []string{"amd64", "arm64"} {
		obj, err := object(arch, ico)
		if err != nil {
			log.Fatal(err)
		}
		name := filepath.Join(*out, "rsrc_windows_"+arch+".syso")
		if err := os.WriteFile(name, obj, 0o644); err != nil {
			log.Fatal(err)
		}
	}
}

// iconImage is one image of an .ico file.
type iconImage struct {
	width, height, colors, reserved uint8
	planes, bitCount                uint16
	data                            []byte
}

func parseICO(data []byte) ([]iconImage, error) {
	if len(data) < 6 || binary.LittleEndian.Uint16(data[0:]) != 0 || binary.LittleEndian.Uint16(data[2:]) != 1 {
		return nil, errors.New("not an icon file")
	}
	n := int(binary.LittleEndian.Uint16(data[4:]))
	if n == 0 || len(data) < 6+16*n {
		return nil, errors.New("truncated icon directory")
	}
	images := make([]iconImage, n)
	for i := range images {
		e := data[6+16*i:]
		size := binary.LittleEndian.Uint32(e[8:])
		offset := binary.LittleEndian.Uint32(e[12:])
		if uint64(offset)+uint64(size) > uint64(len(data)) {
			return nil, fmt.Errorf("image %d outside the file", i)
		}
		images[i] = iconImage{
			width: e[0], height: e[1], colors: e[2], reserved: e[3],
			planes:   binary.LittleEndian.Uint16(e[4:]),
			bitCount: binary.LittleEndian.Uint16(e[6:]),
			data:     data[offset : offset+size],
		}
	}
	return images, nil
}

// groupIcon is the RT_GROUP_ICON resource: the icon directory with
// resource IDs (1…n) instead of file offsets.
func groupIcon(images []iconImage) []byte {
	var b bytes.Buffer
	le := func(v any) { binary.Write(&b, binary.LittleEndian, v) }
	le([3]uint16{0, 1, uint16(len(images))})
	for i, img := range images {
		b.Write([]byte{img.width, img.height, img.colors, img.reserved})
		le([2]uint16{img.planes, img.bitCount})
		le(uint32(len(img.data)))
		le(uint16(i + 1))
	}
	return b.Bytes()
}

type resource struct {
	typ, id uint16
	data    []byte
}

// object returns a COFF object with a single .rsrc section holding the
// resource directory (type → ID → language → data entry) and the data.
// The data entries' addresses are relative to the section and relocated by
// the linker.
func object(arch string, images []iconImage) ([]byte, error) {
	m, ok := machines[arch]
	if !ok {
		return nil, fmt.Errorf("unknown architecture %q", arch)
	}

	// Resources sorted by type, then ID, as the directory requires.
	var res []resource
	for i, img := range images {
		res = append(res, resource{rtIcon, uint16(i + 1), img.data})
	}
	res = append(res, resource{rtGroupIcon, 1, groupIcon(images)})
	types := []uint16{rtIcon, rtGroupIcon}

	const dirSize, entrySize, dataEntrySize = 16, 8, 16
	byType := func(t uint16) []resource {
		var out []resource
		for _, r := range res {
			if r.typ == t {
				out = append(out, r)
			}
		}
		return out
	}

	// Layout: root dir, one ID dir per type, one language dir per
	// resource, the data entries, then the data (8-byte aligned).
	idDirs := dirSize + entrySize*len(types)
	langDirs := idDirs
	for _, t := range types {
		langDirs += dirSize + entrySize*len(byType(t))
	}
	dataEntries := langDirs + (dirSize+entrySize)*len(res)
	dataStart := dataEntries + dataEntrySize*len(res)

	var sec bytes.Buffer
	le := func(v any) { binary.Write(&sec, binary.LittleEndian, v) }
	writeDir := func(ids int) {
		le(uint32(0)) // characteristics
		le(uint32(0)) // time stamp
		le(uint16(0)) // major version
		le(uint16(0)) // minor version
		le(uint16(0)) // named entries
		le(uint16(ids))
	}
	const subdir = 0x80000000

	// Root: one entry per type.
	writeDir(len(types))
	off := idDirs
	for _, t := range types {
		le(uint32(t))
		le(uint32(subdir | off))
		off += dirSize + entrySize*len(byType(t))
	}
	// Per type: one entry per ID.
	langOff := langDirs
	for _, t := range types {
		rs := byType(t)
		writeDir(len(rs))
		for _, r := range rs {
			le(uint32(r.id))
			le(uint32(subdir | langOff))
			langOff += dirSize + entrySize
		}
	}
	// Per resource: one language entry pointing at its data entry. The
	// order matches res, which is sorted by type.
	for i := range res {
		writeDir(1)
		le(uint32(langEnUS))
		le(uint32(dataEntries + dataEntrySize*i))
	}
	// Data entries; OffsetToData is relocated.
	var relocs []uint32
	dataOff := dataStart
	for _, r := range res {
		relocs = append(relocs, uint32(sec.Len()))
		le(uint32(dataOff))
		le(uint32(len(r.data)))
		le(uint32(0)) // code page
		le(uint32(0)) // reserved
		dataOff = align8(dataOff + len(r.data))
	}
	for _, r := range res {
		sec.Write(r.data)
		sec.Write(make([]byte, align8(sec.Len())-sec.Len()))
	}

	// COFF file: header, section header, section data, relocations,
	// symbol table (the section symbol) and an empty string table.
	const fileHeader, sectionHeader, relocSize, symbolSize = 20, 40, 10, 18
	rawOff := fileHeader + sectionHeader
	relocOff := rawOff + sec.Len()
	symOff := relocOff + relocSize*len(relocs)

	var f bytes.Buffer
	w := func(v any) { binary.Write(&f, binary.LittleEndian, v) }
	w(m.machine)
	w(uint16(1)) // sections
	w(uint32(0)) // time stamp, for reproducible files
	w(uint32(symOff))
	w(uint32(1)) // symbols
	w(uint16(0)) // optional header size
	w(uint16(0)) // characteristics

	f.WriteString(".rsrc\x00\x00\x00")
	w(uint32(0)) // virtual size
	w(uint32(0)) // virtual address
	w(uint32(sec.Len()))
	w(uint32(rawOff))
	w(uint32(relocOff))
	w(uint32(0)) // line numbers
	w(uint16(len(relocs)))
	w(uint16(0))
	w(uint32(0x40000040)) // initialized data, readable

	f.Write(sec.Bytes())
	for _, r := range relocs {
		w(r)
		w(uint32(0)) // symbol index: the section symbol
		w(m.reloc)
	}

	f.WriteString(".rsrc\x00\x00\x00")
	w(uint32(0))   // value
	w(int16(1))    // section number
	w(uint16(0))   // type
	f.WriteByte(3) // storage class: static
	f.WriteByte(0) // aux symbols
	w(uint32(4))   // string table size (empty)
	return f.Bytes(), nil
}

func align8(n int) int { return (n + 7) &^ 7 }
