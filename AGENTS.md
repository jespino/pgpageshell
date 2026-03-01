# AGENTS.md — pgpageshell

## Project overview

pgpageshell is a CLI tool (and desktop GUI) for inspecting PostgreSQL data files at the page level. It reads raw 8 KB pages from heap and index files and decodes their binary structure: headers, line pointers, tuple data, MVCC metadata, and index-specific special regions.

## Languages and tooling

- **Go 1.22+** — all backend code, single module at repo root (`go.mod`)
- **TypeScript / React 19** — GUI frontend in `frontend/`, built with Vite, managed with pnpm
- **Wails v2** — desktop app framework, Go functions bound directly to the frontend
- No ORM, no database connection — this tool reads raw binary files directly

## Repository layout

```
.
├── main.go              # Entry point: Wails GUI (default) or --shell for CLI
├── app.go               # Wails-bound App struct with GetFiles, GetFileInfo, GetPageDetail
├── api_types.go         # Shared types and page detail builders
├── page.go              # Page parsing, type detection, struct definitions, constants
├── commands.go          # Shell commands: cat, format, info, data (hex dump, ASCII art, tuple decoding)
├── special.go           # Index-specific special region decoders (btree, hash, gist, gin, spgist, brin)
├── wails.json           # Wails project config
├── frontend/            # Vite React+TypeScript app
│   ├── src/
│   │   ├── main.tsx
│   │   ├── types.ts     # TypeScript types for UI state
│   │   ├── colors.ts    # Color constants for SVG regions and tuples
│   │   └── components/  # App, Sidebar, PageSVG, Tooltip, DetailPanel
│   ├── wailsjs/         # Wails-generated Go bindings for frontend
│   ├── vite.config.ts
│   └── dist/            # Built output, embedded into Go binary (gitignored)
├── go.mod
├── go.sum
└── .devcontainer/       # Go 1.22 + Node 20 via devcontainer features
```

## Build and run

```bash
# Build with make
make

# Or manually
cd frontend && pnpm install && pnpm run build && cd ..
go build -o pgpageshell .

# Run GUI (default)
./pgpageshell <postgres-data-file> [file2 ...]

# Run interactive CLI shell
./pgpageshell --shell <postgres-data-file>
```

## Architecture notes

- **Wails bindings** replace the old HTTP API. Go methods on the `App` struct (`GetFiles`, `GetFileInfo`, `GetPageDetail`) are called directly from the frontend via generated JS bindings in `frontend/wailsjs/`.
- **Page type detection** (`page.go:detectPageType`) uses the special region size and magic bytes to identify btree, hash, gist, gin, spgist, brin, or heap pages.
- **All binary parsing is little-endian** (`encoding/binary.LittleEndian`), matching x86 PostgreSQL.
- **`frontend/dist/` is gitignored** and built before `go build`. The `//go:embed` directive in `main.go` requires the files to exist at build time.

## Conventions

- Go code uses standard library style — no frameworks, no DI, flat package structure.
- Constants for PostgreSQL page internals (flag bits, struct sizes, magic numbers) are defined in `page.go` with names matching the PostgreSQL C source.
- Go types in `api_types.go` are used by Wails bindings and mirrored in `frontend/wailsjs/go/models.ts` — keep them in sync when adding fields.
- The frontend uses a dark theme with monospace fonts. Color palettes for SVG regions are in `frontend/src/colors.ts`.
