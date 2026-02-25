# Tutorial: Exploring PostgreSQL Pages on Disk

This tutorial walks you through how PostgreSQL stores data on disk at the page
level. You will set up a PostgreSQL instance with the Pagila sample database,
create different index types, and use `pgpageshell` to inspect the raw pages.

By the end you will understand:

- How heap (table) pages are structured: header, line pointers, tuples, free space.
- How each tuple carries MVCC metadata (xmin, xmax, infomask).
- How B-tree, Hash, GiST, GIN, and BRIN index pages differ from heap pages.
- How to read raw page data and connect it back to SQL-level concepts.

## 1. Setting Up the Environment

### 1.1 Start PostgreSQL with Docker

We use a Docker volume so we can access the raw data files from outside the
container:

```bash
mkdir -p pgdata

docker run -d --name pgpagila \
  -e POSTGRES_PASSWORD=secret \
  -v $(pwd)/pgdata:/var/lib/postgresql/data \
  postgres:16

# Wait for it to be ready
docker exec pgpagila pg_isready -U postgres
```

### 1.2 Load the Pagila Database

Download the Pagila schema and data, then load them:

```bash
curl -sL -o /tmp/pagila-schema.sql \
  https://raw.githubusercontent.com/devrimgunduz/pagila/master/pagila-schema.sql
curl -sL -o /tmp/pagila-data.sql \
  https://raw.githubusercontent.com/devrimgunduz/pagila/master/pagila-data.sql

docker exec pgpagila psql -U postgres -c "CREATE DATABASE pagila;"
cat /tmp/pagila-schema.sql | docker exec -i pgpagila psql -U postgres -d pagila
cat /tmp/pagila-data.sql   | docker exec -i pgpagila psql -U postgres -d pagila
```

### 1.3 Create Additional Index Types

Pagila ships with B-tree indexes and one GiST index. Let's add Hash, GIN, and
BRIN indexes so we can compare all the major types:

```sql
-- Connect to pagila
-- docker exec -it pgpagila psql -U postgres -d pagila

-- Hash index on customer email (equality lookups only)
CREATE INDEX idx_hash_customer_email ON customer USING hash (email);

-- GIN index on film full-text search column
CREATE INDEX idx_gin_film_fulltext ON film USING gin (fulltext);

-- BRIN index on rental_id (works well on naturally ordered data)
CREATE INDEX idx_brin_rental_id ON rental USING brin (rental_id);

-- Flush everything to disk
CHECKPOINT;
```

### 1.4 Locate the Data Files

Every table and index is stored as one or more files under the data directory.
Use `pg_relation_filepath` to find them:

```sql
SELECT c.relname, am.amname, pg_relation_filepath(c.oid) AS filepath
FROM pg_class c
LEFT JOIN pg_am am ON c.relam = am.oid
WHERE c.relnamespace = 'public'::regnamespace
  AND c.relname IN (
    'actor',                    -- heap table
    'actor_pkey',               -- btree index
    'idx_hash_customer_email',  -- hash index
    'film_fulltext_idx',        -- gist index
    'idx_gin_film_fulltext',    -- gin index
    'idx_brin_rental_id'        -- brin index
  )
ORDER BY am.amname;
```

```
         relname         | amname |     filepath
-------------------------+--------+------------------
 idx_brin_rental_id      | brin   | base/16384/17985
 actor_pkey              | btree  | base/16384/17726
 idx_gin_film_fulltext   | gin    | base/16384/17984
 film_fulltext_idx       | gist   | base/16384/17754
 idx_hash_customer_email | hash   | base/16384/17983
 actor                   | heap   | base/16384/17543
```

The actual OIDs will differ on your system. The files live under
`pgdata/base/<database_oid>/`.

### 1.5 Build pgpageshell

```bash
cd pgpageshell
go build -o pgpageshell .
```

Now you can point it at any of those files. For example:

```bash
./pgpageshell pgdata/base/16384/17543
```

> **Note**: If PostgreSQL is running, the files are still readable. PostgreSQL
> uses shared buffers and does not hold exclusive locks on the data files. You
> can safely read them while the server is up.

---

## 2. The PostgreSQL Page: General Structure

Every data file in PostgreSQL is divided into **pages** (also called blocks),
each exactly **8192 bytes** (8 KB) by default. Whether it's a heap table, a
B-tree index, or a GIN index, every page shares the same basic skeleton:

```
+--------------------------------------------------------------+
| Page Header (24 bytes)                                       |
+--------------------------------------------------------------+
| Line Pointers (ItemId array, 4 bytes each)                   |
+--------------------------------------------------------------+
| Free Space                                                   |
+--------------------------------------------------------------+
| Tuples (grow downward from the end of the page)              |
+--------------------------------------------------------------+
| Special Space (index-specific, at the very end)              |
+--------------------------------------------------------------+
```

Key points:

- **Line pointers** grow forward (low to high addresses).
- **Tuples** grow backward (high to low addresses).
- **Free space** is the gap between them.
- **Special space** is fixed at the end of the page and is used by index
  access methods to store per-page metadata. Heap pages have no special space.

The page header tracks where each region starts and ends through the fields
`pd_lower` (end of line pointers), `pd_upper` (start of tuples), and
`pd_special` (start of special space).

---

## 3. Heap Pages (Tables)

Let's inspect the `actor` table. It has 200 rows across 2 pages.

### 3.1 Page Layout

```
pgpageshell(page 0)> format

  Page Layout (page size: 8192, type: heap)
  Offset 0x0000 - 0x1fff

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

This page holds 142 actor rows. The page is nearly full (only 32 bytes free).
Notice there is no Special Space — heap pages use the full 8192 bytes for
header + line pointers + tuples.

### 3.2 Page Header

```
pgpageshell(page 0)> info

=== Page Header (detected type: heap) ===
  pd_lsn             : 0/01F08DB0
  pd_checksum        : 0x0000 (0)
  pd_flags           : 0x0000 [none]
  pd_lower           : 592 (0x0250)
  pd_upper           : 624 (0x0270)
  pd_special         : 8192 (0x2000)
  pd_pagesize_version: 0x2004 (size: 8192, version: 4)
  pd_prune_xid       : 0
```

Field by field:

| Field | Value | Meaning |
|-------|-------|---------|
| `pd_lsn` | `0/01F08DB0` | WAL position of the last change to this page. Used for crash recovery. |
| `pd_checksum` | `0x0000` | Page checksum (disabled by default). |
| `pd_flags` | `0x0000` | No flags set. Could be `HAS_FREE_LINES`, `PAGE_FULL`, or `ALL_VISIBLE`. |
| `pd_lower` | `592` | Byte offset where line pointers end. (24 header + 142×4 = 592) |
| `pd_upper` | `624` | Byte offset where tuple data starts. |
| `pd_special` | `8192` | Equal to page size → no special space (heap page). |
| `pd_pagesize_version` | `0x2004` | Page size 8192 (0x2000) + layout version 4. |
| `pd_prune_xid` | `0` | Oldest xmax on the page that might be reclaimable. |

### 3.3 Line Pointers and Tuples

Each line pointer is a 4-byte entry that stores the offset and length of a
tuple within the page, plus a 2-bit status flag:

| Flag | Meaning |
|------|---------|
| `NORMAL` | Points to a live tuple. |
| `REDIRECT` | HOT chain redirect — points to another line pointer, not a tuple. |
| `DEAD` | Tuple has been removed but the slot hasn't been reclaimed yet. |
| `UNUSED` | Slot is free and can be reused. |

Let's look at the actual tuples:

```
pgpageshell(page 0)> data

=== Heap Tuples ===

--- Tuple 1 (offset 8136, length 56) ---
  Tuple Header (HeapTupleHeaderData):
    t_xmin       : 969
    t_xmax       : 978
    t_cid        : 0
    t_ctid       : (0, 1)
    t_infomask2  : 0x0004 (natts: 4)
    t_infomask   : 0x0192 [HAS_VARWIDTH | XMAX_KEYSHR_LOCK | XMAX_LOCK_ONLY | XMIN_COMMITTED]
    t_hoff       : 24
    User data (32 bytes at offset 8160):
      00001fe0: 01 00 00 00 13 50 45 4e  45 4c 4f 50 45 11 47 55  |.....PENELOPE.GU|
      00001ff0: 49 4e 45 53 53 00 00 00  40 9c 5d 02 0a 7b 02 00  |INESS...@.]..{..|
    Printable strings:
      "PENELOPE"
      "GUINESS"

--- Tuple 2 (offset 8080, length 56) ---
  Tuple Header (HeapTupleHeaderData):
    t_xmin       : 969
    t_xmax       : 978
    t_cid        : 0
    t_ctid       : (0, 2)
    t_infomask2  : 0x0004 (natts: 4)
    t_infomask   : 0x0192 [HAS_VARWIDTH | XMAX_KEYSHR_LOCK | XMAX_LOCK_ONLY | XMIN_COMMITTED]
    t_hoff       : 24
    User data (32 bytes at offset 8104):
      00001fa8: 02 00 00 00 0b 4e 49 43  4b 13 57 41 48 4c 42 45  |.....NICK.WAHLBE|
      00001fb8: 52 47 00 00 00 00 00 00  40 9c 5d 02 0a 7b 02 00  |RG......@.]..{..|
    Printable strings:
      "NICK"
      "WAHLBERG"
```

This is actor #1 (PENELOPE GUINESS) and actor #2 (NICK WAHLBERG). Let's break
down what we see:

**Tuple header (23 bytes, padded to `t_hoff` = 24):**

| Field | Value | Meaning |
|-------|-------|---------|
| `t_xmin` | `969` | Transaction ID that inserted this row. |
| `t_xmax` | `978` | Transaction that last locked/modified this row. |
| `t_cid` | `0` | Command ID within the transaction. |
| `t_ctid` | `(0, 1)` | Physical location: page 0, line pointer 1. Points to itself for live tuples. |
| `t_infomask2` | `natts: 4` | The `actor` table has 4 columns (actor_id, first_name, last_name, last_update). |
| `t_infomask` | see flags | MVCC visibility flags. `XMIN_COMMITTED` means the inserting transaction committed. |
| `t_hoff` | `24` | User data starts 24 bytes after the tuple start. |

**User data:**

The raw bytes after the header contain the column values. For the actor table:
- `01 00 00 00` → actor_id = 1 (int4, little-endian)
- `13 50 45 4e 45 4c 4f 50 45` → varlena header (0x13 = 19 bytes total) + "PENELOPE"
- `11 47 55 49 4e 45 53 53` → varlena header (0x11 = 17 bytes total) + "GUINESS"
- The remaining bytes are the `last_update` timestamp.

Notice how tuples grow **downward**: tuple 1 is at offset 8136 (near the end
of the page), tuple 2 is at 8080, and so on. The line pointers at the top of
the page point into these locations.

### 3.4 MVCC in Action

The `t_infomask` flags encode the MVCC state of each tuple. Common flags you
will see:

| Flag | Meaning |
|------|---------|
| `XMIN_COMMITTED` | The inserting transaction has committed — this tuple is visible. |
| `XMIN_INVALID` | The inserting transaction aborted — this tuple is dead. |
| `XMIN_FROZEN` | The xmin has been frozen by VACUUM — visible to all transactions. |
| `XMAX_INVALID` | No valid deleting/locking transaction — tuple is live. |
| `XMAX_COMMITTED` | The deleting transaction committed — tuple is dead. |
| `HOT_UPDATED` | This tuple has been updated via a Heap-Only Tuple (HOT) update. |
| `HEAP_ONLY` | This tuple is the result of a HOT update (not indexed). |

When you UPDATE a row, PostgreSQL creates a new tuple version. The old tuple's
`t_ctid` points to the new version, and the `HOT_UPDATED` flag is set. After
VACUUM, old versions are removed and their line pointers become `REDIRECT` or
`UNUSED`.

---

## 4. B-tree Index Pages

B-tree is the default index type. Let's look at `actor_pkey` (primary key on
`actor_id`).

### 4.1 Meta Page (Page 0)

Every B-tree index starts with a meta page at page 0:

```
pgpageshell(page 0)> info

=== Special Region ===
  Size: 16 bytes at offset 8176

  B-tree Page Opaque Data (BTPageOpaqueData):
    btpo_prev    : 0
    btpo_next    : 0
    btpo_level   : 0 (leaf)
    btpo_flags   : 0x0008 [BTP_META]
    btpo_cycleid : 0

  B-tree Meta Page Data (BTMetaPageData):
    btm_magic          : 0x053162 (valid)
    btm_version        : 4
    btm_root           : 1
    btm_level          : 0
    btm_fastroot       : 1
    btm_fastlevel      : 0
```

The meta page tells us:
- `btm_root = 1`: The root page is page 1.
- `btm_level = 0`: The tree has only one level (root is also a leaf). With
  only 200 actors, everything fits in a single leaf page.

### 4.2 Leaf Page (Page 1)

```
pgpageshell(page 1)> format

  Page Layout (page size: 8192, type: btree)
  Offset 0x0000 - 0x1fff

+--------------------------------------------------------------+
| Page Header (PageHeaderData)   [    0 -    23]    24 bytes   |
+--------------------------------------------------------------+
| Line Pointers (200 items)      [   24 -   823]   800 bytes   |
+--------------------------------------------------------------+
| Free Space                     [  824 -  4975]  4152 bytes   |
+--------------------------------------------------------------+
| Index Tuples (btree)           [ 4976 -  8175]  3200 bytes   |
+--------------------------------------------------------------+
| Special Space (btree)          [ 8176 -  8191]    16 bytes   |
+--------------------------------------------------------------+
```

Key differences from a heap page:

1. **Special space (16 bytes)** at the end contains `BTPageOpaqueData` with
   sibling pointers and tree level.
2. **Index tuples** are much smaller than heap tuples — they only contain the
   indexed key and a pointer (TID) back to the heap.

```
pgpageshell(page 1)> info

=== Special Region ===
  B-tree Page Opaque Data (BTPageOpaqueData):
    btpo_prev    : 0
    btpo_next    : 0
    btpo_level   : 0 (leaf)
    btpo_flags   : 0x0003 [BTP_LEAF | BTP_ROOT]
    btpo_cycleid : 0
```

`BTP_LEAF | BTP_ROOT` — this single page is both the root and a leaf.

### 4.3 Index Tuples

```
pgpageshell(page 1)> data

=== Index Tuples (btree) ===

--- Item 1 (offset 8160, length 16) ---
  Index Tuple Header (IndexTupleData):
    t_tid        : (0, 1)  -> heap ctid
    t_info       : 0x0010 (size: 16)
    Key data (8 bytes):
      00001fe8: 01 00 00 00 00 00 00 00                           |........|

--- Item 2 (offset 8144, length 16) ---
  Index Tuple Header (IndexTupleData):
    t_tid        : (0, 2)  -> heap ctid
    t_info       : 0x0010 (size: 16)
    Key data (8 bytes):
      00001fd8: 02 00 00 00 00 00 00 00                           |........|
```

Each index tuple is only 16 bytes:
- **8 bytes header**: 6 bytes for `t_tid` (block + offset pointing to the heap
  tuple) + 2 bytes for `t_info` (tuple size and flags).
- **8 bytes key data**: The `actor_id` value (int4 = 4 bytes) plus padding.

`t_tid = (0, 1)` means "heap page 0, line pointer 1" — this is how the index
points back to the actual row in the table.

---

## 5. Hash Index Pages

Hash indexes are optimized for equality lookups (`=`). They cannot support
range queries or ordering.

```
pgpageshell(page 0)> info

=== Special Region ===
  Hash Page Opaque Data (HashPageOpaqueData):
    hasho_prevblkno : NONE
    hasho_nextblkno : NONE
    hasho_bucket    : 4294967295
    hasho_flag      : 0x0008 [LH_META_PAGE]
    hasho_page_id   : 0xFF80 (HASHO_PAGE_ID)

  Hash Meta Page Data (HashMetaPageData):
    hashm_magic      : 0x6440640 (valid)
    hashm_version    : 4
    hashm_ntuples    : 599.000000
    hashm_ffactor    : 307
    hashm_bsize      : 8152
    hashm_maxbucket  : 1
    hashm_highmask   : 0x00000003
    hashm_lowmask    : 0x00000001
```

The hash meta page stores:
- `hashm_ntuples`: Total number of indexed tuples (599 customer emails).
- `hashm_ffactor`: Fill factor — target number of tuples per bucket.
- `hashm_maxbucket`: Highest bucket number. Buckets are split dynamically as
  the index grows.

A bucket page looks like:

```
pgpageshell(page 2)> info

=== Special Region ===
  Hash Page Opaque Data (HashPageOpaqueData):
    hasho_prevblkno : 1
    hasho_nextblkno : NONE
    hasho_bucket    : 1
    hasho_flag      : 0x0002 [LH_BUCKET_PAGE]
    hasho_page_id   : 0xFF80 (HASHO_PAGE_ID)
```

Hash pages have four types: `LH_META_PAGE`, `LH_BUCKET_PAGE`,
`LH_OVERFLOW_PAGE`, and `LH_BITMAP_PAGE`. The `hasho_page_id` field
(`0xFF80`) is a magic number that identifies the page as belonging to a hash
index.

---

## 6. GiST Index Pages

GiST (Generalized Search Tree) supports complex data types like geometric
objects, full-text search, and ranges. Pagila uses a GiST index on the `film`
table's `fulltext` column.

```
pgpageshell(page 1)> info

=== Special Region ===
  GiST Page Opaque Data (GISTPageOpaqueData):
    nsn          : 0/00000000
    rightlink    : 2
    flags        : 0x0001 [F_LEAF]
    gist_page_id : 0xFF81 (GIST_PAGE_ID)
```

GiST-specific fields:
- `nsn` (Node Sequence Number): Used for concurrent access — tracks page
  splits.
- `rightlink`: Pointer to the right sibling page (page 2 in this case).
- `flags`: `F_LEAF` means this is a leaf page. Internal pages don't have this
  flag.
- `gist_page_id`: Magic number `0xFF81` identifying this as a GiST page.

GiST internal nodes store bounding keys that encompass all entries in their
subtree. Leaf nodes store the actual indexed values with TIDs pointing to heap
tuples, just like B-tree.

---

## 7. GIN Index Pages

GIN (Generalized Inverted Index) is designed for values that contain multiple
elements — arrays, full-text search vectors, JSONB. It maps each element to
the set of rows that contain it.

```
pgpageshell(page 0)> info

=== Special Region ===
  GIN Page Opaque Data (GinPageOpaqueData):
    rightlink    : NONE
    maxoff       : 0
    flags        : 0x0008 [GIN_META]

  GIN Meta Page Data (GinMetaPageData):
    head                : NONE
    tail                : NONE
    tailFreeSize        : 0
    nPendingPages       : 0
    nPendingHeapTuples  : 0
    nTotalPages         : 14
    nEntryPages         : 13
    nDataPages          : 0
    nEntries            : 1108
```

The GIN meta page reveals the index structure:
- `nEntries = 1108`: The number of distinct lexemes indexed from the film
  descriptions.
- `nEntryPages = 13`: Pages storing the entry tree (the B-tree of keys).
- `nDataPages = 0`: No separate posting tree pages (all posting lists fit
  inline).
- `nPendingPages = 0`: The pending list is empty (no fast-inserted entries
  waiting to be merged).

GIN has a unique two-level structure:
1. An **entry tree** (B-tree of keys/lexemes).
2. For each key, a **posting list** or **posting tree** of heap TIDs.

The `GIN_META`, `GIN_LEAF`, `GIN_DATA`, and `GIN_COMPRESSED` flags tell you
what kind of GIN page you're looking at.

---

## 8. BRIN Index Pages

BRIN (Block Range Index) is the most space-efficient index type. Instead of
indexing individual rows, it stores summary information for ranges of
consecutive heap pages.

```
pgpageshell(page 0)> info

=== Special Region ===
  BRIN Special Space (BrinSpecialSpace):
    flags     : 0x0000
    page_type : 0xF091 (BRIN_PAGETYPE_META)

  BRIN Meta Page Data (BrinMetaPageData):
    brinMagic        : 0xA8109CFA (valid)
    brinVersion      : 1
    pagesPerRange    : 128
    lastRevmapPage   : 1
```

BRIN has three page types:
- **Meta page** (page 0): Stores `pagesPerRange` (128 heap pages per range)
  and the location of the revmap.
- **Revmap pages**: A map from block range number to the BRIN tuple that
  summarizes it.
- **Regular pages**: Store the actual summary tuples (min/max values for each
  range).

With `pagesPerRange = 128`, each BRIN entry covers 128 heap pages (about 1 MB
of table data). For the `rental` table with ~16,000 rows across 192 pages,
the entire BRIN index fits in just 3 pages (24 KB) compared to a B-tree that
would need ~46 pages (376 KB).

BRIN works best when the indexed column is naturally correlated with the
physical row order — like auto-incrementing IDs or timestamps.

---

## 9. Comparing Page Structures

Here's a summary of how the special space differs across index types:

| Index Type | Special Size | Key Fields | Magic/ID |
|------------|-------------|------------|----------|
| **Heap** | 0 bytes | (none) | — |
| **B-tree** | 16 bytes | prev, next, level, flags, cycleid | `0x053162` (meta magic) |
| **Hash** | 16 bytes | prevblkno, nextblkno, bucket, flag | `0xFF80` (page ID) |
| **GiST** | 16 bytes | nsn, rightlink, flags | `0xFF81` (page ID) |
| **GIN** | 8 bytes | rightlink, maxoff, flags | — |
| **SP-GiST** | 8 bytes | flags, nRedirection, nPlaceholder | `0xFF82` (page ID) |
| **BRIN** | 8 bytes | flags, page_type | `0xF091`-`0xF093` (type) |

`pgpageshell` uses these magic numbers and sizes to auto-detect the page type
when you open a file.

---

## 10. Exercises

1. **Find a specific actor**: Use `data` on the actor heap file. Can you find
   the tuple for "JOHNNY DEPP" (actor_id = 30)? What page and line pointer is
   it on?

2. **Trace an index lookup**: Look up actor_id = 30 in the B-tree index. Find
   the index tuple, note its `t_tid`, then go to that heap page and line
   pointer to find the actual row.

3. **Observe MVCC**: Connect to the database, UPDATE an actor's name, then
   CHECKPOINT. Inspect the heap page with `data` — you should see the old
   tuple with `HOT_UPDATED` and the new tuple with `HEAP_ONLY`.

4. **Compare index sizes**: Use `pages` on each index file. Count the pages.
   Compare the BRIN index on `rental_id` (3 pages) with the B-tree rental
   primary key (46 pages). Why is BRIN so much smaller?

5. **Inspect after VACUUM**: Run `VACUUM actor;` then `CHECKPOINT;`. Re-inspect
   the actor heap page. Look for `XMIN_FROZEN` flags and `REDIRECT`/`UNUSED`
   line pointers.
