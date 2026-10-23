package repoharbor

import "embed"

// Migrations contains the goose SQL migrations bundled into the binary.
//go:embed migrations/*.sql
var Migrations embed.FS

// WebFS contains the built React frontend. The placeholder is replaced by
// `pnpm build` before production builds.
//go:embed all:web/dist
var WebFS embed.FS
