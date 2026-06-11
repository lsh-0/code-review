package assets

import "embed"

//go:embed index.html style.css review.js fonts
var Assets embed.FS
