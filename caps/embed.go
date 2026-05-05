package caps

import "embed"

//go:embed bundled-caps/*
var BundledCaps embed.FS
