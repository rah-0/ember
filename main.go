package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

var ErrMinify = errors.New("cannot minify JavaScript")

func main() {
	if err := minify("Ember.js", "Ember.js.min"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Created Ember.js.min")
}

func minify(input, output string) error {
	source, err := os.ReadFile(input)
	if err != nil {
		return fmt.Errorf("read JavaScript: %w", err)
	}
	result := api.Transform(string(source), api.TransformOptions{
		Sourcefile:        input,
		Loader:            api.LoaderJS,
		Target:            api.ESNext,
		Platform:          api.PlatformBrowser,
		Charset:           api.CharsetUTF8,
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		LegalComments:     api.LegalCommentsNone,
	})
	if len(result.Errors) != 0 {
		messages := api.FormatMessages(result.Errors, api.FormatMessagesOptions{Kind: api.ErrorMessage})
		return fmt.Errorf("%w:\n%s", ErrMinify, strings.Join(messages, ""))
	}
	if err := os.WriteFile(output, result.Code, 0o644); err != nil {
		return fmt.Errorf("write minified JavaScript: %w", err)
	}
	return nil
}
