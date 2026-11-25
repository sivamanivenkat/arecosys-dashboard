package main

import "embed"

//go:embed index.html styles.css design.js dashboard.js logo.png
var Assets embed.FS
