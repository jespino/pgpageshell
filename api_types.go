package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

var binLE = binary.LittleEndian

// JSON response types shared between web server and Wails bindings.

type PageSummary struct {
	PageNum     int    `json:"page_num"`
	Type        string `json:"type"`
	NumItems    int    `json:"num_items"`
	FreeSpace   int    `json:"free_space"`
	SpecialSize int    `json:"special_size"`
}

type FileInfo struct {
	Filename   string        `json:"filename"`
	TotalPages int           `json:"total_pages"`
	FileType   string        `json:"file_type"`
	Pages      []PageSummary `json:"pages"`
}

type PageRegion struct {
	Name       string `json:"name"`
	StartByte  int    `json:"start_byte"`
	EndByte    int    `json:"end_byte"`
	Size       int    `json:"size"`
	RegionType string `json:"region_type"`
}

type LinePointerInfo struct {
	Index  int    `json:"index"`
	Status string `json:"status"`
	Offset int    `json:"offset"`
	Length int    `json:"length"`
}

type TupleInfo struct {
	Index      int               `json:"index"`
	Status     string            `json:"status"`
	Offset     int               `json:"offset"`
	Length     int               `json:"length"`
	StartByte  int               `json:"start_byte"`
	EndByte    int               `json:"end_byte"`
	Properties map[string]string `json:"properties"`
}

// MetaField describes one named field (or array entry) within a
// meta/bitmap/revmap page's content area.
type MetaField struct {
	Name      string `json:"name"`
	Value     string `json:"value"`
	StartByte int    `json:"start_byte"`
	EndByte   int    `json:"end_byte"`
	Size      int    `json:"size"`
}

type PageDetail struct {
	PageNum      int               `json:"page_num"`
	Type         string            `json:"type"`
	PageSubtype  string            `json:"page_subtype,omitempty"`
	Header       map[string]string `json:"header"`
	Regions      []PageRegion      `json:"regions"`
	LinePointers []LinePointerInfo `json:"line_pointers"`
	Tuples       []TupleInfo       `json:"tuples"`
	SpecialInfo  map[string]string `json:"special_info,omitempty"`
	MetaFields   []MetaField       `json:"meta_fields,omitempty"`
}

type FileEntry struct {
	Index      int    `json:"index"`
	Filename   string `json:"filename"`
	TotalPages int    `json:"total_pages"`
	FileType   string `json:"file_type"`
}

func buildPageDetail(p *Page) PageDetail {
	h := &p.Header
	pageSize := int(h.PageSz())
	if pageSize == 0 {
		pageSize = PageSize
	}

	subtype := detectPageSubtype(p)

	headerEnd := PageHeaderSize
	linpEnd := int(h.Lower)
	freeEnd := int(h.Upper)
	tupleEnd := int(h.Special)
	specialEnd := pageSize

	regions := []PageRegion{}

	regions = append(regions, PageRegion{
		Name:       "Page Header",
		StartByte:  0,
		EndByte:    headerEnd,
		Size:       headerEnd,
		RegionType: "header",
	})

	// For meta/bitmap/revmap pages the bytes between header and special
	// are structured data, not line pointers + tuples.
	specialSubtype := subtype == "meta" || subtype == "bitmap" || subtype == "revmap"

	if specialSubtype {
		// Determine the extent of the structured content.
		// BRIN revmap pages have pd_lower=24 (no adjustment) but the
		// revmap array fills from header to pd_special.
		contentEnd := linpEnd
		if subtype == "revmap" && linpEnd <= headerEnd {
			contentEnd = tupleEnd // data fills up to special
		}

		if contentEnd > headerEnd {
			regionType := subtype // "meta", "bitmap", or "revmap"
			name := specialSubtypeRegionName(p.Detected, subtype, contentEnd-headerEnd)
			regions = append(regions, PageRegion{
				Name:       name,
				StartByte:  headerEnd,
				EndByte:    contentEnd,
				Size:       contentEnd - headerEnd,
				RegionType: regionType,
			})

			// Free space after the structured content
			if freeEnd > contentEnd {
				regions = append(regions, PageRegion{
					Name:       "Free Space",
					StartByte:  contentEnd,
					EndByte:    freeEnd,
					Size:       freeEnd - contentEnd,
					RegionType: "free",
				})
			}
		}
	} else {
		// Normal slotted-page layout: line pointers, free space, tuples
		if linpEnd > headerEnd {
			nItems := (linpEnd - headerEnd) / ItemIdSize
			regions = append(regions, PageRegion{
				Name:       fmt.Sprintf("Line Pointers (%d)", nItems),
				StartByte:  headerEnd,
				EndByte:    linpEnd,
				Size:       linpEnd - headerEnd,
				RegionType: "linp",
			})
		}

		if freeEnd > linpEnd {
			regions = append(regions, PageRegion{
				Name:       "Free Space",
				StartByte:  linpEnd,
				EndByte:    freeEnd,
				Size:       freeEnd - linpEnd,
				RegionType: "free",
			})
		}

		if tupleEnd > freeEnd {
			tupleLabel := "Tuples"
			if p.Detected == PageTypeHeap {
				tupleLabel = "Heap Tuples"
			} else if p.Detected != PageTypeUnknown {
				tupleLabel = fmt.Sprintf("Index Tuples (%s)", p.Detected)
			}
			regions = append(regions, PageRegion{
				Name:       tupleLabel,
				StartByte:  freeEnd,
				EndByte:    tupleEnd,
				Size:       tupleEnd - freeEnd,
				RegionType: "tuple",
			})
		}
	}

	if specialEnd > tupleEnd {
		regions = append(regions, PageRegion{
			Name:       fmt.Sprintf("Special (%s)", p.Detected),
			StartByte:  tupleEnd,
			EndByte:    specialEnd,
			Size:       specialEnd - tupleEnd,
			RegionType: "special",
		})
	}

	headerMap := map[string]string{
		"pd_lsn":              fmt.Sprintf("%X/%08X", h.LSN>>32, h.LSN&0xFFFFFFFF),
		"pd_checksum":         fmt.Sprintf("0x%04X", h.Checksum),
		"pd_flags":            fmt.Sprintf("0x%04X [%s]", h.Flags, FlagsString(h.Flags)),
		"pd_lower":            fmt.Sprintf("%d", h.Lower),
		"pd_upper":            fmt.Sprintf("%d", h.Upper),
		"pd_special":          fmt.Sprintf("%d", h.Special),
		"pd_pagesize_version": fmt.Sprintf("size=%d, version=%d", h.PageSz(), h.LayoutVersion()),
		"pd_prune_xid":        fmt.Sprintf("%d", h.PruneXID),
	}

	var linePointers []LinePointerInfo
	if specialSubtype {
		linePointers = []LinePointerInfo{}
	} else {
		linePointers = make([]LinePointerInfo, len(p.Items))
		for i, lp := range p.Items {
			linePointers[i] = LinePointerInfo{
				Index:  i + 1,
				Status: lp.FlagsStr(),
				Offset: int(lp.Offset()),
				Length: int(lp.Length()),
			}
		}
	}

	isIndex := p.Detected != PageTypeHeap && p.Detected != PageTypeUnknown
	var tuples []TupleInfo
	if specialSubtype {
		tuples = []TupleInfo{}
	} else {
		tuples = buildTupleInfos(p, isIndex, subtype)
	}
	specialInfo := buildSpecialInfo(p, subtype)

	var metaFields []MetaField
	if specialSubtype {
		metaFields = buildMetaFields(p, subtype)
	}

	return PageDetail{
		PageNum:      p.PageNum,
		Type:         p.Detected.String(),
		PageSubtype:  subtype,
		Header:       headerMap,
		Regions:      regions,
		LinePointers: linePointers,
		Tuples:       tuples,
		SpecialInfo:  specialInfo,
		MetaFields:   metaFields,
	}
}

func specialSubtypeRegionName(pageType PageType, subtype string, size int) string {
	switch subtype {
	case "meta":
		return fmt.Sprintf("%s Meta Data (%d bytes)", pageType, size)
	case "bitmap":
		bits := size * 8
		return fmt.Sprintf("Overflow Bitmap (%d bytes, %d bits)", size, bits)
	case "revmap":
		entries := size / 6 // each ItemPointerData is 6 bytes
		return fmt.Sprintf("Revmap Entries (%d entries, %d bytes)", entries, size)
	}
	return fmt.Sprintf("%s (%d bytes)", subtype, size)
}

// buildMetaFields returns per-field (or per-entry) metadata for
// meta/bitmap/revmap pages so the frontend can show individual
// cell-level tooltips instead of one opaque block.
func buildMetaFields(p *Page, subtype string) []MetaField {
	d := p.Data[:]
	le := binLE
	base := PageHeaderSize // content starts at offset 24

	switch subtype {
	case "meta":
		return buildMetaStructFields(p, d, le, base)
	case "bitmap":
		return buildBitmapFields(p, d, le, base)
	case "revmap":
		return buildRevmapFields(p, d, le, base)
	}
	return nil
}

func buildMetaStructFields(p *Page, d []byte, le binary.ByteOrder, base int) []MetaField {
	switch p.Detected {
	case PageTypeBTree:
		if len(d) < base+44 {
			return nil
		}
		return []MetaField{
			metaU32(d, le, base, 0, "btm_magic", "0x%08X"),
			metaU32(d, le, base, 4, "btm_version", "%d"),
			metaU32(d, le, base, 8, "btm_root", "%d"),
			metaU32(d, le, base, 12, "btm_level", "%d"),
			metaU32(d, le, base, 16, "btm_fastroot", "%d"),
			metaU32(d, le, base, 20, "btm_fastlevel", "%d"),
			metaU32(d, le, base, 24, "btm_last_cleanup_num_delpages", "%d"),
			{
				Name:      "padding",
				Value:     "",
				StartByte: base + 28,
				EndByte:   base + 32,
				Size:      4,
			},
			metaF64(d, le, base, 32, "btm_last_cleanup_num_heap_tuples"),
			metaBool(d, base, 40, "btm_allequalimage"),
			{
				Name:      "padding",
				Value:     "",
				StartByte: base + 41,
				EndByte:   base + 48,
				Size:      7,
			},
		}

	case PageTypeHash:
		if len(d) < base+48 {
			return nil
		}
		fields := []MetaField{
			metaU32(d, le, base, 0, "hashm_magic", "0x%08X"),
			metaU32(d, le, base, 4, "hashm_version", "%d"),
			metaF64(d, le, base, 8, "hashm_ntuples"),
			metaU16(d, le, base, 16, "hashm_ffactor", "%d"),
			metaU16(d, le, base, 18, "hashm_bsize", "%d"),
			metaU16(d, le, base, 20, "hashm_bmsize", "%d"),
			metaU16(d, le, base, 22, "hashm_bmshift", "%d"),
			metaU32(d, le, base, 24, "hashm_maxbucket", "%d"),
			metaU32(d, le, base, 28, "hashm_highmask", "0x%08X"),
			metaU32(d, le, base, 32, "hashm_lowmask", "0x%08X"),
			metaU32(d, le, base, 36, "hashm_ovflpoint", "%d"),
			metaU32(d, le, base, 40, "hashm_firstfree", "%d"),
			metaU32(d, le, base, 44, "hashm_nmaps", "%d"),
			metaU32(d, le, base, 48, "hashm_procid", "%d"),
		}
		// hashm_spares[HASH_MAX_SPLITPOINTS] = uint32[32] at offset 52
		sparesOff := 52
		if len(d) >= base+sparesOff+32*4 {
			fields = append(fields, MetaField{
				Name:      "hashm_spares[32]",
				Value:     fmt.Sprintf("uint32[32] array"),
				StartByte: base + sparesOff,
				EndByte:   base + sparesOff + 32*4,
				Size:      32 * 4,
			})
		}
		// hashm_mapp[HASH_MAX_BITMAPS] at offset 180
		mappOff := 52 + 32*4
		linpEnd := int(p.Header.Lower)
		if linpEnd > base+mappOff {
			mappSize := linpEnd - (base + mappOff)
			nMaps := mappSize / 4
			fields = append(fields, MetaField{
				Name:      fmt.Sprintf("hashm_mapp[%d]", nMaps),
				Value:     fmt.Sprintf("BlockNumber[%d] array", nMaps),
				StartByte: base + mappOff,
				EndByte:   linpEnd,
				Size:      mappSize,
			})
		}
		return fields

	case PageTypeGIN:
		if len(d) < base+48 {
			return nil
		}
		return []MetaField{
			metaU32(d, le, base, 0, "head", "%d"),
			metaU32(d, le, base, 4, "tail", "%d"),
			metaU32(d, le, base, 8, "tailFreeSize", "%d"),
			metaU32(d, le, base, 12, "nPendingPages", "%d"),
			metaI64(d, le, base, 16, "nPendingHeapTuples"),
			metaU32(d, le, base, 24, "nTotalPages", "%d"),
			metaU32(d, le, base, 28, "nEntryPages", "%d"),
			metaU32(d, le, base, 32, "nDataPages", "%d"),
			{
				Name:      "padding",
				Value:     "",
				StartByte: base + 36,
				EndByte:   base + 40,
				Size:      4,
			},
			metaI64(d, le, base, 40, "nEntries"),
			metaU32(d, le, base, 48, "ginVersion", "%d"),
			{
				Name:      "padding",
				Value:     "",
				StartByte: base + 52,
				EndByte:   base + 56,
				Size:      4,
			},
		}

	case PageTypeBRIN:
		if len(d) < base+16 {
			return nil
		}
		return []MetaField{
			metaU32(d, le, base, 0, "brinMagic", "0x%08X"),
			metaU32(d, le, base, 4, "brinVersion", "%d"),
			metaU32(d, le, base, 8, "pagesPerRange", "%d"),
			metaU32(d, le, base, 12, "lastRevmapPage", "%d"),
		}

	case PageTypeSPGiST:
		if len(d) < base+8 {
			return nil
		}
		fields := []MetaField{
			metaU32(d, le, base, 0, "magicNumber", "0x%08X"),
			{
				Name:      "padding",
				Value:     "",
				StartByte: base + 4,
				EndByte:   base + 8,
				Size:      4,
			},
		}
		// SpGistLUPCache: 8 entries × 8 bytes each
		lupOff := 8
		if len(d) >= base+lupOff+64 {
			for i := 0; i < 8; i++ {
				off := lupOff + i*8
				blk := le.Uint32(d[base+off : base+off+4])
				offnum := le.Uint16(d[base+off+4 : base+off+6])
				fields = append(fields, MetaField{
					Name:      fmt.Sprintf("lastUsedPages[%d]", i),
					Value:     fmt.Sprintf("block=%d, offset=%d", blk, offnum),
					StartByte: base + off,
					EndByte:   base + off + 8,
					Size:      8,
				})
			}
		}
		return fields
	}
	return nil
}

func buildBitmapFields(p *Page, d []byte, le binary.ByteOrder, base int) []MetaField {
	linpEnd := int(p.Header.Lower)
	if linpEnd <= base {
		return nil
	}
	nWords := (linpEnd - base) / 4
	fields := make([]MetaField, 0, nWords)
	for i := 0; i < nWords; i++ {
		off := base + i*4
		if off+4 > len(d) {
			break
		}
		word := le.Uint32(d[off : off+4])
		set := popcount32(word)
		fields = append(fields, MetaField{
			Name:      fmt.Sprintf("word[%d]", i),
			Value:     fmt.Sprintf("0x%08X (%d/32 bits set)", word, set),
			StartByte: off,
			EndByte:   off + 4,
			Size:      4,
		})
	}
	return fields
}

func buildRevmapFields(p *Page, d []byte, le binary.ByteOrder, base int) []MetaField {
	specialOff := int(p.Header.Special)
	if specialOff <= base {
		return nil
	}
	contentSize := specialOff - base
	nEntries := contentSize / 6
	fields := make([]MetaField, 0, nEntries)
	for i := 0; i < nEntries; i++ {
		off := base + i*6
		if off+6 > len(d) {
			break
		}
		// ItemPointerData: ip_blkid (2×uint16 = BlockIdData) + ip_posid (uint16)
		blkHi := le.Uint16(d[off : off+2])
		blkLo := le.Uint16(d[off+2 : off+4])
		blk := uint32(blkHi)<<16 | uint32(blkLo)
		posid := le.Uint16(d[off+4 : off+6])

		var value string
		if blk == 0 && posid == 0 {
			value = "(invalid)"
		} else {
			value = fmt.Sprintf("(%d, %d)", blk, posid)
		}
		fields = append(fields, MetaField{
			Name:      fmt.Sprintf("range[%d]", i),
			Value:     value,
			StartByte: off,
			EndByte:   off + 6,
			Size:      6,
		})
	}
	return fields
}

// Helper functions for building MetaField entries from raw bytes.

func metaU32(d []byte, le binary.ByteOrder, base, off int, name, format string) MetaField {
	v := le.Uint32(d[base+off : base+off+4])
	return MetaField{
		Name:      name,
		Value:     fmt.Sprintf(format, v),
		StartByte: base + off,
		EndByte:   base + off + 4,
		Size:      4,
	}
}

func metaU16(d []byte, le binary.ByteOrder, base, off int, name, format string) MetaField {
	v := le.Uint16(d[base+off : base+off+2])
	return MetaField{
		Name:      name,
		Value:     fmt.Sprintf(format, v),
		StartByte: base + off,
		EndByte:   base + off + 2,
		Size:      2,
	}
}

func metaF64(d []byte, le binary.ByteOrder, base, off int, name string) MetaField {
	v := math.Float64frombits(le.Uint64(d[base+off : base+off+8]))
	return MetaField{
		Name:      name,
		Value:     fmt.Sprintf("%.0f", v),
		StartByte: base + off,
		EndByte:   base + off + 8,
		Size:      8,
	}
}

func metaI64(d []byte, le binary.ByteOrder, base, off int, name string) MetaField {
	v := int64(le.Uint64(d[base+off : base+off+8]))
	return MetaField{
		Name:      name,
		Value:     fmt.Sprintf("%d", v),
		StartByte: base + off,
		EndByte:   base + off + 8,
		Size:      8,
	}
}

func metaBool(d []byte, base, off int, name string) MetaField {
	v := d[base+off] != 0
	return MetaField{
		Name:      name,
		Value:     fmt.Sprintf("%t", v),
		StartByte: base + off,
		EndByte:   base + off + 1,
		Size:      1,
	}
}

func popcount32(x uint32) int {
	n := 0
	for x != 0 {
		n++
		x &= x - 1
	}
	return n
}

func detectPageSubtype(p *Page) string {
	special := p.SpecialData()
	le := binLE

	switch p.Detected {
	case PageTypeBTree:
		if special != nil && len(special) >= BTreeOpaqueSize {
			flags := le.Uint16(special[12:14])
			if flags&BTPMeta != 0 {
				return "meta"
			}
			if flags&BTPLeaf != 0 {
				return "leaf"
			}
			return "internal"
		}
	case PageTypeHash:
		if special != nil && len(special) >= HashOpaqueSize {
			flag := le.Uint16(special[12:14])
			pageType := flag & 0x000F
			switch pageType {
			case LHMetaPage:
				return "meta"
			case LHBucketPage:
				return "bucket"
			case LHOverflowPage:
				return "overflow"
			case LHBitmapPage:
				return "bitmap"
			}
		}
	case PageTypeGIN:
		if special != nil && len(special) >= GINOpaqueSize {
			flags := le.Uint16(special[6:8])
			if flags&GINMeta != 0 {
				return "meta"
			}
			if flags&GINData != 0 {
				if flags&GINLeaf != 0 {
					return "data-leaf"
				}
				return "data-internal"
			}
			if flags&GINLeaf != 0 {
				return "entry-leaf"
			}
			return "entry-internal"
		}
	case PageTypeSPGiST:
		if special != nil && len(special) >= SPGistOpaqueSize {
			flags := le.Uint16(special[0:2])
			if flags&SPGistMeta != 0 {
				return "meta"
			}
			if flags&SPGistLeaf != 0 {
				return "leaf"
			}
			return "internal"
		}
	case PageTypeBRIN:
		if special != nil && len(special) >= BRINSpecialSize {
			pageType := le.Uint16(special[6:8])
			switch pageType {
			case BRINPageTypeMeta:
				return "meta"
			case BRINPageTypeRevmap:
				return "revmap"
			case BRINPageTypeRegular:
				return "regular"
			}
		}
	case PageTypeGiST:
		if special != nil && len(special) >= GistOpaqueSize {
			flags := le.Uint16(special[12:14])
			if flags&GistFLeaf != 0 {
				return "leaf"
			}
			return "internal"
		}
	}
	return ""
}

func buildTupleInfos(p *Page, isIndex bool, subtype string) []TupleInfo {
	tuples := make([]TupleInfo, 0, len(p.Items))

	for i, lp := range p.Items {
		ti := TupleInfo{
			Index:      i + 1,
			Status:     lp.FlagsStr(),
			Offset:     int(lp.Offset()),
			Length:     int(lp.Length()),
			Properties: map[string]string{},
		}

		if lp.Flags() == LPUnused {
			ti.Properties["note"] = "UNUSED - no data"
			tuples = append(tuples, ti)
			continue
		}
		if lp.Flags() == LPRedirect {
			ti.Properties["redirect_to"] = fmt.Sprintf("%d", lp.Offset())
			tuples = append(tuples, ti)
			continue
		}
		if lp.Flags() == LPDead {
			ti.Properties["note"] = "DEAD"
		}
		if lp.Length() == 0 || lp.Offset() == 0 {
			tuples = append(tuples, ti)
			continue
		}
		if int(lp.Offset())+int(lp.Length()) > PageSize {
			ti.Properties["error"] = "extends beyond page"
			tuples = append(tuples, ti)
			continue
		}

		ti.StartByte = int(lp.Offset())
		ti.EndByte = int(lp.Offset()) + int(lp.Length())

		if isIndex {
			if lp.Length() >= uint16(IndexTupleHdrSize) {
				it := p.ParseIndexTupleHeader(lp.Offset())
				if subtype == "internal" && p.Detected == PageTypeBTree {
					ti.Properties["child_block"] = fmt.Sprintf("%d", it.TidBlock)
				} else {
					ti.Properties["t_tid"] = fmt.Sprintf("(%d, %d)", it.TidBlock, it.TidOffset)
				}
				ti.Properties["t_info"] = fmt.Sprintf("0x%04X (size: %d)", it.Info, it.Size())
				if flags := it.InfoFlags(); len(flags) > 0 {
					ti.Properties["flags"] = strings.Join(flags, " | ")
				}
			}
		} else {
			t := p.ParseHeapTupleHeader(lp.Offset())
			ti.Properties["t_xmin"] = fmt.Sprintf("%d", t.Xmin)
			ti.Properties["t_xmax"] = fmt.Sprintf("%d", t.Xmax)
			ti.Properties["t_cid"] = fmt.Sprintf("%d", t.Field3)
			ti.Properties["t_ctid"] = fmt.Sprintf("(%d, %d)", t.CtidBlock, t.CtidOffset)
			ti.Properties["t_infomask"] = fmt.Sprintf("0x%04X", t.Infomask)
			ti.Properties["t_infomask2"] = fmt.Sprintf("0x%04X (natts: %d)", t.Infomask2, t.NAttrs())
			ti.Properties["t_hoff"] = fmt.Sprintf("%d", t.Hoff)
			if flags := t.InfomaskFlags(); len(flags) > 0 {
				ti.Properties["infomask_flags"] = strings.Join(flags, " | ")
			}
			if flags := t.Infomask2Flags(); len(flags) > 0 {
				ti.Properties["infomask2_flags"] = strings.Join(flags, " | ")
			}
			dataStart := int(lp.Offset()) + int(t.Hoff)
			dataEnd := int(lp.Offset()) + int(lp.Length())
			if dataEnd > PageSize {
				dataEnd = PageSize
			}
			if dataStart < dataEnd {
				if strs := extractPrintable(p.Data[dataStart:dataEnd]); len(strs) > 0 {
					ti.Properties["printable"] = strings.Join(strs, ", ")
				}
			}
		}

		tuples = append(tuples, ti)
	}

	return tuples
}

func buildSpecialInfo(p *Page, subtype string) map[string]string {
	special := p.SpecialData()
	if special == nil || p.SpecialSize() == 0 {
		return nil
	}

	info := map[string]string{
		"size": fmt.Sprintf("%d bytes", p.SpecialSize()),
	}

	switch p.Detected {
	case PageTypeBTree:
		if len(special) >= BTreeOpaqueSize {
			le := binLE
			info["btpo_prev"] = blockStr(le.Uint32(special[0:4]))
			info["btpo_next"] = blockStr(le.Uint32(special[4:8]))
			info["btpo_level"] = fmt.Sprintf("%d", le.Uint32(special[8:12]))
			flags := le.Uint16(special[12:14])
			info["btpo_flags"] = fmt.Sprintf("0x%04X", flags)
			if fl := btreeFlags(flags); len(fl) > 0 {
				info["btpo_flags_decoded"] = strings.Join(fl, " | ")
			}
		}
		if subtype == "meta" && len(p.Data) >= PageHeaderSize+44 {
			d := p.Data[PageHeaderSize:]
			le := binLE
			info["btm_magic"] = fmt.Sprintf("0x%08X", le.Uint32(d[0:4]))
			info["btm_version"] = fmt.Sprintf("%d", le.Uint32(d[4:8]))
			info["btm_root"] = fmt.Sprintf("%d", le.Uint32(d[8:12]))
			info["btm_level"] = fmt.Sprintf("%d", le.Uint32(d[12:16]))
			info["btm_fastroot"] = fmt.Sprintf("%d", le.Uint32(d[16:20]))
			info["btm_fastlevel"] = fmt.Sprintf("%d", le.Uint32(d[20:24]))
		}
	case PageTypeHash:
		if len(special) >= HashOpaqueSize {
			le := binLE
			info["hasho_prevblkno"] = blockStr(le.Uint32(special[0:4]))
			info["hasho_nextblkno"] = blockStr(le.Uint32(special[4:8]))
			info["hasho_bucket"] = fmt.Sprintf("%d", le.Uint32(special[8:12]))
			flag := le.Uint16(special[12:14])
			info["hasho_flag"] = fmt.Sprintf("0x%04X", flag)
			if fl := hashFlags(flag); len(fl) > 0 {
				info["hasho_flag_decoded"] = strings.Join(fl, " | ")
			}
		}
		if subtype == "meta" && len(p.Data) >= PageHeaderSize+48 {
			d := p.Data[PageHeaderSize:]
			le := binLE
			info["hashm_magic"] = fmt.Sprintf("0x%08X", le.Uint32(d[0:4]))
			info["hashm_version"] = fmt.Sprintf("%d", le.Uint32(d[4:8]))
			info["hashm_ntuples"] = fmt.Sprintf("%.0f", math.Float64frombits(le.Uint64(d[8:16])))
			info["hashm_ffactor"] = fmt.Sprintf("%d", le.Uint16(d[16:18]))
			info["hashm_bsize"] = fmt.Sprintf("%d", le.Uint16(d[18:20]))
			info["hashm_maxbucket"] = fmt.Sprintf("%d", le.Uint32(d[24:28]))
			info["hashm_highmask"] = fmt.Sprintf("0x%08X", le.Uint32(d[28:32]))
			info["hashm_lowmask"] = fmt.Sprintf("0x%08X", le.Uint32(d[32:36]))
		}
	case PageTypeGiST:
		if len(special) >= GistOpaqueSize {
			le := binLE
			flags := le.Uint16(special[12:14])
			info["flags"] = fmt.Sprintf("0x%04X", flags)
			if fl := gistFlags(flags); len(fl) > 0 {
				info["flags_decoded"] = strings.Join(fl, " | ")
			}
			info["rightlink"] = blockStr(le.Uint32(special[8:12]))
		}
	case PageTypeGIN:
		if len(special) >= GINOpaqueSize {
			le := binLE
			info["rightlink"] = blockStr(le.Uint32(special[0:4]))
			info["maxoff"] = fmt.Sprintf("%d", le.Uint16(special[4:6]))
			flags := le.Uint16(special[6:8])
			info["flags"] = fmt.Sprintf("0x%04X", flags)
			if fl := ginFlags(flags); len(fl) > 0 {
				info["flags_decoded"] = strings.Join(fl, " | ")
			}
		}
		if subtype == "meta" && len(p.Data) >= PageHeaderSize+48 {
			d := p.Data[PageHeaderSize:]
			le := binLE
			info["head"] = blockStr(le.Uint32(d[0:4]))
			info["tail"] = blockStr(le.Uint32(d[4:8]))
			info["nPendingPages"] = fmt.Sprintf("%d", le.Uint32(d[12:16]))
			info["nPendingHeapTuples"] = fmt.Sprintf("%d", int64(le.Uint64(d[16:24])))
			info["nTotalPages"] = fmt.Sprintf("%d", le.Uint32(d[24:28]))
			info["nEntryPages"] = fmt.Sprintf("%d", le.Uint32(d[28:32]))
			info["nDataPages"] = fmt.Sprintf("%d", le.Uint32(d[32:36]))
			info["nEntries"] = fmt.Sprintf("%d", int64(le.Uint64(d[40:48])))
		}
	case PageTypeSPGiST:
		if len(special) >= SPGistOpaqueSize {
			le := binLE
			flags := le.Uint16(special[0:2])
			info["flags"] = fmt.Sprintf("0x%04X", flags)
			if fl := spgistFlags(flags); len(fl) > 0 {
				info["flags_decoded"] = strings.Join(fl, " | ")
			}
			info["nRedirection"] = fmt.Sprintf("%d", le.Uint16(special[2:4]))
			info["nPlaceholder"] = fmt.Sprintf("%d", le.Uint16(special[4:6]))
		}
	case PageTypeBRIN:
		if len(special) >= BRINSpecialSize {
			le := binLE
			pageType := le.Uint16(special[6:8])
			switch pageType {
			case BRINPageTypeMeta:
				info["page_type"] = "META"
			case BRINPageTypeRevmap:
				info["page_type"] = "REVMAP"
			case BRINPageTypeRegular:
				info["page_type"] = "REGULAR"
			default:
				info["page_type"] = fmt.Sprintf("0x%04X", pageType)
			}
		}
		if subtype == "meta" && len(p.Data) >= PageHeaderSize+16 {
			d := p.Data[PageHeaderSize:]
			le := binLE
			info["brinMagic"] = fmt.Sprintf("0x%08X", le.Uint32(d[0:4]))
			info["brinVersion"] = fmt.Sprintf("%d", le.Uint32(d[4:8]))
			info["pagesPerRange"] = fmt.Sprintf("%d", le.Uint32(d[8:12]))
			info["lastRevmapPage"] = fmt.Sprintf("%d", le.Uint32(d[12:16]))
		}
	}

	return info
}
