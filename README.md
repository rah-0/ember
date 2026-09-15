# Ember

Self-contained JavaScript toast notifications, with Go and Rod browser tests.

Load [Ember.js](Ember.js) directly. Ember applies styles to each element and uses
the Web Animations API for motion. It creates no stylesheet and needs no CSS,
LESS, JavaScript dependencies, or build step. Built-in icons are embedded.

## Usage

```html
<script type="module">
    const response = await fetch(
        "https://raw.githubusercontent.com/rah-0/ember/master/Ember.js"
    );
    if (!response.ok) throw new Error("Could not load Ember.");
    const url = URL.createObjectURL(new Blob(
        [await response.text()], {type: "text/javascript"}
    ));
    try {
        await import(url);
    } finally {
        URL.revokeObjectURL(url);
    }

    const toast = Ember.Success({
        title: "Saved",
        message: "Your changes are ready.",
        duration: 5000,
        position: "top-right"
    });

    // Close a notification before its timeout.
    toast.Close();
</script>
```

Call Ember after the document body exists. This loads the `master` branch
directly from GitHub. GitHub serves raw files as `text/plain` with `nosniff`, so
the loader fetches the source and imports a JavaScript Blob, then releases its
temporary URL. This avoids a third-party CDN or GitHub Pages setup.

Download and open [examples/index.html](examples/index.html) in a browser for
24 runnable examples with code snippets: toast types, positioning, multi-line
layouts, custom styles, light/dark themes, animations, actions, forms, timer
controls, overlays, duplicate handling, cancellation, and shared defaults. The
gallery loads Ember from the same raw GitHub URL.

### API

| Method | Behavior |
| --- | --- |
| `Show(options)` | Create a notification and return its toast handle. |
| `Info`, `Success`, `Warning`, `Error`, `Question` | Create a toast with the corresponding built-in type; accept the same options as `Show`. |
| `Configure(options)` | Update defaults for future toasts. Existing toasts keep their settings. |
| `Clear()` | Close all current toasts; returns a promise that resolves after they close. Keeps configured defaults. |
| `Destroy()` | Immediately remove all current toasts, cancel pending work, and reset defaults. |

Each toast is an `EventTarget` with these members:

| Member | Behavior |
| --- | --- |
| `Element` | Read-only reference to its DOM element. |
| `State` | Read-only `"opening"`, `"open"`, `"closing"`, or `"closed"`. |
| `Closed` | Promise that resolves with the close reason after removal. |
| `Close(reason = "api")` | Close the toast and return `Closed`. The reason must be a non-empty string. Repeated calls return the same promise. |
| `Pause()`, `Resume()`, `Restart()` | Control the timeout and return the toast for chaining. Restart begins the full duration again. |

Listen for `opening`, `opened`, `closing`, and `closed` on the toast. Each event
has `{ toast, reason }` in `event.detail`. The `opening` event runs in a microtask,
so listeners attached immediately after creation receive it.

```js
const toast = Ember.Info({ message: "Uploading…", duration: 0 });
toast.addEventListener("opened", () => console.log("Visible"), { once: true });
toast.Closed.then(reason => console.log("Closed:", reason));

// When the upload finishes:
await toast.Close("uploaded");
```

Built-in close reasons are `api`, `button`, `escape`, `click`, `drag`, `overlay`,
`timeout`, `replaced`, `clear`, `destroy`, and `aborted`.

### Options

Options use lower camel case; methods and handle properties use PascalCase.
Unknown options and invalid values throw before Ember changes the document.

| Option | Values / behavior |
| --- | --- |
| `title`, `message` | Plain text strings. HTML characters display literally. |
| `type` | `"default"`, `"info"`, `"success"`, `"warning"`, `"error"`, or `"question"`. |
| `duration` | Non-negative finite milliseconds; defaults to `5000`. Set `0` to keep the toast open. |
| `position` | `"top-left"`, `"top-center"`, `"top-right"`, `"bottom-left"`, `"bottom-center"`, `"bottom-right"`, or `"center"`. |
| `target`, `newestFirst` | An existing `HTMLElement` container or `null`; `newestFirst` defaults to `true` for targeted toasts. |
| `id`, `duplicate` | Non-empty string ID or `null`; `"stack"`, `"ignore"`, or `"replace"`. Ignore returns the existing handle; replace closes existing matches. |
| `theme`, `color` | `"light"` or `"dark"`; override the background with a CSS color or `null`. Modern CSS color functions are supported when the browser supports them. |
| `direction`, `layout` | `"ltr"` or `"rtl"`; `"inline"` or `"stacked"`. |
| `maxWidth`, `zIndex` | Maximum width as non-negative finite pixels or a CSS `max-width` value, default `480`; integer stacking order, default `99999`. |
| `closable`, `closeOnEscape`, `closeOnClick` | Boolean dismissal controls; default to `true`, `false`, and `false`. |
| `draggable`, `hover`, `progress` | Drag dismissal, default `true`; `"pause"`, `"restart"`, or `"none"`; progress bar visibility, default `true`. |
| `overlay` | `null` or `{ color: "rgba(0, 0, 0, 0.6)", closeOnClick: true }`. |
| `animation`, `animationDuration` | `"slide"`, `"fade"`, `"bounce"`, `"flip"`, or `"none"`; non-negative finite milliseconds, default `250`. Entry and exit motion respect reduced-motion preferences. |
| `actions` | Array of `{ label, onClick, autoFocus?, disabled? }`. Labels are non-empty plain text; handlers receive `{ toast, event }`. |
| `content`, `icon` | Custom `Element` or `DocumentFragment` content; an icon `Element`, `false` to hide, or `null` for the built-in icon. |
| `signal` | An `AbortSignal` or `null`. Aborting closes with reason `"aborted"`; an already aborted signal returns a closed, unmounted handle. |

Duplicate handling uses explicit IDs, stored in `data-ember-id`; Ember does not
derive IDs from messages or assign them to the element's `id` attribute.
At widths up to 568px, notifications use centered placement and adapt to the
viewport. Responsive behavior follows the viewport, without user-agent sniffing.

`Configure` accepts reusable defaults. Supply `id`, `content`, `icon`, `target`,
`actions`, and `signal` separately when creating each toast.

### Actions and custom content

Action buttons keep their toast open unless their handler calls `Close()`.

```js
Ember.Question({
    title: "Continue?",
    message: "Your changes will be saved.",
    duration: 0,
    actions: [
        { label: "Save", onClick: ({ toast }) => toast.Close("save") },
        { label: "Cancel", onClick: ({ toast }) => toast.Close("cancel") }
    ]
});

const input = document.createElement("input");
input.type = "text";
input.placeholder = "Name";
input.setAttribute("aria-label", "Name");
input.addEventListener("input", event => console.log(event.target.value));
Ember.Show({ title: "Your name", content: input, duration: 0 });
```

Ember inserts the nodes supplied as `content` or `icon`; it does not clone them.
Create fresh nodes for each toast. Connected nodes are rejected, and a
`DocumentFragment` is consumed when its children move into the toast. Ember never
parses strings as HTML. Caller-created content can still load external resources
or execute its own handlers. Custom nodes and targets must belong to the current
document. An already aborted signal leaves supplied custom nodes untouched.

## Minify

Run from the repository root:

```sh
go run .
```

This writes [Ember.js.min](Ember.js.min) using esbuild's Go API. It removes
comments, including the license header, and preserves Ember's public API and
modern JavaScript syntax. The source, [LICENSE](LICENSE), and [NOTICE](NOTICE)
retain the attribution. No Node or npm installation is needed.

```html
<script src="Ember.js.min"></script>
```

Serve this file with a JavaScript content type, such as `text/javascript`, since
its filename ends in `.min`.

## Tests

Requirements: Go 1.27.1 and an installed Chrome or Chromium browser. Set
`EMBER_BROWSER_BIN` when its executable is outside the usual locations.

```sh
go run .
go test -count=1 -race -cover -covermode=atomic ./...
go vet ./...
node --check Ember.js
```

Tests serve both JavaScript files locally and use `github.com/rah-0/rod` to check
rendering, inline styles, notification lifecycle, controls, and responsive
placement. A freshness check fails if `Ember.js.min` needs regeneration.
Every browser scenario fails on uncaught JavaScript exceptions, unhandled promise
rejections, console errors, failed console assertions, and browser error logs or
deprecation reports. Collection starts before Ember loads and continues across
reloads. A detector self-test checks deliberately broken scripts and deprecated
APIs. These checks cover executed paths and deprecations reported by the installed
browser; they do not prove that unexecuted code is error-free or detect every
deprecated API.

Go coverage does not measure JavaScript statement coverage. Node is needed only
for the optional syntax check.

Gallery tests click the example controls on desktop and mobile layouts. They
intercept the raw GitHub request and serve the local `Ember.js` with GitHub's
content type and CORS headers. This verifies the loader without network access
or publishing changes first, including a failed request and its disabled controls.

## License

Ember derives from iziToast v1.4.0 by Marcelo Dolce and is distributed under
[Apache-2.0](LICENSE). See [NOTICE](NOTICE) for attribution.

## ☕ Support

🔥 **Like your toasts hot and your dependencies at zero?** Buy me a coffee and help keep Ember burning. Your UI gets the toasts; I get the caffeine. ☕

[![Buy Me A Coffee](https://cdn.buymeacoffee.com/buttons/default-orange.png)](https://www.buymeacoffee.com/rah.0)
