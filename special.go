package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

// DecodeBTreeSpecial decodes BTPageOpaqueData (16 bytes).
func DecodeBTreeSpecial(data []byte) {
	if len(data) < BTreeOpaqueSize {
		fmt.Println("  [B-tree special too short]")
		return
	}
	le := binary.LittleEndian
	prev := le.Uint32(data[0:4])
	next := le.Uint32(data[4:8])
	level := le.Uint32(data[8:12])
	flags := le.Uint16(data[12:14])
	cycleID := le.Uint16(data[14:16])

	fmt.Println("  B-tree Page Opaque Data (BTPageOpaqueData):")
	fmt.Printf("    btpo_prev    : %s\n", blockStr(prev))
	fmt.Printf("    btpo_next    : %s\n", blockStr(next))
	fmt.Printf("    btpo_level   : %d", level)
	if level == 0 {
		fmt.Print(" (leaf)")
	}
	fmt.Println()
	fmt.Printf("    btpo_flags   : 0x%04X", flags)
	if fl := btreeFlags(flags); len(fl) > 0 {
		fmt.Printf(" [%s]", strings.Join(fl, " | "))
	}
	fmt.Println()
	fmt.Printf("    btpo_cycleid : %d\n", cycleID)
}

func btreeFlags(f uint16) []string {
	var fl []string
	if f&BTPLeaf != 0 {
		fl = append(fl, "BTP_LEAF")
	}
	if f&BTPRoot != 0 {
		fl = append(fl, "BTP_ROOT")
	}
	if f&BTPDeleted != 0 {
		fl = append(fl, "BTP_DELETED")
	}
	if f&BTPMeta != 0 {
		fl = append(fl, "BTP_META")
	}
	if f&BTPHalfDead != 0 {
		fl = append(fl, "BTP_HALF_DEAD")
	}
	if f&BTPSplitEnd != 0 {
		fl = append(fl, "BTP_SPLIT_END")
	}
	if f&BTPHasGarbage != 0 {
		fl = append(fl, "BTP_HAS_GARBAGE")
	}
	if f&BTPIncompleteSplit != 0 {
		fl = append(fl, "BTP_INCOMPLETE_SPLIT")
	}
	if f&BTPHasFullXID != 0 {
		fl = append(fl, "BTP_HAS_FULLXID")
	}
	return fl
}

// DecodeBTreeMeta decodes BTMetaPageData from the page content area (after header).
func DecodeBTreeMeta(p *Page) {
	// Meta page content starts at MAXALIGN(SizeOfPageHeaderData) = 24 rounded to 8 = 24
	// Actually MAXALIGN(24) = 24 on 8-byte aligned systems
	offset := 24 // MAXALIGN(PageHeaderSize)
	if offset+44 > PageSize {
		return
	}
	d := p.Data[offset:]
	le := binary.LittleEndian

	magic := le.Uint32(d[0:4])
	version := le.Uint32(d[4:8])
	root := le.Uint32(d[8:12])
	level := le.Uint32(d[12:16])
	fastroot := le.Uint32(d[16:20])
	fastlevel := le.Uint32(d[20:24])

	fmt.Println()
	fmt.Println("  B-tree Meta Page Data (BTMetaPageData):")
	fmt.Printf("    btm_magic          : 0x%06X", magic)
	if magic == BTreeMagic {
		fmt.Print(" (valid)")
	} else {
		fmt.Print(" (INVALID!)")
	}
	fmt.Println()
	fmt.Printf("    btm_version        : %d\n", version)
	fmt.Printf("    btm_root           : %s\n", blockStr(root))
	fmt.Printf("    btm_level          : %d\n", level)
	fmt.Printf("    btm_fastroot       : %s\n", blockStr(fastroot))
	fmt.Printf("    btm_fastlevel      : %d\n", fastlevel)
}

// DecodeHashSpecial decodes HashPageOpaqueData (16 bytes).
func DecodeHashSpecial(data []byte) {
	if len(data) < HashOpaqueSize {
		fmt.Println("  [Hash special too short]")
		return
	}
	le := binary.LittleEndian
	prevblkno := le.Uint32(data[0:4])
	nextblkno := le.Uint32(data[4:8])
	bucket := le.Uint32(data[8:12])
	flag := le.Uint16(data[12:14])
	pageID := le.Uint16(data[14:16])

	fmt.Println("  Hash Page Opaque Data (HashPageOpaqueData):")
	fmt.Printf("    hasho_prevblkno : %s\n", blockStr(prevblkno))
	fmt.Printf("    hasho_nextblkno : %s\n", blockStr(nextblkno))
	fmt.Printf("    hasho_bucket    : %d\n", bucket)
	fmt.Printf("    hasho_flag      : 0x%04X", flag)
	if fl := hashFlags(flag); len(fl) > 0 {
		fmt.Printf(" [%s]", strings.Join(fl, " | "))
	}
	fmt.Println()
	fmt.Printf("    hasho_page_id   : 0x%04X", pageID)
	if pageID == HashPageID {
		fmt.Print(" (HASHO_PAGE_ID)")
	}
	fmt.Println()
}

func hashFlags(f uint16) []string {
	var fl []string
	pageType := f & 0x000F
	switch pageType {
	case LHOverflowPage:
		fl = append(fl, "LH_OVERFLOW_PAGE")
	case LHBucketPage:
		fl = append(fl, "LH_BUCKET_PAGE")
	case LHBitmapPage:
		fl = append(fl, "LH_BITMAP_PAGE")
	case LHMetaPage:
		fl = append(fl, "LH_META_PAGE")
	case 0:
		fl = append(fl, "LH_UNUSED_PAGE")
	}
	if f&LHBucketBeingPopulated != 0 {
		fl = append(fl, "LH_BUCKET_BEING_POPULATED")
	}
	if f&LHBucketBeingSplit != 0 {
		fl = append(fl, "LH_BUCKET_BEING_SPLIT")
	}
	if f&LHBucketNeedsSplitCleanup != 0 {
		fl = append(fl, "LH_BUCKET_NEEDS_SPLIT_CLEANUP")
	}
	if f&LHPageHasDeadTuples != 0 {
		fl = append(fl, "LH_PAGE_HAS_DEAD_TUPLES")
	}
	return fl
}

// DecodeHashMeta decodes HashMetaPageData from the page content area.
func DecodeHashMeta(p *Page) {
	offset := 24
	if offset+64 > PageSize {
		return
	}
	d := p.Data[offset:]
	le := binary.LittleEndian

	magic := le.Uint32(d[0:4])
	version := le.Uint32(d[4:8])
	// ntuples is float64 at offset 8
	ntuples := binary.LittleEndian.Uint64(d[8:16])
	ffactor := le.Uint16(d[16:18])
	bsize := le.Uint16(d[18:20])
	bmsize := le.Uint16(d[20:22])
	bmshift := le.Uint16(d[22:24])
	maxbucket := le.Uint32(d[24:28])
	highmask := le.Uint32(d[28:32])
	lowmask := le.Uint32(d[32:36])
	ovflpoint := le.Uint32(d[36:40])
	firstfree := le.Uint32(d[40:44])
	nmaps := le.Uint32(d[44:48])

	fmt.Println()
	fmt.Println("  Hash Meta Page Data (HashMetaPageData):")
	fmt.Printf("    hashm_magic      : 0x%07X", magic)
	if magic == HashMagic {
		fmt.Print(" (valid)")
	} else {
		fmt.Print(" (INVALID!)")
	}
	fmt.Println()
	fmt.Printf("    hashm_version    : %d\n", version)
	fmt.Printf("    hashm_ntuples    : %f\n", float64FromBits(ntuples))
	fmt.Printf("    hashm_ffactor    : %d\n", ffactor)
	fmt.Printf("    hashm_bsize      : %d\n", bsize)
	fmt.Printf("    hashm_bmsize     : %d\n", bmsize)
	fmt.Printf("    hashm_bmshift    : %d\n", bmshift)
	fmt.Printf("    hashm_maxbucket  : %d\n", maxbucket)
	fmt.Printf("    hashm_highmask   : 0x%08X\n", highmask)
	fmt.Printf("    hashm_lowmask    : 0x%08X\n", lowmask)
	fmt.Printf("    hashm_ovflpoint  : %d\n", ovflpoint)
	fmt.Printf("    hashm_firstfree  : %d\n", firstfree)
	fmt.Printf("    hashm_nmaps      : %d\n", nmaps)
}

func float64FromBits(bits uint64) float64 {
	return math.Float64frombits(bits)
}

// DecodeGiSTSpecial decodes GISTPageOpaqueData (16 bytes).
func DecodeGiSTSpecial(data []byte) {
	if len(data) < GistOpaqueSize {
		fmt.Println("  [GiST special too short]")
		return
	}
	le := binary.LittleEndian
	// nsn: PageXLogRecPtr (8 bytes)
	nsnLo := le.Uint32(data[0:4])
	nsnHi := le.Uint32(data[4:8])
	nsn := uint64(nsnLo)<<32 | uint64(nsnHi)
	rightlink := le.Uint32(data[8:12])
	flags := le.Uint16(data[12:14])
	pageID := le.Uint16(data[14:16])

	fmt.Println("  GiST Page Opaque Data (GISTPageOpaqueData):")
	fmt.Printf("    nsn          : %X/%08X\n", nsn>>32, nsn&0xFFFFFFFF)
	fmt.Printf("    rightlink    : %s\n", blockStr(rightlink))
	fmt.Printf("    flags        : 0x%04X", flags)
	if fl := gistFlags(flags); len(fl) > 0 {
		fmt.Printf(" [%s]", strings.Join(fl, " | "))
	}
	fmt.Println()
	fmt.Printf("    gist_page_id : 0x%04X", pageID)
	if pageID == GistPageID {
		fmt.Print(" (GIST_PAGE_ID)")
	}
	fmt.Println()
}

func gistFlags(f uint16) []string {
	var fl []string
	if f&GistFLeaf != 0 {
		fl = append(fl, "F_LEAF")
	}
	if f&GistFDeleted != 0 {
		fl = append(fl, "F_DELETED")
	}
	if f&GistFTuplesDeleted != 0 {
		fl = append(fl, "F_TUPLES_DELETED")
	}
	if f&GistFFollowRight != 0 {
		fl = append(fl, "F_FOLLOW_RIGHT")
	}
	if f&GistFHasGarbage != 0 {
		fl = append(fl, "F_HAS_GARBAGE")
	}
	return fl
}

// DecodeGINSpecial decodes GinPageOpaqueData (8 bytes).
func DecodeGINSpecial(data []byte) {
	if len(data) < GINOpaqueSize {
		fmt.Println("  [GIN special too short]")
		return
	}
	le := binary.LittleEndian
	rightlink := le.Uint32(data[0:4])
	maxoff := le.Uint16(data[4:6])
	flags := le.Uint16(data[6:8])

	fmt.Println("  GIN Page Opaque Data (GinPageOpaqueData):")
	fmt.Printf("    rightlink    : %s\n", blockStr(rightlink))
	fmt.Printf("    maxoff       : %d\n", maxoff)
	fmt.Printf("    flags        : 0x%04X", flags)
	if fl := ginFlags(flags); len(fl) > 0 {
		fmt.Printf(" [%s]", strings.Join(fl, " | "))
	}
	fmt.Println()
}

func ginFlags(f uint16) []string {
	var fl []string
	if f&GINData != 0 {
		fl = append(fl, "GIN_DATA")
	}
	if f&GINLeaf != 0 {
		fl = append(fl, "GIN_LEAF")
	}
	if f&GINDeleted != 0 {
		fl = append(fl, "GIN_DELETED")
	}
	if f&GINMeta != 0 {
		fl = append(fl, "GIN_META")
	}
	if f&GINList != 0 {
		fl = append(fl, "GIN_LIST")
	}
	if f&GINListFullRow != 0 {
		fl = append(fl, "GIN_LIST_FULLROW")
	}
	if f&GINIncompleteSplit != 0 {
		fl = append(fl, "GIN_INCOMPLETE_SPLIT")
	}
	if f&GINCompressed != 0 {
		fl = append(fl, "GIN_COMPRESSED")
	}
	return fl
}

// DecodeGINMeta decodes GinMetaPageData from the page content area.
// C struct layout on x86-64 with alignment padding:
//   head(4) tail(4) tailFreeSize(4) nPendingPages(4)
//   nPendingHeapTuples(8)
//   nTotalPages(4) nEntryPages(4) nDataPages(4) [pad 4]
//   nEntries(8)
func DecodeGINMeta(p *Page) {
	offset := 24
	if offset+48 > PageSize {
		return
	}
	d := p.Data[offset:]
	le := binary.LittleEndian

	head := le.Uint32(d[0:4])
	tail := le.Uint32(d[4:8])
	tailFreeSize := le.Uint32(d[8:12])
	nPendingPages := le.Uint32(d[12:16])
	nPendingHeapTuples := int64(le.Uint64(d[16:24]))
	nTotalPages := le.Uint32(d[24:28])
	nEntryPages := le.Uint32(d[28:32])
	nDataPages := le.Uint32(d[32:36])
	// 4 bytes padding at d[36:40] for int64 alignment
	nEntries := int64(le.Uint64(d[40:48]))

	fmt.Println()
	fmt.Println("  GIN Meta Page Data (GinMetaPageData):")
	fmt.Printf("    head                : %s\n", blockStr(head))
	fmt.Printf("    tail                : %s\n", blockStr(tail))
	fmt.Printf("    tailFreeSize        : %d\n", tailFreeSize)
	fmt.Printf("    nPendingPages       : %d\n", nPendingPages)
	fmt.Printf("    nPendingHeapTuples  : %d\n", nPendingHeapTuples)
	fmt.Printf("    nTotalPages         : %d\n", nTotalPages)
	fmt.Printf("    nEntryPages         : %d\n", nEntryPages)
	fmt.Printf("    nDataPages          : %d\n", nDataPages)
	fmt.Printf("    nEntries            : %d\n", nEntries)
}

// DecodeSPGiSTSpecial decodes SpGistPageOpaqueData (8 bytes).
func DecodeSPGiSTSpecial(data []byte) {
	if len(data) < SPGistOpaqueSize {
		fmt.Println("  [SP-GiST special too short]")
		return
	}
	le := binary.LittleEndian
	flags := le.Uint16(data[0:2])
	nRedirection := le.Uint16(data[2:4])
	nPlaceholder := le.Uint16(data[4:6])
	pageID := le.Uint16(data[6:8])

	fmt.Println("  SP-GiST Page Opaque Data (SpGistPageOpaqueData):")
	fmt.Printf("    flags          : 0x%04X", flags)
	if fl := spgistFlags(flags); len(fl) > 0 {
		fmt.Printf(" [%s]", strings.Join(fl, " | "))
	}
	fmt.Println()
	fmt.Printf("    nRedirection   : %d\n", nRedirection)
	fmt.Printf("    nPlaceholder   : %d\n", nPlaceholder)
	fmt.Printf("    spgist_page_id : 0x%04X", pageID)
	if pageID == SPGistPageID {
		fmt.Print(" (SPGIST_PAGE_ID)")
	}
	fmt.Println()
}

func spgistFlags(f uint16) []string {
	var fl []string
	if f&SPGistMeta != 0 {
		fl = append(fl, "SPGIST_META")
	}
	if f&SPGistDeleted != 0 {
		fl = append(fl, "SPGIST_DELETED")
	}
	if f&SPGistLeaf != 0 {
		fl = append(fl, "SPGIST_LEAF")
	}
	if f&SPGistNulls != 0 {
		fl = append(fl, "SPGIST_NULLS")
	}
	return fl
}

// DecodeBRINSpecial decodes BrinSpecialSpace (8 bytes).
func DecodeBRINSpecial(data []byte) {
	if len(data) < BRINSpecialSize {
		fmt.Println("  [BRIN special too short]")
		return
	}
	le := binary.LittleEndian
	// vector[4] of uint16: [0],[1], flags=[2], type=[3]
	flags := le.Uint16(data[4:6])
	pageType := le.Uint16(data[6:8])

	fmt.Println("  BRIN Special Space (BrinSpecialSpace):")
	fmt.Printf("    flags     : 0x%04X", flags)
	if flags&BRINEvacuatePage != 0 {
		fmt.Print(" [BRIN_EVACUATE_PAGE]")
	}
	fmt.Println()
	fmt.Printf("    page_type : 0x%04X", pageType)
	switch pageType {
	case BRINPageTypeMeta:
		fmt.Print(" (BRIN_PAGETYPE_META)")
	case BRINPageTypeRevmap:
		fmt.Print(" (BRIN_PAGETYPE_REVMAP)")
	case BRINPageTypeRegular:
		fmt.Print(" (BRIN_PAGETYPE_REGULAR)")
	}
	fmt.Println()
}

// DecodeBRINMeta decodes BrinMetaPageData from the page content area.
func DecodeBRINMeta(p *Page) {
	offset := 24
	if offset+16 > PageSize {
		return
	}
	d := p.Data[offset:]
	le := binary.LittleEndian

	magic := le.Uint32(d[0:4])
	version := le.Uint32(d[4:8])
	pagesPerRange := le.Uint32(d[8:12])
	lastRevmapPage := le.Uint32(d[12:16])

	fmt.Println()
	fmt.Println("  BRIN Meta Page Data (BrinMetaPageData):")
	fmt.Printf("    brinMagic        : 0x%08X", magic)
	if magic == BRINMetaMagic {
		fmt.Print(" (valid)")
	} else {
		fmt.Print(" (INVALID!)")
	}
	fmt.Println()
	fmt.Printf("    brinVersion      : %d\n", version)
	fmt.Printf("    pagesPerRange    : %d\n", pagesPerRange)
	fmt.Printf("    lastRevmapPage   : %d\n", lastRevmapPage)
}
