package main_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rah-0/rod"
	"github.com/rah-0/rod/lib/launcher"
	"github.com/rah-0/rod/lib/proto"
)

const exampleScriptURL = "https://raw.githubusercontent.com/rah-0/ember/master/Ember.js"

func TestExamples(t *testing.T) {
	source, err := os.ReadFile("Ember.js")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.FileServer(http.Dir("examples")))
	t.Cleanup(server.Close)

	bin := os.Getenv("EMBER_BROWSER_BIN")
	if bin == "" {
		var found bool
		bin, found = launcher.LookPath()
		if !found {
			t.Fatal("install Chrome or Chromium, or set EMBER_BROWSER_BIN to its executable")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	browser := rod.New().Context(ctx)
	if err := browser.Launch(launcher.New().Bin(bin).Headless(true)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := browser.Close(); err != nil {
			t.Error(err)
		}
	})

	examples := []string{
		"basic", "success", "info", "warning", "error", "question", "position",
		"stacked", "multiline", "rtl", "target", "themes", "styled", "icon", "animation",
		"undo", "confirm", "form", "timer", "dismiss", "ignore", "replace", "abort", "defaults",
	}
	for _, name := range examples {
		t.Run(name, func(t *testing.T) {
			page := newExamplesPage(t, browser, server.URL, source)
			if count := evaluate[int](t, page, `() => document.querySelectorAll("[data-example]").length`); count != len(examples) {
				t.Fatalf("gallery has %d examples; smoke test covers %d", count, len(examples))
			}
			if name == "multiline" {
				setViewport(t, page, 1280, 900)
			}
			clickElement(t, page, `[data-example="`+name+`"]`)
			waitJS(t, page, `() => document.querySelector(".ember") !== null`)
			assertJS(t, page, "example notification should be visible", `() => {
				const bounds = document.querySelector(".ember").getBoundingClientRect();
				return bounds.width > 0 && bounds.height > 0;
			}`)
			if name == "target" {
				assertJS(t, page, "target example should render inside its preview area", `() => document.querySelector("#target-area .ember") !== null`)
			}
			if name == "multiline" {
				assertMultilineExample(t, page)
			}
			if name == "timer" {
				for _, label := range []string{"Pause", "Resume", "Restart", "Close"} {
					clickExampleAction(t, page, label)
				}
				waitJS(t, page, `() => document.querySelector(".ember") === null`)
			}
			if name == "abort" {
				clickExampleAction(t, page, "Cancel operation")
				waitJS(t, page, `() => document.querySelector(".ember") === null`)
			}
			// Overlay examples cover the gallery controls, so clear through the API here.
			// The gallery's own Clear and Reset controls are exercised separately below.
			runJS(t, page, `async () => { await Ember.Clear(); }`)
			assertJS(t, page, "clearing an example should remove its notifications and overlay", `() => document.querySelector(".ember, .ember-overlay") === null`)
		})
	}

	t.Run("custom input and action", func(t *testing.T) {
		page := newExamplesPage(t, browser, server.URL, source)
		clickElement(t, page, `[data-example="form"]`)
		field, err := page.Element("#example-name")
		if err != nil {
			t.Fatal(err)
		}
		if err := field.Input("Morgan"); err != nil {
			t.Fatal(err)
		}
		clickExampleAction(t, page, "Greet")
		waitJS(t, page, `() => Array.from(document.querySelectorAll(".ember")).some(toast => toast.textContent.includes("Morgan"))`)
		waitJS(t, page, `() => document.querySelector("#example-name") === null`)
	})

	t.Run("clear and reset controls", func(t *testing.T) {
		page := newExamplesPage(t, browser, server.URL, source)
		clickElement(t, page, `[data-example="basic"]`)
		waitJS(t, page, `() => document.querySelector(".ember") !== null`)
		clickElement(t, page, "#clear-toasts")
		waitJS(t, page, `() => document.querySelector(".ember") === null`)
		assertJS(t, page, "clear should update the gallery status", `() => document.querySelector("#status").textContent.toLowerCase().includes("clear")`)
		clickElement(t, page, `[data-example="defaults"]`)
		waitJS(t, page, `() => document.querySelector(".ember") !== null`)
		clickElement(t, page, "#reset-defaults")
		waitJS(t, page, `() => document.querySelector(".ember") === null`)
		assertJS(t, page, "reset should update the gallery status", `() => {
			const status = document.querySelector("#status").textContent.toLowerCase();
			return status.includes("reset") || status.includes("restor");
		}`)
		clickElement(t, page, `[data-example="basic"]`)
		waitJS(t, page, `() => document.querySelector(".ember") !== null`)
	})

	t.Run("mobile layout", func(t *testing.T) {
		page := newExamplesPage(t, browser, server.URL, source)
		setViewport(t, page, 390, 844)
		assertJS(t, page, "gallery should fit a narrow viewport without horizontal scrolling", `() => document.documentElement.scrollWidth <= window.innerWidth`)
		clickElement(t, page, `[data-example="success"]`)
		waitJS(t, page, `() => document.querySelector(".ember") !== null`)
		runJS(t, page, `async () => { await Ember.Clear(); }`)
		clickElement(t, page, `[data-example="multiline"]`)
		assertMultilineExample(t, page)
	})

	t.Run("script load failure", func(t *testing.T) {
		page := newExamplesPageWithStatus(t, browser, server.URL, source, http.StatusNotFound)
		assertJS(t, page, "failed loading should keep every demo control disabled", `() => {
			const controls = document.querySelectorAll("[data-example], #clear-toasts, #reset-defaults, #position, #animation");
			return controls.length > 0 && Array.from(controls).every(control => control.disabled);
		}`)
		assertJS(t, page, "failed loading should explain the problem and leave Ember unavailable", `() => {
			const status = document.querySelector("#status").textContent.toLowerCase();
			return status.includes("load") && status.includes("ember") && typeof window.Ember === "undefined";
		}`)
	})

	t.Run("downloaded HTML", func(t *testing.T) {
		path, err := filepath.Abs("examples/index.html")
		if err != nil {
			t.Fatal(err)
		}
		address := (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
		page := newExamplesPage(t, browser, address, source)
		clickElement(t, page, `[data-example="success"]`)
		waitJS(t, page, `() => document.querySelector(".ember") !== null`)
	})
}

func assertMultilineExample(t *testing.T, page *rod.Page) {
	t.Helper()
	waitJS(t, page, `() => {
		const toast = document.querySelector(".ember");
		return toast !== null && toast.getAnimations().length === 0;
	}`)
	assertJS(t, page, "large toast should show a title above a multiline body and fit the viewport", `() => {
		const toast = document.querySelector(".ember");
		const title = toast.querySelector(".ember-title");
		const message = toast.querySelector(".ember-message");
		const bounds = toast.getBoundingClientRect();
		const titleBounds = title.getBoundingClientRect();
		const messageBounds = message.getBoundingClientRect();
		const lineHeight = Number.parseFloat(getComputedStyle(message).lineHeight);
		return title.textContent.length > 0 && titleBounds.bottom <= messageBounds.top &&
			messageBounds.height >= lineHeight * 4 && bounds.width > 300 && bounds.height > 180 &&
			bounds.left >= 0 && bounds.top >= 0 &&
			bounds.right <= window.innerWidth && bounds.bottom <= window.innerHeight &&
			toast.scrollWidth <= toast.clientWidth && toast.scrollHeight <= toast.clientHeight;
	}`)
}

func clickExampleAction(t *testing.T, page *rod.Page, label string) {
	t.Helper()
	button, err := page.ElementR(".ember button", "^"+regexp.QuoteMeta(label)+"$")
	if err != nil {
		t.Fatal(err)
	}
	if err := button.Click(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
}

func newExamplesPage(t *testing.T, browser *rod.Browser, url string, source []byte) *rod.Page {
	t.Helper()
	return newExamplesPageWithStatus(t, browser, url, source, http.StatusOK)
}

func newExamplesPageWithStatus(t *testing.T, browser *rod.Browser, url string, source []byte, statusCode int) *rod.Page {
	t.Helper()
	page := newTestPage(t, browser)
	router := page.Context(context.WithoutCancel(page.GetContext())).HijackRequests()
	var served atomic.Bool
	// Match GitHub's raw response headers while testing the current local source before publication.
	if err := router.Add(exampleScriptURL, "", func(request *rod.Hijack) {
		request.Response.Payload().ResponseCode = statusCode
		request.Response.SetHeader("Content-Type", "text/plain; charset=utf-8")
		request.Response.SetHeader("X-Content-Type-Options", "nosniff")
		request.Response.SetHeader("Access-Control-Allow-Origin", "*")
		request.Response.SetBody(source)
		served.Store(true)
	}); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- router.Run() }()
	t.Cleanup(func() {
		if err := router.Stop(); err != nil {
			t.Error(err)
		}
		if err := <-done; err != nil {
			t.Error(err)
		}
	})
	stop := collectBrowserDiagnostics(t, page)
	t.Cleanup(func() {
		for _, diagnostic := range stop() {
			if statusCode == http.StatusNotFound && diagnostic.Kind == "log error" {
				var entry proto.LogLogEntry
				if json.Unmarshal([]byte(diagnostic.Detail), &entry) == nil &&
					entry.Source == proto.LogLogEntrySourceNetwork && entry.URL == exampleScriptURL &&
					strings.Contains(entry.Text, "404") {
					continue
				}
			}
			t.Errorf("browser %s: %s", diagnostic.Kind, diagnostic.Detail)
		}
	})
	if err := page.Navigate(url); err != nil {
		t.Fatal(err)
	}
	if err := page.WaitLoad(); err != nil {
		t.Fatal(err)
	}
	if statusCode == http.StatusOK {
		waitJS(t, page, `() => document.querySelector("#status").dataset.state === "ready"`)
	} else {
		waitJS(t, page, `() => document.querySelector("#status").dataset.state === "error"`)
	}
	if !served.Load() {
		t.Fatal("gallery did not request the published Ember script URL")
	}
	return page
}
