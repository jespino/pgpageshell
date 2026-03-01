# AGENTS.md — pgpageshell

## Project overview

pgpageshell is a CLI tool (and web UI) for inspecting PostgreSQL data files at the page level. It reads raw 8 KB pages from heap and index files and decodes their binary structure: headers, line pointers, tuple data, MVCC metadata, and index-specific special regions.

## Languages and tooling

- **Go 1.22+** — all backend code, single module at repo root (`go.mod`)
- **TypeScript / React 19** — web UI in `web/`, built with Vite, managed with pnpm
- No ORM, no database connection — this tool reads raw binary files directly

## Repository layout

```
.
├── main.go              # CLI entry point, --web flag, interactive shell loop
├── page.go              # Page parsing, type detection, struct definitions, constants
├── commands.go          # Shell commands: cat, format, info, data (hex dump, ASCII art, tuple decoding)
├── special.go           # Index-specific special region decoders (btree, hash, gist, gin, spgist, brin)
├── web_server.go        # HTTP server, JSON API endpoints (/api/file, /api/page/<n>)
├── web_frontend.go      # go:embed directive for web/dist/
├── web/                 # Vite React+TypeScript app
│   ├── src/
│   │   ├── main.tsx
│   │   ├── types.ts     # TypeScript types matching Go JSON API responses
│   │   ├── colors.ts    # Color constants for SVG regions and tuples
│   │   └── components/  # App, Sidebar, PageSVG, Tooltip, DetailPanel
│   ├── vite.config.ts   # Dev proxy: /api → localhost:8080
│   └── dist/            # Built output, embedded into Go binary (committed, not gitignored)
├── go.mod
├── go.sum
└── .devcontainer/       # Go 1.22 + Node 20 via devcontainer features
```

## Build and run

```bash
# Full build: generate frontend then compile Go binary
go generate ./...
go build -o pgpageshell .

# Run interactive shell
./pgpageshell <postgres-data-file>

# Run web UI
./pgpageshell --web <postgres-data-file>
./pgpageshell --web :3000 <postgres-data-file>   # custom address
```

`go generate` runs `pnpm install && pnpm run build` inside `web/`, producing `web/dist/` which is then embedded into the Go binary via `//go:embed`. Node.js and pnpm must be available for the generate step.

## Development workflow

For frontend development, run the Go backend and Vite dev server side by side:

```bash
# Terminal 1: Go backend on :8080
go run . --web :8080 <data-file>

# Terminal 2: Vite dev server (proxies /api to :8080)
cd web && pnpm dev
```

After frontend changes, rebuild with `pnpm run build` before `go build` — the Go binary embeds `web/dist/` via `//go:embed`.

## Architecture notes

- **Page type detection** (`page.go:detectPageType`) uses the special region size and magic bytes to identify btree, hash, gist, gin, spgist, brin, or heap pages. This is heuristic-based, matching PostgreSQL's own internal layout.
- **All binary parsing is little-endian** (`encoding/binary.LittleEndian`), matching x86 PostgreSQL.
- **The web UI is a single-page app** embedded into the Go binary. The Go server handles SPA routing by falling back to `index.html` for unknown paths.
- **`web/dist/` is gitignored** and built on demand via `go generate`. The `//go:embed` directive requires the files to exist at build time, so `go generate ./...` must run before `go build`.
- **No external Go dependencies** beyond `github.com/chzyer/readline` for the interactive shell.

## Conventions

- Go code uses standard library style — no frameworks, no DI, flat package structure.
- Constants for PostgreSQL page internals (flag bits, struct sizes, magic numbers) are defined in `page.go` with names matching the PostgreSQL C source.
- JSON API types in `web_server.go` mirror the TypeScript types in `web/src/types.ts` — keep them in sync when adding fields.
- The web UI uses a dark theme with monospace fonts. Color palettes for SVG regions are in `web/src/colors.ts`.
