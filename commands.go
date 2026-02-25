package main

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// CmdCat prints a hex dump of the page.
func CmdCat(p *Page) {
	for i := 0; i < PageSize; i += 16 {
		fmt.Printf("%08x: ", i)
		for j := 0; j < 16; j++ {
			if j == 8 {
				fmt.Print(" ")
			}
			fmt.Printf("%02x", p.Data[i+j])
			if j < 15 {
				fmt.Print(" ")
			}
		}
		fmt.Print("  |")
		for j := 0; j < 16; j++ {
			b := p.Data[i+j]
			if b >= 0x20 && b <= 0x7e {
				fmt.Printf("%c", b)
			} else {
				fmt.Print(".")
			}
		}
		fmt.Println("|")
	}
}

// CmdFormat prints an ASCII art visualization of the page layout.
func CmdFormat(p *Page) {
	h := &p.Header
	pageSize := int(h.PageSz())
	if pageSize == 0 {
		pageSize = PageSize
	}

	width := 64
	bar := "+" + strings.Repeat("-", width-2) + "+"

	headerEnd := PageHeaderSize
	linpEnd := int(h.Lower)
	freeStart := linpEnd
	freeEnd := int(h.Upper)
	tupleEnd := int(h.Special)
	specialEnd := pageSize

	region := func(label string, start, end int) {
		size := end - start
		if size <= 0 {
			return
		}
		content := fmt.Sprintf(" %-30s [%5d - %5d] %5d bytes ", label, start, end-1, size)
		pad := width - 2 - len(content)
		if pad < 0 {
			pad = 0
		}
		fmt.Println(bar)
		fmt.Printf("|%s%s|\n", content, strings.Repeat(" ", pad))
	}

	fmt.Printf("\n  Page Layout (page size: %d, type: %s)\n", pageSize, p.Detected)
	fmt.Printf("  Offset 0x%04x - 0x%04x\n\n", 0, pageSize-1)

	region("Page Header (PageHeaderData)", 0, headerEnd)
	if linpEnd > headerEnd {
		nItems := (linpEnd - headerEnd) / ItemIdSize
		label := fmt.Sprintf("Line Pointers (%d items)", nItems)
		region(label, headerEnd, linpEnd)
	}
	if freeEnd > freeStart {
		region("Free Space", freeStart, freeEnd)
	}
	if tupleEnd > freeEnd {
		tupleLabel := "Tuples"
		if p.Detected == PageTypeHeap {
			tupleLabel = "Heap Tuples"
		} else {
			tupleLabel = fmt.Sprintf("Index Tuples (%s)", p.Detected)
		}
		region(tupleLabel, freeEnd, tupleEnd)
	}
	if specialEnd > tupleEnd {
		region(fmt.Sprintf("Special Space (%s)", p.Detected), tupleEnd, specialEnd)
	}
	fmt.Println(bar)

	// Proportional view
	fmt.Println()
	fmt.Println("  Proportional view:")
	totalCols := 60

	type regionInfo struct {
		char byte
		size int
	}
	regions := []regionInfo{
		{'H', headerEnd},
		{'L', linpEnd - headerEnd},
		{'.', freeEnd - freeStart},
		{'T', tupleEnd - freeEnd},
		{'S', specialEnd - tupleEnd},
	}

	cols := make([]int, len(regions))
	remaining := totalCols
	totalSize := 0
	for i, r := range regions {
		if r.size > 0 {
			cols[i] = 1
			remaining--
			totalSize += r.size
		}
	}
	if totalSize > 0 && remaining > 0 {
		for i, r := range regions {
			if r.size > 0 {
				extra := r.size * remaining / totalSize
				cols[i] += extra
			}
		}
		used := 0
		for _, c := range cols {
			used += c
		}
		for i := range regions {
			if used >= totalCols {
				break
			}
			if regions[i].size > 0 {
				cols[i]++
				used++
			}
		}
	}

	fmt.Print("  [")
	for i, r := range regions {
		fmt.Print(strings.Repeat(string(r.char), cols[i]))
	}
	fmt.Println("]")
	fmt.Println("   H=Header  L=LinePointers  .=Free  T=Tuples  S=Special")
	fmt.Println()
}

// CmdInfo prints human-readable header and special region information.
func CmdInfo(p *Page) {
	h := &p.Header

	fmt.Println()
	fmt.Printf("=== Page Header (detected type: %s) ===\n", p.Detected)
	fmt.Printf("  pd_lsn             : %X/%08X\n", h.LSN>>32, h.LSN&0xFFFFFFFF)
	fmt.Printf("  pd_checksum        : 0x%04X (%d)\n", h.Checksum, h.Checksum)
	fmt.Printf("  pd_flags           : 0x%04X [%s]\n", h.Flags, FlagsString(h.Flags))
	fmt.Printf("  pd_lower           : %d (0x%04X)\n", h.Lower, h.Lower)
	fmt.Printf("  pd_upper           : %d (0x%04X)\n", h.Upper, h.Upper)
	fmt.Printf("  pd_special         : %d (0x%04X)\n", h.Special, h.Special)
	fmt.Printf("  pd_pagesize_version: 0x%04X (size: %d, version: %d)\n",
		h.PageSizeVer, h.PageSz(), h.LayoutVersion())
	fmt.Printf("  pd_prune_xid       : %d\n", h.PruneXID)

	numItems := 0
	if h.Lower > PageHeaderSize {
		numItems = int(h.Lower-PageHeaderSize) / ItemIdSize
	}
	freeSpace := 0
	if h.Upper > h.Lower {
		freeSpace = int(h.Upper - h.Lower)
	}

	fmt.Println()
	fmt.Println("=== Derived Info ===")
	fmt.Printf("  Line pointers      : %d\n", numItems)
	fmt.Printf("  Free space         : %d bytes\n", freeSpace)
	fmt.Printf("  Special space size : %d bytes\n", p.SpecialSize())

	// Decode special region based on detected type
	fmt.Println()
	fmt.Println("=== Special Region ===")

	special := p.SpecialData()
	if special == nil || p.SpecialSize() == 0 {
		fmt.Println("  (empty - heap/table page)")
	} else {
		fmt.Printf("  Size: %d bytes at offset %d\n", p.SpecialSize(), h.Special)
		fmt.Println()

		switch p.Detected {
		case PageTypeBTree:
			DecodeBTreeSpecial(special)
			// If meta page, also decode meta content
			btFlags := binary.LittleEndian.Uint16(special[12:14])
			if btFlags&BTPMeta != 0 {
				DecodeBTreeMeta(p)
			}
		case PageTypeHash:
			DecodeHashSpecial(special)
			hashFlag := binary.LittleEndian.Uint16(special[12:14])
			if hashFlag&LHMetaPage != 0 {
				DecodeHashMeta(p)
			}
		case PageTypeGiST:
			DecodeGiSTSpecial(special)
		case PageTypeGIN:
			DecodeGINSpecial(special)
			ginFlags := binary.LittleEndian.Uint16(special[6:8])
			if ginFlags&GINMeta != 0 {
				DecodeGINMeta(p)
			}
		case PageTypeSPGiST:
			DecodeSPGiSTSpecial(special)
		case PageTypeBRIN:
			DecodeBRINSpecial(special)
			brinType := binary.LittleEndian.Uint16(special[6:8])
			if brinType == BRINPageTypeMeta {
				DecodeBRINMeta(p)
			}
		default:
			fmt.Print("  Raw bytes: ")
			for i, b := range special {
				fmt.Printf("%02x ", b)
				if (i+1)%16 == 0 {
					fmt.Println()
					fmt.Print("             ")
				}
			}
			fmt.Println()
		}
	}
	fmt.Println()
}

// CmdData prints item pointers and tuple data with metadata.
func CmdData(p *Page) {
	h := &p.Header
	isIndex := p.Detected != PageTypeHeap && p.Detected != PageTypeUnknown

	fmt.Println()
	fmt.Printf("=== Line Pointers (Item IDs) [page type: %s] ===\n", p.Detected)
	fmt.Printf("  %-6s %-8s %-10s %-8s %-8s\n", "Index", "Status", "Offset", "Length", "Raw")
	fmt.Printf("  %-6s %-8s %-10s %-8s %-8s\n", "-----", "--------", "----------", "--------", "--------")

	for i, lp := range p.Items {
		fmt.Printf("  %-6d %-8s %-10d %-8d 0x%08X\n",
			i+1, lp.FlagsStr(), lp.Offset(), lp.Length(), lp.Raw)
	}

	if isIndex {
		printIndexTuples(p)
	} else {
		printHeapTuples(p)
	}

	// Summary
	fmt.Println()
	fmt.Println("=== Summary ===")
	normal, dead, unused, redirect := 0, 0, 0, 0
	for _, lp := range p.Items {
		switch lp.Flags() {
		case LPNormal:
			normal++
		case LPDead:
			dead++
		case LPUnused:
			unused++
		case LPRedirect:
			redirect++
		}
	}
	fmt.Printf("  Total line pointers: %d\n", len(p.Items))
	fmt.Printf("  NORMAL: %d, DEAD: %d, UNUSED: %d, REDIRECT: %d\n",
		normal, dead, unused, redirect)
	freeSpace := 0
	if h.Upper > h.Lower {
		freeSpace = int(h.Upper - h.Lower)
	}
	fmt.Printf("  Free space: %d bytes\n", freeSpace)
	fmt.Println()
}

func printHeapTuples(p *Page) {
	fmt.Println()
	fmt.Println("=== Heap Tuples ===")

	for i, lp := range p.Items {
		fmt.Printf("\n--- Tuple %d (offset %d, length %d) ---\n", i+1, lp.Offset(), lp.Length())

		if lp.Flags() == LPUnused {
			fmt.Println("  [UNUSED - no data]")
			continue
		}
		if lp.Flags() == LPRedirect {
			fmt.Printf("  [REDIRECT -> line pointer %d]\n", lp.Offset())
			continue
		}
		if lp.Flags() == LPDead {
			if lp.Length() == 0 {
				fmt.Println("  [DEAD - no storage]")
				continue
			}
			fmt.Println("  [DEAD - has storage]")
		}
		if lp.Length() == 0 || lp.Offset() == 0 {
			fmt.Println("  [no storage]")
			continue
		}
		if int(lp.Offset())+int(lp.Length()) > PageSize {
			fmt.Println("  [ERROR: tuple extends beyond page]")
			continue
		}

		t := p.ParseHeapTupleHeader(lp.Offset())

		fmt.Println("  Tuple Header (HeapTupleHeaderData):")
		fmt.Printf("    t_xmin       : %d\n", t.Xmin)
		fmt.Printf("    t_xmax       : %d", t.Xmax)
		if t.Xmax == InvalidXID {
			fmt.Print(" (INVALID)")
		}
		fmt.Println()
		fmt.Printf("    t_cid        : %d\n", t.Field3)
		fmt.Printf("    t_ctid       : (%d, %d)\n", t.CtidBlock, t.CtidOffset)
		fmt.Printf("    t_infomask2  : 0x%04X (natts: %d", t.Infomask2, t.NAttrs())
		if flags := t.Infomask2Flags(); len(flags) > 0 {
			fmt.Printf(", %s", strings.Join(flags, " | "))
		}
		fmt.Println(")")
		fmt.Printf("    t_infomask   : 0x%04X", t.Infomask)
		if flags := t.InfomaskFlags(); len(flags) > 0 {
			fmt.Printf(" [%s]", strings.Join(flags, " | "))
		}
		fmt.Println()
		fmt.Printf("    t_hoff       : %d\n", t.Hoff)

		// Null bitmap
		if t.Infomask&HeapHasNull != 0 {
			bitmapBytes := (t.NAttrs() + 7) / 8
			bitmapStart := int(lp.Offset()) + HeapTupleHdrSize
			fmt.Printf("    null bitmap  : ")
			for b := 0; b < bitmapBytes && bitmapStart+b < PageSize; b++ {
				fmt.Printf("%08b ", p.Data[bitmapStart+b])
			}
			fmt.Println()
		}

		// User data
		dataStart := int(lp.Offset()) + int(t.Hoff)
		dataEnd := int(lp.Offset()) + int(lp.Length())
		if dataEnd > PageSize {
			dataEnd = PageSize
		}
		dataLen := dataEnd - dataStart

		if dataLen > 0 {
			fmt.Printf("    User data (%d bytes at offset %d):\n", dataLen, dataStart)
			printHexBlock(p.Data[dataStart:dataEnd], dataStart, "      ")
			if strs := extractPrintable(p.Data[dataStart:dataEnd]); len(strs) > 0 {
				fmt.Println("    Printable strings:")
				for _, s := range strs {
					fmt.Printf("      \"%s\"\n", s)
				}
			}
		}
	}
}

func printIndexTuples(p *Page) {
	fmt.Println()
	fmt.Printf("=== Index Tuples (%s) ===\n", p.Detected)

	// Check if this is a meta page (btree/hash/gin/brin meta pages store
	// metadata in the content area, not standard tuples)
	if isMeta(p) {
		fmt.Println("  (meta page - content is metadata, not standard index tuples)")
		fmt.Println("  Use 'info' command to see decoded metadata.")
		return
	}

	for i, lp := range p.Items {
		fmt.Printf("\n--- Item %d (offset %d, length %d) ---\n", i+1, lp.Offset(), lp.Length())

		if lp.Flags() == LPUnused {
			fmt.Println("  [UNUSED]")
			continue
		}
		if lp.Flags() == LPRedirect {
			fmt.Printf("  [REDIRECT -> %d]\n", lp.Offset())
			continue
		}
		if lp.Flags() == LPDead {
			if lp.Length() == 0 {
				fmt.Println("  [DEAD - no storage]")
				continue
			}
			fmt.Println("  [DEAD - has storage]")
		}
		if lp.Length() == 0 || lp.Offset() == 0 {
			fmt.Println("  [no storage]")
			continue
		}
		if int(lp.Offset())+int(lp.Length()) > PageSize {
			fmt.Println("  [ERROR: extends beyond page]")
			continue
		}
		if lp.Length() < uint16(IndexTupleHdrSize) {
			fmt.Printf("  [too short for IndexTupleData: %d bytes]\n", lp.Length())
			// Show raw hex
			printHexBlock(p.Data[lp.Offset():int(lp.Offset())+int(lp.Length())], int(lp.Offset()), "    ")
			continue
		}

		it := p.ParseIndexTupleHeader(lp.Offset())

		fmt.Println("  Index Tuple Header (IndexTupleData):")
		fmt.Printf("    t_tid        : (%d, %d)  -> heap ctid\n", it.TidBlock, it.TidOffset)
		fmt.Printf("    t_info       : 0x%04X (size: %d", it.Info, it.Size())
		if flags := it.InfoFlags(); len(flags) > 0 {
			fmt.Printf(", %s", strings.Join(flags, " | "))
		}
		fmt.Println(")")

		// Key data follows the 8-byte header (possibly with null bitmap)
		keyStart := int(lp.Offset()) + IndexTupleHdrSize
		keyEnd := int(lp.Offset()) + int(lp.Length())
		if keyEnd > PageSize {
			keyEnd = PageSize
		}
		keyLen := keyEnd - keyStart

		if it.HasNulls() {
			// Null bitmap is right after the header, before key data
			// For index tuples, bitmap size depends on number of index columns
			// We don't know the column count, so just note it
			fmt.Println("    (has null bitmap before key data)")
		}

		if keyLen > 0 {
			fmt.Printf("    Key data (%d bytes):\n", keyLen)
			printHexBlock(p.Data[keyStart:keyEnd], keyStart, "      ")
			if strs := extractPrintable(p.Data[keyStart:keyEnd]); len(strs) > 0 {
				fmt.Println("    Printable strings:")
				for _, s := range strs {
					fmt.Printf("      \"%s\"\n", s)
				}
			}
		}
	}
}

// isMeta checks if the current page is a meta page for its index type.
func isMeta(p *Page) bool {
	special := p.SpecialData()
	if special == nil {
		return false
	}
	le := binary.LittleEndian

	switch p.Detected {
	case PageTypeBTree:
		if len(special) >= 14 {
			return le.Uint16(special[12:14])&BTPMeta != 0
		}
	case PageTypeHash:
		if len(special) >= 14 {
			return le.Uint16(special[12:14])&LHMetaPage != 0
		}
	case PageTypeGIN:
		if len(special) >= 8 {
			return le.Uint16(special[6:8])&GINMeta != 0
		}
	case PageTypeSPGiST:
		if len(special) >= 2 {
			return le.Uint16(special[0:2])&SPGistMeta != 0
		}
	case PageTypeBRIN:
		if len(special) >= 8 {
			return le.Uint16(special[6:8]) == BRINPageTypeMeta
		}
	}
	return false
}

func printHexBlock(data []byte, baseOffset int, indent string) {
	for i := 0; i < len(data); i += 16 {
		fmt.Printf("%s%08x: ", indent, baseOffset+i)
		end := i + 16
		if end > len(data) {
			end = len(data)
		}
		for j := i; j < end; j++ {
			if j == i+8 {
				fmt.Print(" ")
			}
			fmt.Printf("%02x ", data[j])
		}
		for j := end; j < i+16; j++ {
			if j == i+8 {
				fmt.Print(" ")
			}
			fmt.Print("   ")
		}
		fmt.Print(" |")
		for j := i; j < end; j++ {
			b := data[j]
			if b >= 0x20 && b <= 0x7e {
				fmt.Printf("%c", b)
			} else {
				fmt.Print(".")
			}
		}
		fmt.Println("|")
	}
}

func extractPrintable(data []byte) []string {
	var result []string
	var current []byte
	for _, b := range data {
		if b >= 0x20 && b <= 0x7e {
			current = append(current, b)
		} else {
			if len(current) >= 3 {
				result = append(result, string(current))
			}
			current = nil
		}
	}
	if len(current) >= 3 {
		result = append(result, string(current))
	}
	return result
}
