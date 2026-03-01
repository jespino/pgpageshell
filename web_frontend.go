package main

import "embed"

//go:generate sh -c "cd web && pnpm install && pnpm run build"

//go:embed web/dist/*
var webDist embed.FS
