// Package scripts embeds the analysis JS and plans, so a deployment can run
// them without knowing where they live on disk.
package scripts

import "embed"

//go:embed plan.json common collections
var FS embed.FS
