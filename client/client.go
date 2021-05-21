// Package client serves a human-usable admin client for a pijector server.
package client

import (
	"fmt"
	"io"
	"net/http"

	"github.com/cfunkhouser/pijector/client/internal"
)

type pathAliases map[string]string

func (a *pathAliases) ForPath(path string) string {
	if alias := (*a)[path]; alias != "" {
		return alias
	}
	return path
}

var alias = pathAliases{
	"/":      "/index.html",
	"/admin": "/admin.html",
}

// StaticHandler serves static files which have been built into the pijector
// server binary.
func StaticHandler(w http.ResponseWriter, r *http.Request) {
	staticFile := internal.Get(alias.ForPath(r.URL.Path))
	if staticFile == nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "Not found, sorry.")
		return
	}
	w.Header().Set("Content-Type", staticFile.MIMEType)
	if _, err := io.Copy(w, staticFile.Reader()); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
