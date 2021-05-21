//go:generate go run ./tools/generator.go ../../static blob.go
package internal

import (
	"bytes"
	"io"
	"sync"
)

var (
	DefaultMemfiles = &memfiles{
		storage: make(map[string]*memfile),
	}

	DefaultPage = []byte(`<!DOCTYPE html>
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
`)
)

func init() {
	// Make sure at least the index page is _always_ in the content package, so
	// that if the generator fails, the server will still be reasonably sane.
	DefaultMemfiles.Add("/index.html", "text/html; charset=utf-8", DefaultPage)
}

type memfile struct {
	Name     string
	MIMEType string
	Data     []byte
}

func (m *memfile) Reader() io.Reader {
	return bytes.NewReader(m.Data)
}

type memfiles struct {
	sync.RWMutex
	storage map[string]*memfile
}

// Add a file to the memfiles
func (m *memfiles) Add(file string, mime string, content []byte) {
	m.Lock()
	defer m.Unlock()
	m.storage[file] = &memfile{
		Name:     file,
		Data:     content[:],
		MIMEType: mime,
	}
}

// Get file from memfiles.
func (m *memfiles) Get(file string) *memfile {
	m.RLock()
	defer m.RUnlock()
	return m.storage[file]
}

// Add a file content to box
func Add(file string, mime string, content []byte) {
	DefaultMemfiles.Add(file, mime, content)
}

// Get a file from box
func Get(file string) *memfile {
	return DefaultMemfiles.Get(file)
}
