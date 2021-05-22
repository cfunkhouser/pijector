//go:generate go run ./tools/generator.go ../../static staticfiles.go
package internal

import (
	"bytes"
	"io"
)

var (
	DefaultPage = `<!DOCTYPE html>
<html><head><style>
body,html {
	background-color: #333;
	color: #eee;
	font-family: serif;
}
div#main-content {
	margin-top: 25%;
	font-size: xx-large;
}
div#main-content p {
	text-align: center;
}
p.subtle {
	font-size: small;
	color: #666;
}
</style><title>Pijector Kiosk</title></head>
<body>
<div id="main-content">
<p>This is Pijector.</p>
<p class="subtle">You should probably set something more interesting to look at.</p>
</div>
</body></html>
<!-- Hello, Min! -->
`

	files map[string]*memfile = map[string]*memfile{
		"/index.html": {
			path: "/index.html",
			mime: "text/html; charset=utf-8",
			data: []byte(DefaultPage),
		},
	}
)

type memfile struct {
	path string
	mime string
	data []byte
}

// Path of the file in the virtual in-memory "filesystem."
func (m *memfile) Path() string {
	return m.path
}

// MIMEType of the file.
func (m *memfile) MIMEType() string {
	return m.mime
}

// Reader which will produce the contents of the file.
func (m *memfile) Reader() io.Reader {
	return bytes.NewReader(m.data)
}

// Get a static file from memory.
func Get(path string) *memfile {
	return files[path]
}

// List static files available in memory.
func List() (ls []string) {
	for p := range files {
		ls = append(ls, p)
	}
	return
}
