# API migration

Ember exposes toast handles, plain text messages, DOM content, and native events.
The iziToast-shaped Ember API has no compatibility aliases.

| Previous API | Current API |
| --- | --- |
| `Ember.Show(options)` returns a DOM element | Returns a toast handle; use `toast.Element` for its element. Typed helpers return handles too. |
| `Ember.Hide({}, element, reason)` | `toast.Close(reason)`; await its promise or `toast.Closed` for removal. |
| `Ember.Settings(options)` | `Ember.Configure(options)` updates future defaults without removing current toasts. Use `Ember.Clear()` to close them. |
| `Ember.Progress({}, element)` | `toast.Pause()`, `toast.Resume()`, and `toast.Restart()`. Restart immediately begins the full duration. |
| `Children`, `Toast`, `GetSetting`, `SetSetting` | Retain the returned handle; use its read-only `Element`, `State`, and `Closed`. Settings are supplied at creation. |
| `onOpening`, `onOpened`, `onClosing`, `onClosed` | `toast.addEventListener("opening" / "opened" / "closing" / "closed", handler)`; read `{ toast, reason }` from `event.detail`. |
| Document `ember-*` lifecycle events | Listen on the toast handle. |
| `timeout: false` or `timeout: 5000` | `duration: 0` or `duration: 5000`. |
| `position: "topRight"` | `position: "top-right"`; all positions use lowercase hyphenated names. |
| Selector `target`, `targetFirst` | An existing `HTMLElement` as `target`, and `newestFirst`. |
| Numeric or string `displayMode` | `duplicate: "stack" / "ignore" / "replace"` with an explicit `id`. Ignore returns the existing handle; IDs are not derived from content. |
| `id` sets the DOM element's ID | `id` identifies duplicates and is stored in `data-ember-id`. Use the handle's `Element` for DOM access. |
| HTML `title` / `message` strings | Plain text. Build elements with DOM APIs and pass `content` for custom markup or inputs. |
| Positional `buttons` tuples | Named `actions` objects: `{ label, onClick, autoFocus?, disabled? }`. Handlers receive `{ toast, event }`. |
| `inputs`, `image`, `imageWidth` | Build the desired DOM nodes, attach handlers, and pass `content`. |
| `icon`, `iconText`, `iconColor`, `iconUrl` | An `Element` as `icon`, `false` to hide it, or `null` for the built-in type icon. |
| `close`, `drag`, `progressBar`, `zindex`, `rtl` | `closable`, `draggable`, `progress`, `zIndex`, `direction: "rtl"`. |
| `pauseOnHover`, `resetOnHover` | `hover: "pause" / "restart" / "none"`. |
| `layout: 1 / 2` | `layout: "inline" / "stacked"`. |
| `backgroundColor`, named palette `color` | `color` accepts a CSS color. Use typed helpers or `type` for built-in palettes. |
| `overlay`, `overlayColor`, `overlayClose` | `overlay: null` or `{ color, closeOnClick }`. |
| Entry/exit and mobile transition names | `animation: "slide" / "fade" / "bounce" / "flip" / "none"` and `animationDuration`. |
| Typography, CSS class, balloon, and progress styling options | Style `toast.Element` or custom content directly where needed. These option names are removed. |

`Configure` rejects per-toast `id`, `content`, `icon`, `target`, `actions`, and
`signal`. Create fresh custom nodes per toast; connected nodes are rejected.
Unknown options and invalid values throw before document mutation.

`Destroy()` immediately closes handles with reason `"destroy"` and resets
defaults. `Clear()` awaits normal closure with reason `"clear"` and keeps
defaults. Cancellation uses `signal: AbortSignal`; an already aborted signal
returns a closed handle without inserting a notification.
