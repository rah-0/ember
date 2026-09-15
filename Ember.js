/**
 * Ember.js - self-contained toast notifications.
 * Derived from iziToast v1.4.0 by Marcelo Dolce.
 * Original project: https://github.com/marcelodolza/iziToast
 * Licensed under Apache-2.0; see LICENSE and NOTICE.
 * Modified for Ember: direct element styles and a modern toast lifecycle API.
 */

window.Ember = (function () {
    "use strict";

    const compactViewport = window.matchMedia("(width <= 568px)");
    const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)");
    const records = new Map();
    const wrappers = new Map();
    let configuration = {};

    const Colors = {
        default: "rgba(238,238,238,0.9)",
        info: "rgba(157,222,255,0.9)",
        success: "rgba(166,239,184,0.9)",
        warning: "rgba(255,207,165,0.9)",
        error: "rgba(255,175,180,0.9)",
        question: "rgba(255,249,178,0.9)",
    };
    const IconPaths = {
        info: '<circle cx="12" cy="12" r="9"/><path d="M12 11v6m0-10v1"/>',
        warning: '<path d="M12 3 2 21h20L12 3Zm0 6v5m0 3v1"/>',
        error: '<circle cx="12" cy="12" r="9"/><path d="m9 9 6 6m0-6-6 6"/>',
        success: '<path d="m4 12 5 5L20 6"/>',
        question:
            '<circle cx="12" cy="12" r="9"/><path d="M9 9a3 3 0 0 1 6 0c0 2-3 2-3 5m0 3v1"/>',
        close: '<path d="m5 5 14 14m0-14L5 19" stroke-width="3"/>',
    };
    const defaults = Object.freeze({
        id: null,
        type: "default",
        title: "",
        message: "",
        content: null,
        icon: null,
        duration: 5000,
        position: "bottom-right",
        target: null,
        newestFirst: true,
        duplicate: "stack",
        theme: "light",
        color: null,
        direction: "ltr",
        layout: "inline",
        maxWidth: 480,
        zIndex: 99999,
        closable: true,
        closeOnEscape: false,
        closeOnClick: false,
        draggable: true,
        hover: "pause",
        progress: true,
        overlay: null,
        actions: [],
        animation: "slide",
        animationDuration: 250,
        signal: null,
    });
    const enums = {
        type: Object.keys(Colors),
        position: [
            "top-left",
            "top-center",
            "top-right",
            "bottom-left",
            "bottom-center",
            "bottom-right",
            "center",
        ],
        duplicate: ["stack", "ignore", "replace"],
        theme: ["light", "dark"],
        direction: ["ltr", "rtl"],
        layout: ["inline", "stacked"],
        hover: ["pause", "restart", "none"],
        animation: ["slide", "fade", "bounce", "flip", "none"],
    };
    const localOptions = new Set([
        "id",
        "content",
        "icon",
        "target",
        "actions",
        "signal",
    ]);

    function validateCondition(condition, message) {
        if (!condition) throw new TypeError("Ember: " + message);
    }

    function checkKeys(value, allowed, name) {
        validateCondition(
            value !== null &&
                typeof value === "object" &&
                !Array.isArray(value),
            name + " must be an object",
        );
        for (const key of Object.keys(value)) {
            validateCondition(
                allowed.includes(key),
                "unknown " + name + " option: " + key,
            );
        }
    }

    function colorValue(value, name) {
        validateCondition(
            typeof value === "string" && CSS.supports("color", value),
            name + " must be a valid CSS color",
        );
        return value;
    }

    function normalize(input, base = configuration) {
        checkKeys(input, Object.keys(defaults), "toast");
        const options = { ...defaults, ...base, ...input };
        for (const [key, values] of Object.entries(enums)) {
            validateCondition(
                values.includes(options[key]),
                key + " must be one of: " + values.join(", "),
            );
        }
        for (const key of ["title", "message"]) {
            validateCondition(
                typeof options[key] === "string",
                key + " must be a string",
            );
        }
        validateCondition(
            options.id === null ||
                (typeof options.id === "string" && options.id.length > 0),
            "id must be a non-empty string or null",
        );
        for (const key of ["duration", "animationDuration"]) {
            validateCondition(
                Number.isFinite(options[key]) && options[key] >= 0,
                key + " must be a non-negative finite number",
            );
        }
        validateCondition(
            Number.isInteger(options.zIndex),
            "zIndex must be an integer",
        );
        for (const key of [
            "newestFirst",
            "closable",
            "closeOnEscape",
            "closeOnClick",
            "draggable",
            "progress",
        ]) {
            validateCondition(
                typeof options[key] === "boolean",
                key + " must be a boolean",
            );
        }
        validateCondition(
            options.color === null || typeof options.color === "string",
            "color must be a CSS color or null",
        );
        if (options.color !== null) colorValue(options.color, "color");
        validateCondition(
            (typeof options.maxWidth === "number" &&
                Number.isFinite(options.maxWidth) &&
                options.maxWidth >= 0) ||
                (typeof options.maxWidth === "string" &&
                    CSS.supports("max-width", options.maxWidth)),
            "maxWidth must be pixels or a CSS length",
        );
        validateCondition(
            options.target === null ||
                (options.target instanceof HTMLElement &&
                    options.target.ownerDocument === document),
            "target must be an HTMLElement in this document or null",
        );
        validateCondition(
            options.content === null ||
                options.content instanceof Element ||
                options.content instanceof DocumentFragment,
            "content must be an Element, DocumentFragment, or null",
        );
        validateCondition(
            options.icon === null ||
                options.icon === false ||
                options.icon instanceof Element,
            "icon must be an Element, false, or null",
        );
        for (const key of ["content", "icon"]) {
            const node = options[key];
            if (node)
                validateCondition(
                    node.ownerDocument === document && !node.isConnected,
                    key + " must be a detached node in this document",
                );
            if (node && options.target)
                validateCondition(
                    !node.contains(options.target),
                    key + " cannot contain the target",
                );
        }
        if (options.content && options.icon) {
            validateCondition(
                !options.content.contains(options.icon) &&
                    !options.icon.contains(options.content),
                "content and icon must be separate nodes",
            );
        }
        validateCondition(
            options.signal === null || options.signal instanceof AbortSignal,
            "signal must be an AbortSignal or null",
        );
        validateCondition(
            Array.isArray(options.actions),
            "actions must be an array",
        );
        const actions = options.actions.map((action) => {
            checkKeys(
                action,
                ["label", "onClick", "autoFocus", "disabled"],
                "action",
            );
            validateCondition(
                typeof action.label === "string" && action.label.length > 0,
                "action label must be a non-empty string",
            );
            validateCondition(
                typeof action.onClick === "function",
                "action onClick must be a function",
            );
            const result = { autoFocus: false, disabled: false, ...action };
            validateCondition(
                typeof result.autoFocus === "boolean" &&
                    typeof result.disabled === "boolean",
                "action autoFocus and disabled must be booleans",
            );
            return Object.freeze(result);
        });
        let overlay = null;
        if (options.overlay !== null) {
            checkKeys(options.overlay, ["color", "closeOnClick"], "overlay");
            const overlayOptions = {
                color: "rgba(0,0,0,0.6)",
                closeOnClick: false,
                ...options.overlay,
            };
            colorValue(overlayOptions.color, "overlay color");
            validateCondition(
                typeof overlayOptions.closeOnClick === "boolean",
                "overlay closeOnClick must be a boolean",
            );
            overlay = Object.freeze(overlayOptions);
        }
        return Object.freeze({ ...options, actions, overlay });
    }

    /**
     * @template {keyof HTMLElementTagNameMap} T
     * @param {T} tag
     * @param {string} className
     * @param {Partial<CSSStyleDeclaration>} [styles]
     * @returns {HTMLElementTagNameMap[T]}
     */
    function element(tag, className, styles = {}) {
        const node = document.createElement(tag);
        node.className = className;
        Object.assign(node.style, { boxSizing: "border-box", ...styles });
        return node;
    }

    function iconImage(name, color) {
        const svg =
            '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="' +
            color +
            '" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">' +
            IconPaths[name] +
            "</svg>";
        return 'url("data:image/svg+xml,' + encodeURIComponent(svg) + '")';
    }

    function effectivePosition(options) {
        if (!compactViewport.matches || options.position === "center")
            return options.position;
        return options.position.startsWith("top-")
            ? "top-center"
            : "bottom-center";
    }

    function styleWrapper(wrapper, position, zIndex) {
        const center = position === "center";
        Object.assign(wrapper.style, {
            position: "fixed",
            width: "100%",
            pointerEvents: "none",
            display: "flex",
            flexDirection: "column",
            padding: compactViewport.matches ? "0" : "10px 15px",
            top: center || position.startsWith("top-") ? "0" : "auto",
            bottom: center || position.startsWith("bottom-") ? "0" : "auto",
            left: center || !position.endsWith("-right") ? "0" : "auto",
            right: center || !position.endsWith("-left") ? "0" : "auto",
            textAlign: position.endsWith("-left")
                ? "left"
                : position.endsWith("-right")
                  ? "right"
                  : "center",
            justifyContent: center ? "center" : "normal",
            zIndex: String(zIndex),
        });
    }

    function removeEmptyWrappers() {
        for (const [position, wrapper] of wrappers) {
            if (!wrapper.children.length) {
                wrapper.remove();
                wrappers.delete(position);
            }
        }
    }

    function place(record) {
        const { options, dom } = record;
        let parent = options.target;
        if (!parent) {
            const position = effectivePosition(options);
            parent = wrappers.get(position);
            if (!parent) {
                parent = element(
                    "div",
                    "ember-wrapper ember-wrapper-" + position,
                );
                wrappers.set(position, parent);
                document.body.append(parent);
            }
            styleWrapper(parent, position, options.zIndex);
        }
        const first = options.target
            ? options.newestFirst
            : options.position.startsWith("top-");
        if (first) parent.prepend(dom.capsule);
        else parent.append(dom.capsule);
        Object.assign(dom.toast.style, {
            width: options.target || compactViewport.matches ? "100%" : "auto",
            margin: compactViewport.matches ? "0" : "5px 0",
            borderRadius: compactViewport.matches ? "0" : "3px",
            boxShadow: compactViewport.matches
                ? "none"
                : "0 8px 8px -5px rgba(0,0,0,0.25), inset 0 -10px 20px -10px rgba(0,0,0,0.2)",
        });
    }

    /**
     * @typedef {object} ToastElements
     * @property {HTMLDivElement} capsule
     * @property {HTMLDivElement} toast
     * @property {HTMLButtonElement | null} close
     * @property {HTMLButtonElement[]} actions
     * @property {HTMLDivElement | null} bar
     * @property {HTMLDivElement | null} overlay
     */

    function build(options) {
        const dark = options.theme === "dark";
        const foreground = dark ? "#fff" : "#000";
        /** @type {ToastElements} */
        const dom = {
            capsule: element("div", "ember-capsule", {
                display: "flow-root",
                width: "100%",
                fontSize: "0",
            }),
            toast: element("div", "ember", {
                display: "inline-block",
                position: "relative",
                pointerEvents: "auto",
                textAlign: "start",
                fontFamily: "Tahoma, Arial, sans-serif",
                fontSize: "14px",
                lineHeight: "1.3",
                minHeight: "54px",
                padding: "16px",
                paddingInlineEnd: options.closable ? "45px" : "16px",
                maxWidth:
                    typeof options.maxWidth === "number"
                        ? options.maxWidth + "px"
                        : options.maxWidth,
                color: foreground,
                background:
                    options.color ?? (dark ? "#565c70" : Colors[options.type]),
                overflowWrap: "anywhere",
                direction: options.direction,
            }),
            close: null,
            actions: [],
            bar: null,
            overlay: null,
        };
        dom.toast.setAttribute(
            "role",
            options.type === "error" ? "alert" : "status",
        );
        dom.toast.setAttribute("aria-atomic", "true");
        if (options.id !== null) dom.toast.dataset.emberId = options.id;
        dom.capsule.append(dom.toast);
        const body = element("div", "ember-body", {
            display: "flex",
            alignItems: "center",
            gap: "10px",
            minHeight: "22px",
        });
        dom.toast.append(body);
        if (
            options.icon !== false &&
            (options.icon || options.type !== "default")
        ) {
            const icon = element("span", "ember-icon", {
                display: "inline-flex",
                flexShrink: "0",
                width: "24px",
                height: "24px",
            });
            if (options.icon) icon.append(options.icon);
            else {
                Object.assign(icon.style, {
                    backgroundImage: iconImage(options.type, foreground),
                    backgroundSize: "85%",
                    backgroundPosition: "center",
                    backgroundRepeat: "no-repeat",
                });
                icon.setAttribute("aria-hidden", "true");
            }
            body.append(icon);
        }
        const contents = element("div", "ember-contents", {
            minWidth: "0",
            flex: "1",
        });
        body.append(contents);
        const texts = element("div", "ember-texts", {
            display: "flex",
            flexWrap: "wrap",
            columnGap: "10px",
            rowGap: "4px",
            flexDirection: options.layout === "stacked" ? "column" : "row",
        });
        contents.append(texts);
        if (options.title) {
            const title = element("strong", "ember-title", {
                margin: "0",
                padding: "0",
                font: "inherit",
                fontWeight: "700",
            });
            title.textContent = options.title;
            texts.append(title);
        }
        if (options.message) {
            const message = element("span", "ember-message", {
                margin: "0",
                padding: "0",
                color: dark ? "rgba(255,255,255,0.8)" : "rgba(0,0,0,0.65)",
            });
            message.textContent = options.message;
            texts.append(message);
        }
        if (options.content) {
            const content = element("div", "ember-content", {
                marginTop: options.title || options.message ? "8px" : "0",
            });
            content.append(options.content);
            contents.append(content);
        }
        if (options.actions.length) {
            const actions = element("div", "ember-actions", {
                display: "flex",
                flexWrap: "wrap",
                gap: "6px",
                marginTop: "8px",
            });
            for (const action of options.actions) {
                const button = element("button", "ember-action", {
                    font: "inherit",
                    fontSize: "12px",
                    padding: "5px 10px",
                    border: "0",
                    borderRadius: "2px",
                    background: dark
                        ? "rgba(255,255,255,0.15)"
                        : "rgba(0,0,0,0.1)",
                    color: "inherit",
                    cursor: action.disabled ? "default" : "pointer",
                    opacity: action.disabled ? "0.5" : "1",
                });
                button.type = "button";
                button.textContent = action.label;
                button.disabled = action.disabled;
                dom.actions.push(button);
                actions.append(button);
            }
            contents.append(actions);
        }
        if (options.closable) {
            dom.close = element("button", "ember-close", {
                position: "absolute",
                insetInlineEnd: "0",
                top: "0",
                width: "42px",
                height: "100%",
                border: "0",
                padding: "0",
                backgroundColor: "transparent",
                backgroundImage: iconImage("close", foreground),
                backgroundPosition: "center",
                backgroundRepeat: "no-repeat",
                backgroundSize: "20px",
                cursor: "pointer",
                opacity: "0.8",
            });
            dom.close.type = "button";
            dom.close.setAttribute("aria-label", "Close notification");
            dom.toast.append(dom.close);
        }
        if (options.progress && options.duration > 0) {
            const track = element("div", "ember-progressbar", {
                position: "absolute",
                left: "0",
                bottom: "0",
                width: "100%",
                background: "rgba(255,255,255,0.2)",
                overflow: "hidden",
            });
            dom.bar = element("div", "ember-progress", {
                height: "2px",
                width: "100%",
                background: "rgba(0,0,0,0.3)",
                transformOrigin: options.direction === "rtl" ? "right" : "left",
            });
            track.setAttribute("aria-hidden", "true");
            track.append(dom.bar);
            dom.toast.append(track);
        }
        if (options.overlay) {
            dom.overlay = element("div", "ember-overlay", {
                position: "fixed",
                inset: "0",
                background: options.overlay.color,
                zIndex: String(options.zIndex - 1),
            });
            dom.overlay.setAttribute("aria-hidden", "true");
        }
        return dom;
    }

    function motionFrames(options, opening) {
        const offset = options.position.startsWith("top-") ? -24 : 24;
        const end = { opacity: 1, transform: "none" };
        /** @type {Keyframe[]} */
        let frames;
        switch (options.animation) {
            case "fade":
                frames = [{ opacity: 0 }, { opacity: 1 }];
                break;
            case "flip":
                frames = [
                    {
                        opacity: 0,
                        transform: "perspective(400px) rotateX(75deg)",
                    },
                    end,
                ];
                break;
            case "bounce":
                frames = [
                    { opacity: 0, transform: `translateY(${offset}px)` },
                    {
                        offset: 0.65,
                        opacity: 1,
                        transform: `translateY(${-offset / 4}px)`,
                    },
                    end,
                ];
                break;
            default:
                frames = [
                    { opacity: 0, transform: `translateY(${offset}px)` },
                    end,
                ];
        }
        return opening
            ? frames
            : frames.toReversed().map(({ offset, ...frame }) => frame);
    }

    class Toast extends EventTarget {
        #options;
        #dom;
        #state = "opening";
        #controller = new AbortController();
        /** @type {PromiseWithResolvers<string>} */
        #completion = Promise.withResolvers();
        /** @type {Animation | null} */
        #motion = null;
        /** @type {Animation | null} */
        #progress = null;
        /** @type {number | null} */
        #timer = null;
        #remaining;
        #deadline = 0;
        #paused = false;

        constructor(options) {
            super();
            this.#options = options;
            this.#remaining = options.duration;
            this.#dom = build(
                options.signal?.aborted
                    ? { ...options, content: null, icon: false }
                    : options,
            );
            if (options.signal?.aborted) {
                this.#dispose("aborted");
                return;
            }
            const record = {
                toast: this,
                options,
                dom: this.#dom,
                dispose: (reason) => this.#dispose(reason),
            };
            records.set(this, record);
            this.#bind();
            if (this.#dom.overlay) document.body.append(this.#dom.overlay);
            place(record);
            queueMicrotask(() => this.#open());
        }

        get Element() {
            return this.#dom.toast;
        }
        get State() {
            return this.#state;
        }
        get Closed() {
            return this.#completion.promise;
        }

        /**
         * @param {string} type
         * @param {string | null} [reason]
         */
        #emit(type, reason = null) {
            this.dispatchEvent(
                new CustomEvent(type, { detail: { toast: this, reason } }),
            );
        }

        async #animate(opening) {
            const options = this.#options;
            if (
                options.animation === "none" ||
                !options.animationDuration ||
                reducedMotion.matches
            )
                return;
            const animation = this.Element.animate(
                motionFrames(options, opening),
                {
                    duration: options.animationDuration,
                    easing: "ease",
                    fill: "both",
                },
            );
            this.#motion = animation;
            try {
                await animation.finished;
            } catch (error) {
                if (
                    !(error instanceof DOMException) ||
                    error.name !== "AbortError"
                )
                    throw error;
            } finally {
                if (this.#motion === animation) this.#motion = null;
                animation.cancel();
            }
        }

        async #open() {
            if (this.#state !== "opening") return;
            this.#emit("opening");
            if (this.#state !== "opening") return;
            await this.#animate(true);
            if (this.#state !== "opening") return;
            this.#state = "open";
            this.#startTimer();
            const focus = this.#options.actions.findIndex(
                (action) => action.autoFocus && !action.disabled,
            );
            if (focus >= 0)
                this.#dom.actions[focus].focus({ preventScroll: true });
            this.#emit("opened");
        }

        Close(reason = "api") {
            validateCondition(
                typeof reason === "string" && reason.length > 0,
                "close reason must be a non-empty string",
            );
            if (this.#state === "closed" || this.#state === "closing")
                return this.#completion.promise;
            this.#state = "closing";
            this.#stopTimer();
            this.#controller.abort();
            this.#motion?.cancel();
            this.Element.style.pointerEvents = "none";
            this.#emit("closing", reason);
            if (this.#state !== "closed") this.#finishClose(reason);
            return this.#completion.promise;
        }

        async #finishClose(reason) {
            await this.#animate(false);
            this.#dispose(reason);
        }

        #dispose(reason) {
            if (this.#state === "closed") return;
            if (this.#state !== "closing") {
                this.#state = "closing";
                this.#emit("closing", reason);
                if (this.#state === "closed") return;
            }
            this.#stopTimer();
            this.#controller.abort();
            this.#motion?.cancel();
            this.#dom.capsule.remove();
            this.#dom.overlay?.remove();
            records.delete(this);
            removeEmptyWrappers();
            this.#state = "closed";
            this.#completion.resolve(reason);
            this.#emit("closed", reason);
        }

        #stopTimer() {
            if (this.#timer !== null) {
                clearTimeout(this.#timer);
                this.#timer = null;
                this.#remaining = Math.max(
                    0,
                    this.#deadline - performance.now(),
                );
            }
            this.#progress?.cancel();
            this.#progress = null;
            if (this.#dom.bar)
                this.#dom.bar.style.transform = `scaleX(${this.#remaining / this.#options.duration})`;
        }

        #startTimer() {
            if (
                this.#state !== "open" ||
                this.#paused ||
                !this.#options.duration ||
                this.#timer !== null
            )
                return;
            this.#deadline = performance.now() + this.#remaining;
            if (this.#dom.bar) {
                this.#progress = this.#dom.bar.animate(
                    [
                        {
                            transform: `scaleX(${this.#remaining / this.#options.duration})`,
                        },
                        { transform: "scaleX(0)" },
                    ],
                    {
                        duration: this.#remaining,
                        easing: "linear",
                        fill: "forwards",
                    },
                );
            }
            this.#timer = setTimeout(() => {
                this.#timer = null;
                this.#remaining = 0;
                this.Close("timeout");
            }, this.#remaining);
        }

        Pause() {
            if (this.#state === "closed" || this.#state === "closing")
                return this;
            this.#paused = true;
            this.#stopTimer();
            return this;
        }

        Resume() {
            if (this.#state === "closed" || this.#state === "closing")
                return this;
            this.#paused = false;
            this.#startTimer();
            return this;
        }

        Restart() {
            if (this.#state === "closed" || this.#state === "closing")
                return this;
            this.#stopTimer();
            this.#remaining = this.#options.duration;
            this.#paused = false;
            if (this.#dom.bar) this.#dom.bar.style.transform = "scaleX(1)";
            this.#startTimer();
            return this;
        }

        #bind() {
            const options = this.#options;
            const signal = this.#controller.signal;
            const dom = this.#dom;
            const on = (node, type, listener) =>
                node.addEventListener(type, listener, { signal });
            if (options.signal)
                on(options.signal, "abort", () => this.Close("aborted"));
            const close = dom.close;
            if (close) {
                on(close, "click", (event) => {
                    event.stopPropagation();
                    this.Close("button");
                });
                on(close, "mouseenter", () => {
                    close.style.opacity = "1";
                });
                on(close, "mouseleave", () => {
                    close.style.opacity = "0.8";
                });
            }
            if (options.closeOnEscape)
                on(document, "keydown", (event) => {
                    if (event.key === "Escape") this.Close("escape");
                });
            if (options.closeOnClick)
                on(dom.toast, "click", (event) => {
                    if (
                        !event.target.closest(
                            "button, a, input, select, textarea, [contenteditable]",
                        )
                    )
                        this.Close("click");
                });
            if (options.overlay?.closeOnClick)
                on(dom.overlay, "click", () => this.Close("overlay"));
            if (options.hover !== "none") {
                on(dom.toast, "mouseenter", () => {
                    if (options.hover === "restart") this.Restart();
                    this.Pause();
                });
                on(dom.toast, "mouseleave", () => this.Resume());
            }
            options.actions.forEach((action, index) => {
                const button = dom.actions[index];
                on(button, "click", (event) => {
                    event.stopPropagation();
                    action.onClick({ toast: this, event });
                });
            });
            if (options.draggable) this.#bindDrag(on);
        }

        #bindDrag(on) {
            const toast = this.Element;
            let start = null;
            toast.style.touchAction = "pan-y";
            on(toast, "pointerdown", (event) => {
                if (
                    !event.isPrimary ||
                    event.button !== 0 ||
                    event.target.closest(
                        "button, a, input, select, textarea, [contenteditable]",
                    )
                )
                    return;
                start = event.clientX;
                toast.setPointerCapture(event.pointerId);
            });
            on(toast, "pointermove", (event) => {
                if (start === null) return;
                const distance = event.clientX - start;
                toast.style.transform = `translateX(${distance}px)`;
                toast.style.opacity = String(
                    Math.max(0, 1 - Math.abs(distance) / 180),
                );
                if (Math.abs(distance) > 126) {
                    start = null;
                    this.Close("drag");
                }
            });
            const stop = () => {
                if (start === null) return;
                start = null;
                toast.style.transform = "";
                toast.style.opacity = "";
            };
            on(toast, "pointerup", stop);
            on(toast, "pointercancel", stop);
            on(toast, "lostpointercapture", stop);
        }
    }

    function Show(input = {}) {
        const options = normalize(input);
        validateCondition(
            document.body !== null,
            "Show requires document.body",
        );
        if (
            options.id !== null &&
            options.duplicate !== "stack" &&
            !options.signal?.aborted
        ) {
            const matches = [...records.values()].filter(
                (record) =>
                    record.options.id === options.id &&
                    (record.toast.State === "opening" ||
                        record.toast.State === "open"),
            );
            if (options.duplicate === "ignore" && matches.length)
                return matches[0].toast;
            for (const record of matches) record.toast.Close("replaced");
        }
        return new Toast(options);
    }

    function typedShow(type, options) {
        checkKeys(options, Object.keys(defaults), "toast");
        return Show({ ...options, type });
    }

    compactViewport.addEventListener("change", () => {
        for (const record of records.values()) place(record);
        removeEmptyWrappers();
    });

    return Object.freeze({
        Show,
        Info: (options = {}) => typedShow("info", options),
        Success: (options = {}) => typedShow("success", options),
        Warning: (options = {}) => typedShow("warning", options),
        Error: (options = {}) => typedShow("error", options),
        Question: (options = {}) => typedShow("question", options),
        Configure(options = {}) {
            checkKeys(
                options,
                Object.keys(defaults).filter((key) => !localOptions.has(key)),
                "default",
            );
            const validated = normalize(options);
            configuration = Object.fromEntries(
                Object.entries(validated).filter(
                    ([key]) => !localOptions.has(key),
                ),
            );
        },
        Clear() {
            return Promise.all(
                [...records.keys()].map((toast) => toast.Close("clear")),
            );
        },
        Destroy() {
            configuration = {};
            for (const record of [...records.values()])
                record.dispose("destroy");
        },
    });
})();
