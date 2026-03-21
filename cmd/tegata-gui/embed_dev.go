//go:build !production

package main

import "embed"

// assets is an empty filesystem used during development and testing.
// In production builds, this is replaced by embed_production.go which
// embeds the built frontend from frontend/dist.
var assets embed.FS
