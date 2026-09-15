package main_test

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rah-0/rod"
	"github.com/rah-0/rod/lib/input"
	"github.com/rah-0/rod/lib/launcher"
	"github.com/rah-0/rod/lib/proto"
)

const testHTML = `<!doctype html>
<html lang="en">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<link rel="icon" href="data:,">
	<title>Ember browser tests</title>
	<script src="/%s"></script>
</head>
<body></body>
</html>`

type browserTest struct {
	Name string
	Run  func(*testing.T, *rod.Page)
}

type browserDiagnostic struct {
	Kind   string
	Detail string
}

type styleState struct {
	Stylesheets int
	StyleNodes  int
	Toasts      int
	Inline      bool
	BoxSizing   string
	Position    string
	CloseImage  string
	Legacy      bool
}

type variantState struct {
	Name       string
	Background string
	Icon       string
	Inline     bool
}

func TestEmber(t *testing.T) {
	bin := os.Getenv("EMBER_BROWSER_BIN")
	if bin == "" {
		var found bool
		bin, found = launcher.LookPath()
		if !found {
			t.Fatal("install Chrome or Chromium, or set EMBER_BROWSER_BIN to its executable")
		}
	}

	files := []string{"Ember.js", "Ember.js.min"}
	mux := http.NewServeMux()
	for _, file := range files {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		mux.HandleFunc("GET /test/"+file, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprintf(w, testHTML, file)
		})
		mux.HandleFunc("GET /"+file, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			_, _ = w.Write(source)
		})
	}
	mux.HandleFunc("GET /diagnostics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, `<!doctype html><html><head><link rel="icon" href="data:,"></head><body>
			<script>const broken = ;</script>
			<script>
				console.error("diagnostic console error");
				console.assert(false, "diagnostic assertion");
				setTimeout(() => { throw new Error("diagnostic async error"); }, 0);
				Promise.reject(new Error("diagnostic rejection"));
				new OfflineAudioContext(1, 128, 44100).createScriptProcessor(256, 1, 1);
				cancelAnimationFrame(webkitRequestAnimationFrame(() => {}));
			</script>
		</body></html>`)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	browserCtx, cancelBrowser := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancelBrowser)
	browser := rod.New().Context(browserCtx)
	if err := browser.Launch(launcher.New().Bin(bin).Headless(true)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := browser.Close(); err != nil {
			t.Error(err)
		}
	})

	t.Run("diagnostic detection", func(t *testing.T) {
		page := newTestPage(t, browser)
		stop := collectBrowserDiagnostics(t, page)
		t.Cleanup(func() {
			actual := stop()
			for _, expected := range []browserDiagnostic{
				{Kind: "exception", Detail: "SyntaxError"},
				{Kind: "exception", Detail: "diagnostic async error"},
				{Kind: "exception", Detail: "diagnostic rejection"},
				{Kind: "console", Detail: "diagnostic console error"},
				{Kind: "console", Detail: "diagnostic assertion"},
				{Kind: "deprecation log", Detail: "ScriptProcessorNode"},
				{Kind: "deprecation audit", Detail: "PrefixedRequestAnimationFrame"},
			} {
				if !slices.ContainsFunc(actual, func(diagnostic browserDiagnostic) bool {
					return diagnostic.Kind == expected.Kind && strings.Contains(diagnostic.Detail, expected.Detail)
				}) {
					t.Errorf("missing browser diagnostic %+v; got %+v", expected, actual)
				}
			}
		})
		if err := page.Navigate(server.URL + "/diagnostics"); err != nil {
			t.Fatal(err)
		}
		if err := page.WaitLoad(); err != nil {
			t.Fatal(err)
		}
		runJS(t, page, `() => new Promise(resolve => setTimeout(resolve, 0))`)
		// A navigation must not erase diagnostics from earlier scripts.
		if err := page.Navigate(server.URL + "/test/Ember.js"); err != nil {
			t.Fatal(err)
		}
		if err := page.WaitLoad(); err != nil {
			t.Fatal(err)
		}
	})

	tests := []browserTest{
		{Name: "inline styles", Run: testInlineStyles},
		{Name: "variants", Run: testVariants},
		{Name: "handle lifecycle", Run: testLifecycle},
		{Name: "close control", Run: testCloseControl},
		{Name: "progress", Run: testProgress},
		{Name: "responsive placement", Run: testResponsivePlacement},
		{Name: "viewport ignores mobile user agent", Run: testMobileUserAgent},
		{Name: "native animations", Run: testNativeAnimations},
		{Name: "responsive breakpoint and stacking", Run: testResponsiveStacking},
		{Name: "modern browser APIs", Run: testModernBrowserAPIs},
		{Name: "escape keyboard control", Run: testEscapeControl},
		{Name: "interactive controls", Run: testInteractiveControls},
		{Name: "monotonic progress", Run: testMonotonicProgress},
		{Name: "clear destroy and reuse", Run: testDestroy},
		{Name: "early close", Run: testEarlyClose},
		{Name: "custom target cleanup", Run: testCustomTarget},
		{Name: "duplicate modes", Run: testDuplicates},
		{Name: "overlay", Run: testOverlay},
		{Name: "safe text and modern colors", Run: testSafeTextAndColors},
		{Name: "validation", Run: testValidation},
		{Name: "configure future defaults", Run: testConfigure},
		{Name: "abort signal", Run: testAbortSignal},
	}
	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			for _, test := range tests {
				t.Run(test.Name, func(t *testing.T) {
					page := newTestPage(t, browser)
					stop := collectBrowserDiagnostics(t, page)
					t.Cleanup(func() {
						for _, diagnostic := range stop() {
							t.Errorf("browser %s: %s", diagnostic.Kind, diagnostic.Detail)
						}
					})
					if err := page.Navigate(server.URL + "/test/" + file); err != nil {
						t.Fatal(err)
					}
					if err := page.WaitLoad(); err != nil {
						t.Fatal(err)
					}
					test.Run(t, page)
				})
			}
		})
	}
}

func newTestPage(t *testing.T, browser *rod.Browser) *rod.Page {
	t.Helper()
	page, err := browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	page = page.Context(ctx)
	t.Cleanup(func() {
		defer cancel()
		cleanupCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer stop()
		if err := page.Context(cleanupCtx).Close(); err != nil {
			t.Error(err)
		}
	})
	return page
}

// collectBrowserDiagnostics subscribes before navigation and retains events across reloads.
// Its returned function drains through a marker before releasing the subscription.
func collectBrowserDiagnostics(t *testing.T, page *rod.Page) func() []browserDiagnostic {
	t.Helper()
	ctx, cancel := context.WithCancel(context.WithoutCancel(page.GetContext()))
	t.Cleanup(cancel)
	marker := "ember-diagnostics-" + rand.Text()
	var diagnostics []browserDiagnostic
	record := func(kind string, event any) {
		detail, err := json.Marshal(event)
		if err != nil {
			t.Errorf("encode browser diagnostic: %v", err)
		}
		diagnostics = append(diagnostics, browserDiagnostic{Kind: kind, Detail: string(detail)})
	}
	wait := page.Context(ctx).EachEvent(
		rod.On(func(event *proto.RuntimeExceptionThrown, _ proto.TargetSessionID) bool {
			record("exception", event.ExceptionDetails)
			return false
		}),
		rod.On(func(event *proto.RuntimeConsoleAPICalled, _ proto.TargetSessionID) bool {
			if event.Type == proto.RuntimeConsoleAPICalledTypeDebug && len(event.Args) == 1 {
				var text string
				if event.Args[0].Value.Unmarshal(&text) == nil && text == marker {
					return true
				}
			}
			if event.Type == proto.RuntimeConsoleAPICalledTypeError || event.Type == proto.RuntimeConsoleAPICalledTypeAssert {
				record("console", event)
			}
			return false
		}),
		rod.On(func(event *proto.LogEntryAdded, _ proto.TargetSessionID) bool {
			if event.Entry.Source == proto.LogLogEntrySourceDeprecation {
				record("deprecation log", event.Entry)
			} else if event.Entry.Level == proto.LogLogEntryLevelError {
				record("log error", event.Entry)
			}
			return false
		}),
		rod.On(func(event *proto.AuditsIssueAdded, _ proto.TargetSessionID) bool {
			if event.Issue.Code == proto.AuditsInspectorIssueCodeDeprecationIssue {
				record("deprecation audit", event.Issue)
			}
			return false
		}),
	)
	return func() []browserDiagnostic {
		t.Helper()
		defer cancel()
		cleanupCtx, stop := context.WithTimeout(ctx, 5*time.Second)
		defer stop()
		stopCancel := context.AfterFunc(cleanupCtx, cancel)
		defer stopCancel()
		// Let queued tasks and rejection reporting run before the protocol marker.
		if err := page.Context(cleanupCtx).EvalJSON(nil, `async marker => {
			await new Promise(resolve => setTimeout(resolve, 0));
			console.debug(marker);
		}`, marker); err != nil {
			t.Errorf("finish browser diagnostics: %v", err)
			cancel()
		}
		if err := wait(); err != nil {
			t.Errorf("drain browser diagnostics: %v", err)
		}
		return diagnostics
	}
}

func testInlineStyles(t *testing.T, page *rod.Page) {
	state := evaluate[styleState](t, page, `() => {
		const toast = Ember.Show({title: "Saved", message: "Your changes are ready.", duration: 0});
		Ember.Show({message: "A second notification", duration: 0});
		const element = toast.Element;
		return {
			stylesheets: document.styleSheets.length,
			styleNodes: document.querySelectorAll("style, link[rel='stylesheet']").length,
			toasts: document.querySelectorAll(".ember").length,
			inline: toast instanceof EventTarget && element instanceof HTMLElement && element.style.length > 0,
			boxSizing: getComputedStyle(element).boxSizing,
			position: getComputedStyle(element.closest(".ember-wrapper")).position,
			closeImage: getComputedStyle(element.querySelector(".ember-close")).backgroundImage,
			legacy: "iziToast" in window || document.querySelector("[class*='iziToast'], [data-iziToast-ref]") !== null
		};
	}`)
	if state.Stylesheets != 0 || state.StyleNodes != 0 {
		t.Errorf("expected no stylesheets or style elements, got %+v", state)
	}
	if state.Toasts != 2 || !state.Inline || state.BoxSizing != "border-box" || state.Position != "fixed" {
		t.Errorf("toast layout was not applied inline: %+v", state)
	}
	if state.CloseImage == "none" || state.Legacy {
		t.Errorf("missing close icon or legacy namespace remains: %+v", state)
	}
}

func testVariants(t *testing.T, page *rod.Page) {
	states := evaluate[[]variantState](t, page, `() => {
		return ["Info", "Success", "Warning", "Error", "Question"].map(name => {
			const toast = Ember[name]({title: name, message: "Notification", duration: 0}).Element;
			const icon = toast.querySelector(".ember-icon");
			return {
				name,
				background: getComputedStyle(toast).backgroundColor,
				icon: getComputedStyle(icon).backgroundImage,
				inline: toast.style.length > 0 && icon.style.length > 0
			};
		});
	}`)
	backgrounds := make(map[string]bool)
	for _, state := range states {
		if !state.Inline || state.Background == "rgba(0, 0, 0, 0)" || state.Icon == "none" {
			t.Errorf("%s has incomplete inline appearance: %+v", state.Name, state)
		}
		backgrounds[state.Background] = true
	}
	if len(backgrounds) != len(states) {
		t.Errorf("expected distinct variant colors, got %v", states)
	}
	assertJS(t, page, "dark variants should use the same dark background and foreground as a default dark toast", `() => {
		const reference = Ember.Show({message: "Dark default", theme: "dark", duration: 0}).Element;
		const expected = getComputedStyle(reference);
		return ["Info", "Success", "Warning", "Error", "Question"].every(name => {
			const toast = Ember[name]({message: name, theme: "dark", duration: 0}).Element;
			const actual = getComputedStyle(toast);
			return actual.backgroundColor === expected.backgroundColor && actual.color === expected.color;
		});
	}`)
}

func testLifecycle(t *testing.T, page *rod.Page) {
	runJS(t, page, `() => {
		window.lifecycle = [];
		window.toast = Ember.Show({title: "Saved", message: "Complete", duration: 0, animationDuration: 20});
		for (const name of ["opening", "opened", "closing", "closed"]) {
			toast.addEventListener(name, event => {
				if (!(event instanceof CustomEvent) || event.detail.toast !== toast) throw new Error("invalid lifecycle detail");
				if (["closing", "closed"].includes(event.type) && event.detail.reason !== "test") throw new Error("invalid lifecycle reason");
				lifecycle.push(event.type);
			});
		}
	}`)
	waitJS(t, page, `() => toast.State === "open"`)
	assertJS(t, page, "Close should return the same completion promise and preserve the first reason", `async () => {
		const first = toast.Close("test");
		const second = toast.Close("ignored");
		const reason = await first;
		return first === toast.Closed && second === first && reason === "test" &&
			toast.State === "closed" && !toast.Element.isConnected;
	}`)
	events := evaluate[[]string](t, page, `() => lifecycle`)
	if !slices.Equal(events, []string{"opening", "opened", "closing", "closed"}) {
		t.Errorf("unexpected lifecycle: %v", events)
	}
}

func testCloseControl(t *testing.T, page *rod.Page) {
	runJS(t, page, `() => {
		window.toast = Ember.Show({message: "Dismiss me", duration: 0, animationDuration: 20});
	}`)
	waitJS(t, page, `() => toast.State === "open"`)
	clickElement(t, page, ".ember-close")
	if reason := evaluate[string](t, page, `() => toast.Closed`); reason != "button" {
		t.Errorf("close control reported reason %q", reason)
	}
	assertJS(t, page, "close control should remove the element", `() => !toast.Element.isConnected`)
}

func testProgress(t *testing.T, page *rod.Page) {
	assertJS(t, page, "Pause and Resume should be fluent and preserve the timer", `async () => {
		window.toast = Ember.Show({message: "Timed notification", duration: 180, animationDuration: 0});
		if (toast.Pause() !== toast) return false;
		await new Promise(resolve => setTimeout(resolve, 300));
		if (!toast.Element.isConnected || toast.State === "closing") return false;
		return toast.Resume() === toast;
	}`)
	if reason := evaluate[string](t, page, `() => toast.Closed`); reason != "timeout" {
		t.Errorf("resumed progress reported reason %q", reason)
	}
	assertJS(t, page, "Restart should restore the full timeout", `async () => {
		window.toast = Ember.Show({message: "Restarted notification", duration: 400, animationDuration: 0});
		await new Promise(resolve => setTimeout(resolve, 250));
		if (toast.Restart() !== toast) return false;
		await new Promise(resolve => setTimeout(resolve, 250));
		return toast.Element.isConnected && toast.State === "open";
	}`)
	waitJS(t, page, `() => toast.State === "closed"`)
}

func testResponsivePlacement(t *testing.T, page *rod.Page) {
	setViewport(t, page, 1280, 800)
	runJS(t, page, `() => {
		window.toast = Ember.Show({
			title: "A notification with a longer title", message: "Long notification text. ".repeat(12),
			position: "top-right", maxWidth: 480, duration: 0, animationDuration: 0
		});
	}`)
	waitJS(t, page, `() => toast.State === "open"`)
	assertJS(t, page, "desktop toast should be at the top right", `() => {
		const rect = toast.Element.getBoundingClientRect();
		return rect.right <= innerWidth && rect.right > innerWidth - 50 && rect.top >= 0 && rect.top < 50;
	}`)
	setViewport(t, page, 360, 640)
	waitJS(t, page, `() => {
		const rect = toast.Element.getBoundingClientRect();
		return rect.left >= 0 && rect.right <= innerWidth && rect.width > 0;
	}`)
	assertJS(t, page, "narrow viewport should use inline responsive layout", `() => {
		return document.styleSheets.length === 0 && toast.Element.style.length > 0 &&
			document.documentElement.scrollWidth <= innerWidth;
	}`)
	setViewport(t, page, 1280, 800)
	waitJS(t, page, `() => {
		const rect = toast.Element.getBoundingClientRect();
		return rect.right > innerWidth - 50 && rect.right <= innerWidth && rect.left > innerWidth / 2;
	}`)
}

func testMobileUserAgent(t *testing.T, page *rod.Page) {
	setViewport(t, page, 1280, 800)
	if err := page.SetUserAgent(&proto.NetworkSetUserAgentOverride{UserAgent: "Ember browser test Mobile"}); err != nil {
		t.Fatal(err)
	}
	if err := page.Reload(); err != nil {
		t.Fatal(err)
	}
	if err := page.WaitLoad(); err != nil {
		t.Fatal(err)
	}
	runJS(t, page, `() => {
		window.toast = Ember.Show({message: "Wide mobile browser", position: "top-right", duration: 0, animationDuration: 0});
	}`)
	waitJS(t, page, `() => toast.State === "open"`)
	assertJS(t, page, "wide viewport should use desktop placement even with a mobile user agent", `() => {
		const rect = toast.Element.getBoundingClientRect();
		return rect.left > innerWidth / 2 && rect.right <= innerWidth && rect.right > innerWidth - 50;
	}`)
}

func testNativeAnimations(t *testing.T, page *rod.Page) {
	setViewport(t, page, 360, 640)
	assertJS(t, page, "narrow viewport should animate without stylesheet animations", `() => {
		window.toast = Ember.Show({message: "Animated notification", duration: 0, animation: "flip", animationDuration: 200});
		return getComputedStyle(toast.Element).animationName === "none" && document.styleSheets.length === 0;
	}`)
	waitJS(t, page, `() => toast.Element.getAnimations().some(animation => animation.effect.getKeyframes().length > 1)`)
	waitJS(t, page, `() => toast.State === "open"`)
	runJS(t, page, `() => toast.Close()`)
	waitJS(t, page, `() => toast.State === "closed"`)
	assertJS(t, page, "animation none should complete opening without an active animation", `async () => {
		const toast = Ember.Show({message: "No motion", duration: 0, animation: "none"});
		await new Promise(resolve => toast.addEventListener("opened", resolve, {once: true}));
		return toast.State === "open" && toast.Element.getAnimations().length === 0;
	}`)
}

func testResponsiveStacking(t *testing.T, page *rod.Page) {
	setViewport(t, page, 569, 800)
	runJS(t, page, `() => {
		window.firstToast = Ember.Show({message: "First notification. ".repeat(8), position: "top-right", maxWidth: 280, duration: 0, animationDuration: 0});
		window.secondToast = Ember.Show({message: "Second notification. ".repeat(8), position: "top-right", maxWidth: 280, duration: 0, animationDuration: 0});
	}`)
	waitJS(t, page, `() => firstToast.State === "open" && secondToast.State === "open"`)
	assertJS(t, page, "569 pixels should use desktop placement with non-overlapping stacked toasts", `() => {
		const wrapper = firstToast.Element.closest(".ember-wrapper");
		const first = firstToast.Element.getBoundingClientRect();
		const second = secondToast.Element.getBoundingClientRect();
		return wrapper.querySelectorAll(".ember")[0] === secondToast.Element &&
			second.bottom <= first.top && first.left > innerWidth / 3 && first.right <= innerWidth;
	}`)
	setViewport(t, page, 568, 800)
	waitJS(t, page, `() => {
		const rect = firstToast.Element.getBoundingClientRect();
		return Math.abs(rect.left - (innerWidth - rect.right)) < 1;
	}`)
	assertJS(t, page, "568 pixels should center both toasts without reversing or overlapping them", `() => {
		const wrapper = firstToast.Element.closest(".ember-wrapper");
		const first = firstToast.Element.getBoundingClientRect();
		const second = secondToast.Element.getBoundingClientRect();
		return secondToast.Element.closest(".ember-wrapper") === wrapper &&
			wrapper.querySelectorAll(".ember")[0] === secondToast.Element && second.bottom <= first.top &&
			document.querySelectorAll(".ember-wrapper").length === 1 && document.styleSheets.length === 0;
	}`)
	setViewport(t, page, 569, 800)
	waitJS(t, page, `() => firstToast.Element.getBoundingClientRect().left > innerWidth / 3`)
	assertJS(t, page, "growing past the breakpoint should restore desktop placement and preserve stacking", `() => {
		const wrapper = firstToast.Element.closest(".ember-wrapper");
		const first = firstToast.Element.getBoundingClientRect();
		const second = secondToast.Element.getBoundingClientRect();
		return secondToast.Element.closest(".ember-wrapper") === wrapper &&
			wrapper.querySelectorAll(".ember")[0] === secondToast.Element && second.bottom <= first.top &&
			first.right <= innerWidth && document.querySelectorAll(".ember-wrapper").length === 1;
	}`)
}

func testModernBrowserAPIs(t *testing.T, page *rod.Page) {
	if _, err := page.EvalOnNewDocument(`
		Object.defineProperty(navigator, "userAgent", {
			get() { throw new Error("Ember must not read navigator.userAgent"); }
		});
		Object.defineProperty(HTMLElement.prototype, "currentStyle", {
			get() { throw new Error("Ember must not read currentStyle"); }
		});
	`); err != nil {
		t.Fatal(err)
	}
	if err := page.Reload(); err != nil {
		t.Fatal(err)
	}
	if err := page.WaitLoad(); err != nil {
		t.Fatal(err)
	}
	runJS(t, page, `() => {
		window.toast = Ember.Show({title: "Modern browser", message: "No legacy browser detection", duration: 0, animationDuration: 0});
	}`)
	waitJS(t, page, `() => toast.State === "open"`)
	assertJS(t, page, "modern layout should calculate a visible capsule without legacy style access", `() => {
		return toast.Element.getBoundingClientRect().height > 0 && toast.Element.parentElement.getBoundingClientRect().height > 0;
	}`)
	runJS(t, page, `() => toast.Close()`)
	waitJS(t, page, `() => !toast.Element.isConnected`)
}

func testEscapeControl(t *testing.T, page *rod.Page) {
	runJS(t, page, `() => {
		for (const name of ["keyCode", "which"]) {
			Object.defineProperty(KeyboardEvent.prototype, name, {
				get() { throw new Error("Ember must use KeyboardEvent.key"); }
			});
		}
		window.toast = Ember.Show({message: "Dismiss with Escape", duration: 0, closeOnEscape: true, animationDuration: 0});
		window.persistentToast = Ember.Show({message: "Remain open", duration: 0, animationDuration: 0});
	}`)
	waitJS(t, page, `() => toast.State === "open"`)
	if err := page.Keyboard.Type(input.Escape); err != nil {
		t.Fatal(err)
	}
	if reason := evaluate[string](t, page, `() => toast.Closed`); reason != "escape" {
		t.Errorf("Escape reported reason %q", reason)
	}
	assertJS(t, page, "Escape should close only opted-in toasts", `() => {
		return !toast.Element.isConnected && persistentToast.Element.isConnected && persistentToast.State === "open";
	}`)
}

func testInteractiveControls(t *testing.T, page *rod.Page) {
	runJS(t, page, `() => {
		window.controlEvents = [];
		window.content = document.createElement("div");
		for (const name of ["first", "second"]) {
			const input = document.createElement("input");
			input.id = name + "-input";
			input.addEventListener("input", event => controlEvents.push({kind: "input", id: input.id, value: input.value, valid: event.isTrusted}));
			content.append(input);
		}
		const onClick = ({toast: receivedToast, event}) => {
			controlEvents.push({kind: "button", label: event.currentTarget.textContent,
				values: Array.from(content.children, input => input.value),
				valid: receivedToast === toast && event.isTrusted && toast.Element.contains(event.currentTarget)});
		};
		window.toast = Ember.Show({
			message: "Interactive notification", duration: 0, content, animationDuration: 0,
			actions: [{label: "First", onClick}, {label: "Second", onClick}]
		});
	}`)
	waitJS(t, page, `() => toast.State === "open"`)
	for index, selector := range []string{"#first-input", "#second-input"} {
		element, err := page.Element(selector)
		if err != nil {
			t.Fatal(err)
		}
		if err := element.Focus(); err != nil {
			t.Fatal(err)
		}
		if err := page.InsertText(fmt.Sprintf("value-%d", index+1)); err != nil {
			t.Fatal(err)
		}
	}
	for _, label := range []string{"First", "Second"} {
		element, err := page.ElementR(".ember button", "^"+label+"$")
		if err != nil {
			t.Fatal(err)
		}
		if err := element.Click(proto.InputMouseButtonLeft, 1); err != nil {
			t.Fatal(err)
		}
	}
	assertJS(t, page, "native content and named actions should preserve DOM identity and callback context", `() => {
		const inputs = controlEvents.filter(event => event.kind === "input");
		const buttons = controlEvents.filter(event => event.kind === "button");
		return toast.Element.contains(content) && controlEvents.every(event => event.valid) &&
			inputs.some(event => event.id === "first-input" && event.value === "value-1") &&
			inputs.some(event => event.id === "second-input" && event.value === "value-2") &&
			buttons.length === 2 && buttons[0].label === "First" && buttons[1].label === "Second" &&
			buttons.every(event => event.values.join() === "value-1,value-2") && document.styleSheets.length === 0;
	}`)
}

func testMonotonicProgress(t *testing.T, page *rod.Page) {
	assertJS(t, page, "a wall-clock adjustment must not exhaust paused progress", `async () => {
		window.toast = Ember.Show({message: "Clock adjustment", duration: 600, animationDuration: 0});
		await new Promise(resolve => setTimeout(resolve, 150));
		const originalNow = Date.now;
		try {
			Date.now = () => originalNow() + 60000;
			toast.Pause();
		} finally {
			Date.now = originalNow;
		}
		toast.Resume();
		await new Promise(resolve => setTimeout(resolve, 100));
		return toast.Element.isConnected && toast.State === "open";
	}`)
	waitJS(t, page, `() => !toast.Element.isConnected`)
}

func testDestroy(t *testing.T, page *rod.Page) {
	assertJS(t, page, "Clear should await all current toasts while allowing new notifications", `async () => {
		const first = Ember.Show({message: "First", duration: 0, animationDuration: 10});
		const second = Ember.Show({message: "Second", duration: 0, overlay: {}, animationDuration: 10});
		const clearing = Ember.Clear();
		const retained = Ember.Show({message: "Created during Clear", duration: 0, animationDuration: 0});
		const reasons = await clearing;
		return reasons.length === 2 && reasons.every(reason => reason === "clear") &&
			!first.Element.isConnected && !second.Element.isConnected && retained.Element.isConnected;
	}`)
	assertJS(t, page, "Destroy should remove active notifications and overlays, then permit reuse", `async () => {
		const toast = Ember.Show({message: "Removed", duration: 50, overlay: {}});
		Ember.Destroy();
		if (document.querySelector(".ember, .ember-wrapper, .ember-overlay")) return false;
		if (await toast.Closed !== "destroy") return false;
		await new Promise(resolve => setTimeout(resolve, 100));
		if (document.querySelector(".ember, .ember-wrapper, .ember-overlay")) return false;
		const next = Ember.Success({message: "Ready again", duration: 0});
		return next.Element.isConnected && next.Element.style.length > 0 && document.styleSheets.length === 0;
	}`)
}

func testEarlyClose(t *testing.T, page *rod.Page) {
	assertJS(t, page, "closing during opening should settle once and never report opened", `async () => {
		const events = [];
		const toast = Ember.Show({message: "Close while opening", duration: 0, animationDuration: 20});
		for (const name of ["opening", "opened", "closing", "closed"]) {
			toast.addEventListener(name, () => events.push(name));
		}
		toast.addEventListener("opening", () => {
			toast.Close("early");
			toast.Close("again");
		});
		return await toast.Closed === "early" && events.join() === "opening,closing,closed" && !toast.Element.isConnected;
	}`)
	assertJS(t, page, "Destroy from opening should settle and remove the toast", `async () => {
		const toast = Ember.Show({message: "Destroy while opening", duration: 0});
		toast.addEventListener("opening", () => Ember.Destroy());
		return await toast.Closed === "destroy" && toast.State === "closed" && !toast.Element.isConnected;
	}`)
}

func testCustomTarget(t *testing.T, page *rod.Page) {
	assertJS(t, page, "custom HTMLElement target should contain the toasts", `() => {
		window.target = document.createElement("div");
		document.body.append(target);
		window.toast = Ember.Show({message: "Targeted", target, duration: 0, animationDuration: 0});
		window.secondToast = Ember.Show({message: "Another targeted toast", target, duration: 0, animationDuration: 0});
		return target.contains(toast.Element) && target.contains(secondToast.Element);
	}`)
	waitJS(t, page, `() => toast.State === "open" && secondToast.State === "open"`)
	assertJS(t, page, "targeted toasts should stack without overlapping", `() => {
		const first = toast.Element.getBoundingClientRect();
		const second = secondToast.Element.getBoundingClientRect();
		return first.height > 0 && second.height > 0 && (first.bottom <= second.top || second.bottom <= first.top);
	}`)
	runJS(t, page, `() => toast.Close()`)
	waitJS(t, page, `() => !toast.Element.isConnected`)
	assertJS(t, page, "closing one targeted toast should preserve its sibling", `() => secondToast.Element.isConnected && target.contains(secondToast.Element)`)
	runJS(t, page, `() => secondToast.Close()`)
	waitJS(t, page, `() => !secondToast.Element.isConnected`)
	assertJS(t, page, "Close and Destroy should clean capsules while preserving the caller's target", `() => {
		if (target.childElementCount !== 0) return false;
		Ember.Show({message: "Targeted again", target, duration: 0});
		Ember.Destroy();
		return target.isConnected && target.childElementCount === 0;
	}`)
}

func testDuplicates(t *testing.T, page *rod.Page) {
	assertJS(t, page, "ignore should return the existing handle for an explicit ID containing selector syntax", `() => {
		window.firstToast = Ember.Show({id: "saved: item [1]", message: "First", duplicate: "ignore", duration: 0});
		const duplicate = Ember.Show({id: "saved: item [1]", message: "Duplicate", duplicate: "ignore", duration: 0});
		return firstToast.Element.isConnected && duplicate === firstToast && document.querySelectorAll(".ember").length === 1;
	}`)
	assertJS(t, page, "replace should close the previous handle and leave its replacement", `async () => {
		const replacement = Ember.Show({id: "saved: item [1]", message: "Replacement", duplicate: "replace", duration: 0});
		await firstToast.Closed;
		return replacement !== firstToast && replacement.Element.isConnected && !firstToast.Element.isConnected &&
			document.querySelectorAll(".ember").length === 1;
	}`)
	assertJS(t, page, "toasts without explicit IDs should not be deduplicated by their text", `() => {
		const first = Ember.Show({message: "Same message", duplicate: "ignore", duration: 0});
		const second = Ember.Show({message: "Same message", duplicate: "ignore", duration: 0});
		return first !== second && first.Element.isConnected && second.Element.isConnected;
	}`)
}

func testOverlay(t *testing.T, page *rod.Page) {
	assertJS(t, page, "overlay should cover the viewport without a stylesheet", `() => {
		window.toast = Ember.Show({
			message: "Click outside to dismiss", overlay: {closeOnClick: true}, duration: 0, animationDuration: 0
		});
		const overlay = document.querySelector(".ember-overlay");
		const rect = overlay.getBoundingClientRect();
		return overlay.style.length > 0 && getComputedStyle(overlay).position === "fixed" &&
			rect.left <= 0 && rect.right >= innerWidth && rect.top <= 0 && rect.bottom >= innerHeight && document.styleSheets.length === 0;
	}`)
	waitJS(t, page, `() => toast.State === "open"`)
	clickElement(t, page, ".ember-overlay")
	if reason := evaluate[string](t, page, `() => toast.Closed`); reason != "overlay" {
		t.Errorf("overlay click reported reason %q", reason)
	}
	assertJS(t, page, "overlay dismissal should remove the overlay", `() => !toast.Element.isConnected && document.querySelector(".ember-overlay") === null`)
}

func testSafeTextAndColors(t *testing.T, page *rod.Page) {
	assertJS(t, page, "title, message, and action labels should render as text", `() => {
		const markup = '<img src="invalid" onerror="window.unsafeMarkupRan = true">';
		const toast = Ember.Show({title: markup, message: markup, actions: [{label: markup, onClick() {}}], duration: 0});
		return toast.Element.textContent.includes(markup) && toast.Element.querySelector("img") === null && !window.unsafeMarkupRan;
	}`)
	assertJS(t, page, "modern CSS colors should be accepted and applied", `() => {
		const color = "oklch(75% 0.12 180)";
		const toast = Ember.Show({message: "Modern color", color, duration: 0});
		const reference = document.createElement("div");
		reference.style.backgroundColor = color;
		document.body.append(reference);
		const expected = getComputedStyle(reference).backgroundColor;
		reference.remove();
		return CSS.supports("color", color) && getComputedStyle(toast.Element).backgroundColor === expected;
	}`)
}

func testValidation(t *testing.T, page *rod.Page) {
	assertJS(t, page, "invalid and legacy options should fail before modifying the DOM", `() => {
		const connected = document.createElement("span");
		document.body.append(connected);
		const invalid = [
			{timeout: false}, {buttons: []}, {onOpened() {}}, {position: "topRight"},
			{duration: -1}, {duration: Infinity}, {color: "definitely-not-a-color"},
			{target: "body"}, {animation: "fadeInUp"}, {actions: [["Old tuple", () => {}]]},
			{overlay: true}, {message: document.createElement("span")}, {content: connected}, {icon: connected}
		];
		return invalid.every(options => {
			const before = document.body.innerHTML;
			try { Ember.Show(options); return false; }
			catch (error) { return error instanceof TypeError && document.body.innerHTML === before; }
		}) && ["Hide", "Settings", "Progress", "GetSetting", "SetSetting", "Children", "Toast"].every(name => !(name in Ember));
	}`)
	assertJS(t, page, "all creation helpers should reject null, arrays, and numbers before modifying the DOM", `() => {
		return ["Show", "Info", "Success", "Warning", "Error", "Question"].every(name => {
			return [null, [], 42].every(options => {
				const before = document.body.innerHTML;
				try { Ember[name](options); return false; }
				catch (error) { return error instanceof TypeError && document.body.innerHTML === before; }
			});
		});
	}`)
	assertJS(t, page, "content or icons containing the target should be rejected without moving caller-owned nodes", `() => {
		return ["content", "icon"].every(option => {
			const source = document.createElement("div");
			const target = document.createElement("div");
			const retained = document.createElement("span");
			retained.textContent = "Caller content";
			target.append(retained);
			source.append(target);
			const before = document.body.innerHTML;
			try { Ember.Show({[option]: source, target}); return false; }
			catch (error) {
				return error instanceof TypeError && source.parentNode === null && source.firstChild === target &&
					target.firstChild === retained && target.childNodes.length === 1 && document.body.innerHTML === before;
			}
		});
	}`)
}

func testConfigure(t *testing.T, page *rod.Page) {
	assertJS(t, page, "Configure should affect future toasts without disposing existing ones", `() => {
		window.firstToast = Ember.Show({message: "Existing", color: "tomato", duration: 0});
		const initialColor = getComputedStyle(firstToast.Element).backgroundColor;
		Ember.Configure({color: "rebeccapurple", duration: 0});
		window.secondToast = Ember.Show({message: "Configured"});
		return firstToast.Element.isConnected && getComputedStyle(firstToast.Element).backgroundColor === initialColor &&
			getComputedStyle(secondToast.Element).backgroundColor === "rgb(102, 51, 153)";
	}`)
	assertJS(t, page, "Destroy should reset configured defaults", `() => {
		Ember.Destroy();
		const toast = Ember.Show({message: "Defaults restored", duration: 0});
		return !firstToast.Element.isConnected && !secondToast.Element.isConnected &&
			getComputedStyle(toast.Element).backgroundColor !== "rgb(102, 51, 153)";
	}`)
}

func testAbortSignal(t *testing.T, page *rod.Page) {
	assertJS(t, page, "AbortSignal should close its toast and leave others open", `async () => {
		const controller = new AbortController();
		const toast = Ember.Show({message: "Abortable", signal: controller.signal, duration: 0, animationDuration: 0});
		const retained = Ember.Show({message: "Unrelated", duration: 0, animationDuration: 0});
		await new Promise(resolve => toast.addEventListener("opened", resolve, {once: true}));
		controller.abort();
		return await toast.Closed === "aborted" && toast.State === "closed" && !toast.Element.isConnected && retained.Element.isConnected;
	}`)
	assertJS(t, page, "an already-aborted signal should return a closed, unmounted handle", `async () => {
		const controller = new AbortController();
		controller.abort();
		const content = document.createDocumentFragment();
		const child = document.createElement("span");
		child.textContent = "Preserved content";
		content.append(child);
		const icon = document.createElement("span");
		const before = document.body.innerHTML;
		const toast = Ember.Show({message: "Already aborted", signal: controller.signal, content, icon});
		return toast.State === "closed" && !toast.Element.isConnected &&
			await toast.Closed === "aborted" && document.body.innerHTML === before &&
			content.childNodes.length === 1 && content.firstChild === child && icon.parentNode === null;
	}`)
	assertJS(t, page, "aborting from a custom element connection should not lose cancellation or emit opened", `async () => {
		const controller = new AbortController();
		let connections = 0;
		class AbortOnConnect extends HTMLElement {
			connectedCallback() {
				connections++;
				controller.abort();
			}
		}
		customElements.define("ember-abort-on-connect", AbortOnConnect);
		const content = document.createElement("ember-abort-on-connect");
		const toast = Ember.Show({content, signal: controller.signal, duration: 0, animationDuration: 0, overlay: {}});
		let opened = false;
		toast.addEventListener("opened", () => opened = true);
		return await toast.Closed === "aborted" && connections === 1 && !opened &&
			toast.State === "closed" && !toast.Element.isConnected && document.querySelector(".ember-overlay") === null;
	}`)
}

func clickElement(t *testing.T, page *rod.Page, selector string) {
	t.Helper()
	element, err := page.Element(selector)
	if err != nil {
		t.Fatal(err)
	}
	if err := element.Click(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
}

func setViewport(t *testing.T, page *rod.Page, width, height int) {
	t.Helper()
	if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width: width, Height: height, DeviceScaleFactor: 1,
	}); err != nil {
		t.Fatal(err)
	}
}

func evaluate[T any](t *testing.T, page *rod.Page, script string) T {
	t.Helper()
	var value T
	if err := page.EvalJSON(&value, script); err != nil {
		t.Fatal(err)
	}
	return value
}

func runJS(t *testing.T, page *rod.Page, script string) {
	t.Helper()
	if err := page.EvalJSON(nil, script); err != nil {
		t.Fatal(err)
	}
}

func assertJS(t *testing.T, page *rod.Page, message, script string) {
	t.Helper()
	if !evaluate[bool](t, page, script) {
		t.Fatal(message)
	}
}

func waitJS(t *testing.T, page *rod.Page, script string) {
	t.Helper()
	if err := page.Wait(rod.Eval(script)); err != nil {
		t.Fatal(err)
	}
}
