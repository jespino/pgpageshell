package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	PageSize          = 8192
	PageHeaderSize    = 24
	ItemIdSize        = 4
	HeapTupleHdrSize  = 23
	IndexTupleHdrSize = 8
	InvalidXID        = uint32(0)
	InvalidBlock      = uint32(0xFFFFFFFF)
)

// ---- Page type identification ----

type PageType int

const (
	PageTypeHeap PageType = iota
	PageTypeBTree
	PageTypeHash
	PageTypeGiST
	PageTypeGIN
	PageTypeSPGiST
	PageTypeBRIN
	PageTypeUnknown
)

func (pt PageType) String() string {
	switch pt {
	case PageTypeHeap:
		return "heap"
	case PageTypeBTree:
		return "btree"
	case PageTypeHash:
		return "hash"
	case PageTypeGiST:
		return "gist"
	case PageTypeGIN:
		return "gin"
	case PageTypeSPGiST:
		return "spgist"
	case PageTypeBRIN:
		return "brin"
	default:
		return "unknown"
	}
}

// ---- Line pointer flags ----

const (
	LPUnused   = 0
	LPNormal   = 1
	LPRedirect = 2
	LPDead     = 3
)

// ---- Heap tuple t_infomask bits ----

const (
	HeapHasNull        = 0x0001
	HeapHasVarWidth    = 0x0002
	HeapHasExternal    = 0x0004
	HeapHasOidOld      = 0x0008
	HeapXmaxKeyShrLock = 0x0010
	HeapComboCID       = 0x0020
	HeapXmaxExclLock   = 0x0040
	HeapXmaxLockOnly   = 0x0080
	HeapXminCommitted  = 0x0100
	HeapXminInvalid    = 0x0200
	HeapXminFrozen     = 0x0300
	HeapXmaxCommitted  = 0x0400
	HeapXmaxInvalid    = 0x0800
	HeapXmaxIsMulti    = 0x1000
	HeapUpdated        = 0x2000
	HeapMovedOff       = 0x4000
	HeapMovedIn        = 0x8000
)

// ---- Heap tuple t_infomask2 bits ----

const (
	HeapNattsMask   = 0x07FF
	HeapKeysUpdated = 0x2000
	HeapHotUpdated  = 0x4000
	HeapOnlyTuple   = 0x8000
)

// ---- Index tuple t_info bits ----

const (
	IndexSizeMask      = 0x1FFF
	IndexAMReservedBit = 0x2000
	IndexVarMask       = 0x4000
	IndexNullMask      = 0x8000
)

// ---- pd_flags bits ----

const (
	PDHasFreeLines = 0x0001
	PDPageFull     = 0x0002
	PDAllVisible   = 0x0004
)

// ---- B-tree constants ----

const (
	BTreeMagic  = 0x053162
	BTreeOpaqueSize = 16

	BTPLeaf            = 0x0001
	BTPRoot            = 0x0002
	BTPDeleted         = 0x0004
	BTPMeta            = 0x0008
	BTPHalfDead        = 0x0010
	BTPSplitEnd        = 0x0020
	BTPHasGarbage      = 0x0040
	BTPIncompleteSplit = 0x0080
	BTPHasFullXID      = 0x0100
)

// ---- Hash constants ----

const (
	HashMagic      = 0x6440640
	HashPageID     = 0xFF80
	HashOpaqueSize = 16

	LHOverflowPage            = 0x0001
	LHBucketPage              = 0x0002
	LHBitmapPage              = 0x0004
	LHMetaPage                = 0x0008
	LHBucketBeingPopulated    = 0x0010
	LHBucketBeingSplit        = 0x0020
	LHBucketNeedsSplitCleanup = 0x0040
	LHPageHasDeadTuples       = 0x0080
)

// ---- GiST constants ----

const (
	GistPageID     = 0xFF81
	GistOpaqueSize = 16

	GistFLeaf          = 0x0001
	GistFDeleted       = 0x0002
	GistFTuplesDeleted = 0x0004
	GistFFollowRight   = 0x0008
	GistFHasGarbage    = 0x0010
)

// ---- GIN constants ----

const (
	GINOpaqueSize = 8

	GINData            = 0x0001
	GINLeaf            = 0x0002
	GINDeleted         = 0x0004
	GINMeta            = 0x0008
	GINList            = 0x0010
	GINListFullRow     = 0x0020
	GINIncompleteSplit = 0x0040
	GINCompressed      = 0x0080
)

// ---- SP-GiST constants ----

const (
	SPGistPageID     = 0xFF82
	SPGistOpaqueSize = 8

	SPGistMeta    = 0x0001
	SPGistDeleted = 0x0002
	SPGistLeaf    = 0x0004
	SPGistNulls   = 0x0008
)

// ---- BRIN constants ----

const (
	BRINPageTypeMeta    = 0xF091
	BRINPageTypeRevmap  = 0xF092
	BRINPageTypeRegular = 0xF093
	BRINEvacuatePage    = 0x0001
	BRINMetaMagic       = 0xA8109CFA
	BRINSpecialSize     = 8
)

// ---- Structures ----

type PageHeader struct {
	LSN         uint64
	Checksum    uint16
	Flags       uint16
	Lower       uint16
	Upper       uint16
	Special     uint16
	PageSizeVer uint16
	PruneXID    uint32
}

func (h *PageHeader) PageSz() uint16  { return h.PageSizeVer & 0xFF00 }
func (h *PageHeader) LayoutVersion() uint8 { return uint8(h.PageSizeVer & 0x00FF) }

type ItemId struct{ Raw uint32 }

func (lp ItemId) Offset() uint16 { return uint16(lp.Raw & 0x7FFF) }
func (lp ItemId) Flags() uint8   { return uint8((lp.Raw >> 15) & 0x03) }
func (lp ItemId) Length() uint16 { return uint16((lp.Raw >> 17) & 0x7FFF) }

func (lp ItemId) FlagsStr() string {
	switch lp.Flags() {
	case LPUnused:
		return "UNUSED"
	case LPNormal:
		return "NORMAL"
	case LPRedirect:
		return "REDIRECT"
	case LPDead:
		return "DEAD"
	default:
		return "UNKNOWN"
	}
}

type HeapTupleHeader struct {
	Xmin, Xmax, Field3 uint32
	CtidBlock          uint32
	CtidOffset         uint16
	Infomask2          uint16
	Infomask           uint16
	Hoff               uint8
}

func (t *HeapTupleHeader) NAttrs() int { return int(t.Infomask2 & HeapNattsMask) }

func (t *HeapTupleHeader) InfomaskFlags() []string {
	var flags []string
	m := t.Infomask
	if m&HeapHasNull != 0 {
		flags = append(flags, "HAS_NULL")
	}
	if m&HeapHasVarWidth != 0 {
		flags = append(flags, "HAS_VARWIDTH")
	}
	if m&HeapHasExternal != 0 {
		flags = append(flags, "HAS_EXTERNAL")
	}
	if m&HeapHasOidOld != 0 {
		flags = append(flags, "HAS_OID_OLD")
	}
	if m&HeapXmaxKeyShrLock != 0 {
		flags = append(flags, "XMAX_KEYSHR_LOCK")
	}
	if m&HeapComboCID != 0 {
		flags = append(flags, "COMBO_CID")
	}
	if m&HeapXmaxExclLock != 0 {
		flags = append(flags, "XMAX_EXCL_LOCK")
	}
	if m&HeapXmaxLockOnly != 0 {
		flags = append(flags, "XMAX_LOCK_ONLY")
	}
	xminBits := m & 0x0300
	switch xminBits {
	case HeapXminFrozen:
		flags = append(flags, "XMIN_FROZEN")
	case HeapXminCommitted:
		flags = append(flags, "XMIN_COMMITTED")
	case HeapXminInvalid:
		flags = append(flags, "XMIN_INVALID")
	}
	if m&HeapXmaxCommitted != 0 {
		flags = append(flags, "XMAX_COMMITTED")
	}
	if m&HeapXmaxInvalid != 0 {
		flags = append(flags, "XMAX_INVALID")
	}
	if m&HeapXmaxIsMulti != 0 {
		flags = append(flags, "XMAX_IS_MULTI")
	}
	if m&HeapUpdated != 0 {
		flags = append(flags, "UPDATED")
	}
	if m&HeapMovedOff != 0 {
		flags = append(flags, "MOVED_OFF")
	}
	if m&HeapMovedIn != 0 {
		flags = append(flags, "MOVED_IN")
	}
	return flags
}

func (t *HeapTupleHeader) Infomask2Flags() []string {
	var flags []string
	if t.Infomask2&HeapKeysUpdated != 0 {
		flags = append(flags, "KEYS_UPDATED")
	}
	if t.Infomask2&HeapHotUpdated != 0 {
		flags = append(flags, "HOT_UPDATED")
	}
	if t.Infomask2&HeapOnlyTuple != 0 {
		flags = append(flags, "HEAP_ONLY")
	}
	return flags
}

type IndexTupleHeader struct {
	TidBlock  uint32
	TidOffset uint16
	Info      uint16
}

func (it *IndexTupleHeader) Size() int      { return int(it.Info & IndexSizeMask) }
func (it *IndexTupleHeader) HasNulls() bool  { return it.Info&IndexNullMask != 0 }
func (it *IndexTupleHeader) HasVarWidths() bool { return it.Info&IndexVarMask != 0 }

func (it *IndexTupleHeader) InfoFlags() []string {
	var flags []string
	if it.Info&IndexNullMask != 0 {
		flags = append(flags, "HAS_NULLS")
	}
	if it.Info&IndexVarMask != 0 {
		flags = append(flags, "HAS_VARWIDTH")
	}
	if it.Info&IndexAMReservedBit != 0 {
		flags = append(flags, "AM_RESERVED")
	}
	return flags
}

// Page holds a full 8KB page in memory.
type Page struct {
	Data     [PageSize]byte
	Header   PageHeader
	Items    []ItemId
	PageNum  int
	Detected PageType
}

func ParsePage(data [PageSize]byte) *Page {
	p := &Page{Data: data}
	le := binary.LittleEndian

	xlogid := le.Uint32(data[0:4])
	xrecoff := le.Uint32(data[4:8])
	p.Header.LSN = uint64(xlogid)<<32 | uint64(xrecoff)
	p.Header.Checksum = le.Uint16(data[8:10])
	p.Header.Flags = le.Uint16(data[10:12])
	p.Header.Lower = le.Uint16(data[12:14])
	p.Header.Upper = le.Uint16(data[14:16])
	p.Header.Special = le.Uint16(data[16:18])
	p.Header.PageSizeVer = le.Uint16(data[18:20])
	p.Header.PruneXID = le.Uint32(data[20:24])

	numItems := 0
	if p.Header.Lower > PageHeaderSize {
		numItems = int(p.Header.Lower-PageHeaderSize) / ItemIdSize
	}
	p.Items = make([]ItemId, numItems)
	for i := 0; i < numItems; i++ {
		off := PageHeaderSize + i*ItemIdSize
		p.Items[i] = ItemId{Raw: le.Uint32(data[off : off+4])}
	}

	p.Detected = p.detectPageType()
	return p
}

func (p *Page) detectPageType() PageType {
	h := &p.Header
	pageSize := int(h.PageSz())
	if pageSize == 0 {
		pageSize = PageSize
	}
	specialSize := pageSize - int(h.Special)

	if specialSize == 0 {
		return PageTypeHeap
	}
	if int(h.Special) >= pageSize || h.Special < PageHeaderSize {
		return PageTypeUnknown
	}

	special := p.Data[h.Special:]
	le := binary.LittleEndian

	// 8-byte special: could be BRIN, SP-GiST, or GIN
	if specialSize == 8 {
		// BRIN: page type at vector[3] (offset 6)
		brinType := le.Uint16(special[6:8])
		if brinType == BRINPageTypeMeta || brinType == BRINPageTypeRevmap || brinType == BRINPageTypeRegular {
			return PageTypeBRIN
		}
		// SP-GiST: page_id at offset 6
		spgistID := le.Uint16(special[6:8])
		if spgistID == SPGistPageID {
			return PageTypeSPGiST
		}
		// GIN: flags at offset 6, valid flags in bits 0-7
		ginFlags := le.Uint16(special[6:8])
		if ginFlags == 0 || (ginFlags&0xFF00 == 0 && ginFlags&0x00FF != 0) {
			return PageTypeGIN
		}
	}

	// 16-byte special: could be B-tree, Hash, or GiST
	if specialSize == 16 {
		// Hash: hasho_page_id at offset 14
		hashID := le.Uint16(special[14:16])
		if hashID == HashPageID {
			return PageTypeHash
		}
		// GiST: gist_page_id at offset 14
		gistID := le.Uint16(special[14:16])
		if gistID == GistPageID {
			return PageTypeGiST
		}
		// B-tree: btpo_flags at offset 12, valid bits 0-8
		btFlags := le.Uint16(special[12:14])
		if btFlags&0xFE00 == 0 {
			return PageTypeBTree
		}
	}

	return PageTypeUnknown
}

func (p *Page) SpecialSize() int {
	pageSize := int(p.Header.PageSz())
	if pageSize == 0 {
		pageSize = PageSize
	}
	return pageSize - int(p.Header.Special)
}

func (p *Page) SpecialData() []byte {
	pageSize := int(p.Header.PageSz())
	if pageSize == 0 {
		pageSize = PageSize
	}
	if int(p.Header.Special) >= pageSize {
		return nil
	}
	return p.Data[p.Header.Special:pageSize]
}

func (p *Page) ParseHeapTupleHeader(offset uint16) HeapTupleHeader {
	d := p.Data[offset:]
	le := binary.LittleEndian
	var t HeapTupleHeader
	t.Xmin = le.Uint32(d[0:4])
	t.Xmax = le.Uint32(d[4:8])
	t.Field3 = le.Uint32(d[8:12])
	biHi := le.Uint16(d[12:14])
	biLo := le.Uint16(d[14:16])
	t.CtidBlock = uint32(biHi)<<16 | uint32(biLo)
	t.CtidOffset = le.Uint16(d[16:18])
	t.Infomask2 = le.Uint16(d[18:20])
	t.Infomask = le.Uint16(d[20:22])
	t.Hoff = d[22]
	return t
}

func (p *Page) ParseIndexTupleHeader(offset uint16) IndexTupleHeader {
	d := p.Data[offset:]
	le := binary.LittleEndian
	var it IndexTupleHeader
	biHi := le.Uint16(d[0:2])
	biLo := le.Uint16(d[2:4])
	it.TidBlock = uint32(biHi)<<16 | uint32(biLo)
	it.TidOffset = le.Uint16(d[4:6])
	it.Info = le.Uint16(d[6:8])
	return it
}

func ReadPage(filename string, pageNum int) (*Page, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	offset := int64(pageNum) * PageSize
	if _, err = f.Seek(offset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek to page %d: %w", pageNum, err)
	}

	var data [PageSize]byte
	n, err := io.ReadFull(f, data[:])
	if err != nil {
		return nil, fmt.Errorf("read page %d (got %d bytes): %w", pageNum, n, err)
	}

	p := ParsePage(data)
	p.PageNum = pageNum
	return p, nil
}

func FilePageCount(filename string) (int, error) {
	fi, err := os.Stat(filename)
	if err != nil {
		return 0, err
	}
	return int(fi.Size() / PageSize), nil
}

func FlagsString(flags uint16) string {
	var parts []string
	if flags&PDHasFreeLines != 0 {
		parts = append(parts, "HAS_FREE_LINES")
	}
	if flags&PDPageFull != 0 {
		parts = append(parts, "PAGE_FULL")
	}
	if flags&PDAllVisible != 0 {
		parts = append(parts, "ALL_VISIBLE")
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, " | ")
}

func blockStr(blk uint32) string {
	if blk == InvalidBlock {
		return "NONE"
	}
	return fmt.Sprintf("%d", blk)
}
