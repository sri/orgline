package frontend

import (
	"embed"
)

//go:embed static/index.html
var staticFS embed.FS

func IndexHTML() ([]byte, error) {
	return staticFS.ReadFile("static/index.html")
}
