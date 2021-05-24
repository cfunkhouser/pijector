// Package admin serves an admin UI for a pijector server.
package admin

import (
	"fmt"
	"io"
	"net/http"

	"github.com/cfunkhouser/pijector/admin/internal"
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

// Handler serves static files which have been built into the pijector
// server binary.
func Handler(w http.ResponseWriter, r *http.Request) {
	path := alias.ForPath(r.URL.Path)
	file := internal.Get(path)
	if file == nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "%v not found, sorry.", path)
		return
	}
	w.Header().Set("Content-Type", file.MIMEType())
	if _, err := io.Copy(w, file.Reader()); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
