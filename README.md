# pgpageshell

An interactive shell for inspecting PostgreSQL data files at the page level.

PostgreSQL stores all table and index data in 8 KB pages on disk. These pages
have a well-defined binary format — headers, line pointers, tuple data, MVCC
metadata, index-specific structures — but there's no built-in way to look at
them directly. `pgpageshell` lets you open any PostgreSQL data file and
navigate through its pages, examining the raw structure that underlies every
query.

<div align="center">
  <a href="https://leanpub.com/deep-dive-into-a-sql-query">
    <img src="cover.png" alt="Deep dive into a SQL query" width="300">
  </a>
</div>

This tool is part of the companion material for
[**Deep dive into a SQL query**](https://leanpub.com/deep-dive-into-a-sql-query),
a book that follows a SQL statement through every stage of PostgreSQL's
internal pipeline — from parsing to execution — and explains how data is
organized on disk along the way.

## Tutorial

See [**TUTORIAL.md**](TUTORIAL.md) for a hands-on walkthrough that uses the
Pagila sample database to explore heap pages, B-tree indexes, Hash indexes,
GiST, GIN, and BRIN — all from the perspective of raw page data.

## Building

Requires Go 1.22 or later.

```bash
go build -o pgpageshell .
```

## Usage

```bash
./pgpageshell <postgres-data-file>
```

Point it at any file from PostgreSQL's data directory. You can find the file
path for a table or index with:

```sql
SELECT pg_relation_filepath('my_table');
-- Returns something like: base/16384/17543
```

The file lives under your PostgreSQL data directory (e.g.,
`/var/lib/postgresql/data/base/16384/17543`).

On startup, `pgpageshell` loads page 0 and auto-detects the page type:

```
$ ./pgpageshell /var/lib/postgresql/data/base/16384/17543
pgpageshell - PostgreSQL Page Inspector
File: /var/lib/postgresql/data/base/16384/17543 (16384 bytes, 2 pages, detected: heap)

[page 0 loaded, type: heap]
pgpageshell(page 0)>
```

## Commands

### `page <n>`

Select a page by number (0-based). Pages are 8192 bytes each.

```
pgpageshell(page 0)> page 1
[page 1 loaded, type: btree]
```

### `cat`

Hex dump of the entire 8192-byte page, with ASCII sidebar.

```
pgpageshell(page 0)> cat
00000000: 00 00 00 00 b0 8d f0 01  00 00 00 00 50 02 70 02  |............P.p.|
00000010: 00 20 04 20 00 00 00 00  c8 9f 6e 00 90 9f 70 00  |. . ......n...p.|
...
```

### `format`

ASCII art visualization showing the page regions and their byte ranges.

```
pgpageshell(page 0)> format

  Page Layout (page size: 8192, type: heap)

+--------------------------------------------------------------+
| Page Header (PageHeaderData)   [    0 -    23]    24 bytes   |
+--------------------------------------------------------------+
| Line Pointers (142 items)      [   24 -   591]   568 bytes   |
+--------------------------------------------------------------+
| Free Space                     [  592 -   623]    32 bytes   |
+--------------------------------------------------------------+
| Heap Tuples                    [  624 -  8191]  7568 bytes   |
+--------------------------------------------------------------+

  Proportional view:
  [HHLLLLL.TTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTT]
   H=Header  L=LinePointers  .=Free  T=Tuples  S=Special
```

### `info`

Decoded page header fields and special region data. The special region is
decoded according to the detected page type.

For heap pages, the special region is empty. For index pages, you get the
full opaque data structure. Meta pages also show the meta page content.

```
pgpageshell(page 0)> info

=== Page Header (detected type: btree) ===
  pd_lsn             : 0/01C77FF8
  pd_checksum        : 0x0000 (0)
  pd_flags           : 0x0000 [none]
  pd_lower           : 72 (0x0048)
  pd_upper           : 8176 (0x1FF0)
  pd_special         : 8176 (0x1FF0)
  pd_pagesize_version: 0x2004 (size: 8192, version: 4)
  pd_prune_xid       : 0

=== Special Region ===
  B-tree Page Opaque Data (BTPageOpaqueData):
    btpo_prev    : 0
    btpo_next    : 0
    btpo_level   : 0 (leaf)
    btpo_flags   : 0x0003 [BTP_LEAF | BTP_ROOT]
    btpo_cycleid : 0
```

### `data`

Line pointer table followed by decoded tuple data. For heap pages, each tuple
shows the full `HeapTupleHeaderData` (xmin, xmax, ctid, infomask flags, null
bitmap) and a hex dump of the user data with printable strings extracted. For
index pages, each tuple shows the `IndexTupleData` header (TID pointing to the
heap, size, flags) and the key data.

```
pgpageshell(page 0)> data

=== Line Pointers (Item IDs) [page type: heap] ===
  Index  Status   Offset     Length   Raw
  1      NORMAL   8136       56       0x00701FC8
  2      NORMAL   8080       56       0x00701F90
  ...

=== Heap Tuples ===

--- Tuple 1 (offset 8136, length 56) ---
  Tuple Header (HeapTupleHeaderData):
    t_xmin       : 969
    t_xmax       : 978
    t_ctid       : (0, 1)
    t_infomask   : 0x0192 [HAS_VARWIDTH | XMIN_COMMITTED]
    t_hoff       : 24
    User data (32 bytes at offset 8160):
      00001fe0: 01 00 00 00 13 50 45 4e  45 4c 4f 50 45 ...
    Printable strings:
      "PENELOPE"
      "GUINESS"
```

### `pages`

Summary of all pages in the file: type, item count, free space, and special
space size.

```
pgpageshell(page 0)> pages
  Page   0: type=heap    items=142  free=32    special=0
  Page   1: type=heap    items=58   free=4216  special=0
```

### `help`

Show the command list.

### `quit` / `exit`

Exit the shell.

## Supported Page Types

`pgpageshell` auto-detects the page type from the special region and decodes
it accordingly:

| Type | Detection | Special Region Contents |
|------|-----------|------------------------|
| **Heap** | No special space | — |
| **B-tree** | 16-byte special, valid btpo_flags | prev/next sibling, level, flags. Meta pages: magic, root, tree level. |
| **Hash** | 16-byte special, page_id = `0xFF80` | prev/next block, bucket number, page type. Meta pages: magic, ntuples, fill factor, masks. |
| **GiST** | 16-byte special, page_id = `0xFF81` | NSN, rightlink, flags (leaf/deleted/follow-right). |
| **GIN** | 8-byte special, valid flags | Rightlink, maxoff, flags (data/leaf/meta/list/compressed). Meta pages: pending list, entry/data page counts. |
| **SP-GiST** | 8-byte special, page_id = `0xFF82` | Flags (meta/deleted/leaf/nulls), redirect and placeholder counts. |
| **BRIN** | 8-byte special, type = `0xF091`–`0xF093` | Flags, page type (meta/revmap/regular). Meta pages: magic, version, pages-per-range. |

## License

MIT
