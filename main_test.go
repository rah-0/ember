package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestMinifyRemovesComments(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.js")
	output := filepath.Join(dir, "output.js")
	source := []byte(`/*! @license Test license */
// Ordinary comment
window.Example = { Message: "https://example.com/*literal*/" };
`)
	if err := os.WriteFile(input, source, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := minify(input, output); err != nil {
		t.Fatal(err)
	}
	result, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(result, []byte("Test license")) || bytes.Contains(result, []byte("Ordinary comment")) {
		t.Fatalf("comments remain in minified JavaScript: %s", result)
	}
	if !bytes.Contains(result, []byte("https://example.com/*literal*/")) {
		t.Fatalf("comment-like string content changed: %s", result)
	}
}

func TestMinifyPreservesOutputOnSyntaxError(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.js")
	output := filepath.Join(dir, "output.js")
	if err := os.WriteFile(input, []byte("const broken = ;"), 0o644); err != nil {
		t.Fatal(err)
	}
	previous := []byte("window.Example=1;\n")
	if err := os.WriteFile(output, previous, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := minify(input, output); !errors.Is(err, ErrMinify) {
		t.Fatalf("expected a minification error, got %v", err)
	}
	result, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(result, previous) {
		t.Fatalf("invalid input replaced existing output: %s", result)
	}
}

func TestMinifiedFileIsCurrent(t *testing.T) {
	output := filepath.Join(t.TempDir(), "Ember.js.min")
	if err := minify("Ember.js", output); err != nil {
		t.Fatal(err)
	}
	expected, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile("Ember.js.min")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, expected) {
		t.Fatal("Ember.js.min is stale; regenerate it with go run .")
	}
}
