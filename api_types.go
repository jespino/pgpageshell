package main

import (
	"encoding/binary"
	"fmt"
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

type PageDetail struct {
	PageNum      int               `json:"page_num"`
	Type         string            `json:"type"`
	Header       map[string]string `json:"header"`
	Regions      []PageRegion      `json:"regions"`
	LinePointers []LinePointerInfo `json:"line_pointers"`
	Tuples       []TupleInfo       `json:"tuples"`
	SpecialInfo  map[string]string `json:"special_info,omitempty"`
}

type FileEntry struct {
	Index      int    `json:"index"`
	Filename   string `json:"filename"`
	TotalPages int    `json:"total_pages"`
}

func buildPageDetail(p *Page) PageDetail {
	h := &p.Header
	pageSize := int(h.PageSz())
	if pageSize == 0 {
		pageSize = PageSize
	}

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

	linePointers := make([]LinePointerInfo, len(p.Items))
	for i, lp := range p.Items {
		linePointers[i] = LinePointerInfo{
			Index:  i + 1,
			Status: lp.FlagsStr(),
			Offset: int(lp.Offset()),
			Length: int(lp.Length()),
		}
	}

	isIndex := p.Detected != PageTypeHeap && p.Detected != PageTypeUnknown
	tuples := buildTupleInfos(p, isIndex)
	specialInfo := buildSpecialInfo(p)

	return PageDetail{
		PageNum:      p.PageNum,
		Type:         p.Detected.String(),
		Header:       headerMap,
		Regions:      regions,
		LinePointers: linePointers,
		Tuples:       tuples,
		SpecialInfo:  specialInfo,
	}
}

func buildTupleInfos(p *Page, isIndex bool) []TupleInfo {
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
				ti.Properties["t_tid"] = fmt.Sprintf("(%d, %d)", it.TidBlock, it.TidOffset)
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

func buildSpecialInfo(p *Page) map[string]string {
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
	}

	return info
}
