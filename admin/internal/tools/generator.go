package main

import (
	"bytes"
	"fmt"
	"go/format"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gabriel-vasile/mimetype"
)

// Define vars for build template
var conv = map[string]interface{}{
	"byteArray": byteArray,
	"mime":      determineMIMEType,
}
var tmpl = template.Must(template.New("").Funcs(conv).Parse(`package internal

// Code generated by go generate; DO NOT EDIT.

func init() {
    {{- range $name, $content := . }}
        files["{{ $name }}"] = &memfile{
			path: "{{ $name }}",
			mime: "{{ mime $content }}",
			data: {{ byteArray $content }},
		}
    {{- end }}
}`),
)

func init() {
	mimetype.Extend(pijectorCSSDetector, "text/css; charset=utf-8", ".css")
	mimetype.Extend(pijectorJSDetector, "application/javascript; charset=utf-8", ".js")
}

func pijectorCSSDetector(raw []byte, limit uint32) bool {
	return bytes.HasPrefix(raw, []byte("/** CSS **/"))
}

func pijectorJSDetector(raw []byte, limit uint32) bool {
	return bytes.HasPrefix(raw, []byte("/** JS **/"))
}

func determineMIMEType(s []byte) string {
	return mimetype.Detect(s).String()
}

func byteArray(s []byte) string {
	return fmt.Sprintf("%#v", s)
}

func main() {
	args := os.Args[1:]
	if len(args) != 2 {
		log.Fatal("usage: generator [static dir] [output filename]")
	}

	staticFileRoot := args[0]
	outputFile := args[1]

	if _, err := os.Stat(staticFileRoot); os.IsNotExist(err) {
		log.Fatalf("configs directory %q does not exist", staticFileRoot)
	}

	configs := make(map[string][]byte)

	if err := filepath.Walk(staticFileRoot, func(path string, info os.FileInfo, err error) error {
		relativePath := filepath.ToSlash(strings.TrimPrefix(path, staticFileRoot))

		if info.IsDir() {
			return nil
		} else {
			log.Printf("packing %q", path)

			b, err := ioutil.ReadFile(path)
			if err != nil {
				log.Printf("Failed reading %q: %v", path, err)
				return err
			}
			configs[relativePath] = b
		}
		return nil
	}); err != nil {
		log.Fatal("Error walking through embed directory:", err)
	}
	f, err := os.Create(outputFile)
	if err != nil {
		log.Fatal("Error creating blob file:", err)
	}
	defer f.Close()

	var builder bytes.Buffer
	if err = tmpl.Execute(&builder, configs); err != nil {
		log.Fatalf("Failed executing template: %v", err)
	}

	data, err := format.Source(builder.Bytes())
	if err != nil {
		log.Fatalf("Failed formatting generated code: %v", err)
	}

	if err = ioutil.WriteFile(outputFile, data, os.ModePerm); err != nil {
		log.Fatalf("Failed writing output file: %v", err)
	}
}
