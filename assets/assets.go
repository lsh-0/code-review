package assets

import "embed"

//go:embed index.html style.css review.js
var Assets embed.FS
