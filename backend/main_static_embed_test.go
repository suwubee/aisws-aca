package main

import (
	"io/fs"
	"testing"
)

func TestEmbeddedStaticAssetsContainIndexHTML(t *testing.T) {
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		t.Fatalf("fs.Sub(staticFiles, \"static\") failed: %v", err)
	}
	data, err := fs.ReadFile(staticFS, "index.html")
	if err != nil {
		t.Fatalf("embedded index.html not found: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("embedded index.html is empty")
	}
}
