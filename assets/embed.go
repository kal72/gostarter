package assets

import "embed"

//go:embed css/tailwind.css css/style.css js/*.js vendor/*
var Assets embed.FS
