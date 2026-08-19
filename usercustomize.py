"""Deterministic browser-session bridge loaded by Python's usercustomize hook.

Goals:
- Keep website opening and later browser interactions in the same Brahma browser
  controller/session.
- Normalize browser-control arguments so natural-language descriptions work for
  click/type/fill actions.
- Recover browser actions when the model accidentally selects computer_control
  for an obvious webpage element interaction.

This module is intentionally small and leaves normal desktop controls untouched.
"""
from __future__ import annotations

import re
from typing import Any


_WEBSITE_URLS = {
    "youtube": "https://www.youtube.com",
    "youtube.com": "https://www.youtube.com",
    "google": "https://www.google.com",
    "google.com": "https://www.google.com",
    "gmail": "https://mail.google.com",
    "gmail.com": "https://mail.google.com",
    "openai": "https://openai.com",
    "openai.com": "https://openai.com",
    "chatgpt": "https://chatgpt.com",
    "chatgpt.com": "https://chatgpt.com",
    "github": "https://github.com",
    "github.com": "https://github.com",
    "openrouter": "https://openrouter.ai",
    "openrouter.ai": "https://openrouter.ai",
    "instagram": "https://www.instagram.com",
    "instagram.com": "https://www.instagram.com",
    "facebook": "https://www.facebook.com",
    "facebook.com": "https://www.facebook.com",
    "linkedin": "https://www.linkedin.com",
    "linkedin.com": "https://www.linkedin.com",
    "reddit": "https://www.reddit.com",
    "reddit.com": "https://www.reddit.com",
    "amazon": "https://www.amazon.in",
    "amazon.in": "https://www.amazon.in",
    "flipkart": "https://www.flipkart.com",
    "flipkart.com": "https://www.flipkart.com",
    "twitter": "https://x.com",
    "twitter.com": "https://x.com",
    "x.com": "https://x.com",
}

_BROWSER_ACTIVE = False


def _website_url(value: Any) -> str | None:
    text = str(value or "").strip()
    if not text:
        return None
    key = text.lower()
    if key in _WEBSITE_URLS:
        return _WEBSITE_URLS[key]
    if key.startswith(("http://", "https://")):
        return key
    if re.fullmatch(r"[A-Za-z0-9.-]+\.[A-Za-z]{2,}(/.*)?", key):
        return "https://" + key
    return None


def _mark_browser_active() -> None:
    global _BROWSER_ACTIVE
    _BROWSER_ACTIVE = True


def _looks_like_web_element(parameters: dict[str, Any]) -> bool:
    if parameters.get("x") is not None or parameters.get("y") is not None:
        return False
    description = str(
        parameters.get("description")
        or parameters.get("text")
        or ""
    ).lower()
    if not description:
        return False
    browser_terms = (
        "search box",
        "search field",
        "search bar",
        "email field",
        "password field",
        "username field",
        "sign in",
        "log in",
        "login",
        "button",
        "link",
        "first result",
        "second result",
        "result",
        "textbox",
        "input",
        "form",
        "website",
        "webpage",
        "page",
        "browser",
        "address bar",
    )
    return any(term in description for term in browser_terms)


def _patch_open_app() -> None:
    try:
        import actions.open_app as module
        original = module.open_app
    except Exception as exc:
        print(f"[BrowserBridge] open_app patch skipped: {exc}")
        return

    if getattr(module, "_BRAHMA_BROWSER_BRIDGE", False):
        return

    def browser_aware_open_app(parameters=None, response=None, player=None, session_memory=None):
        params = dict(parameters or {})
        app_name = str(params.get("app_name") or "").strip()
        url = _website_url(app_name)
        if url:
            try:
                from actions.browser_control import browser_control
                _mark_browser_active()
                result = browser_control(
                    {
                        "action": "go_to",
                        "url": url,
                    },
                    response=response,
                    player=player,
                    session_memory=session_memory,
                )
                print(f"[BrowserBridge] Opened website in shared browser: {url}")
                return result or f"Opened {url} in the browser."
            except Exception as exc:
                print(f"[BrowserBridge] Website open failed: {exc}")
        return original(parameters=parameters, response=response, player=player, session_memory=session_memory)

    module.open_app = browser_aware_open_app
    module._BRAHMA_BROWSER_BRIDGE = True


def _patch_browser_control() -> None:
    try:
        import actions.browser_control as module
        original = module.browser_control
    except Exception as exc:
        print(f"[BrowserBridge] browser_control patch skipped: {exc}")
        return

    if getattr(module, "_BRAHMA_BROWSER_BRIDGE", False):
        return

    def browser_aware_control(parameters=None, response=None, player=None, session_memory=None):
        global _BROWSER_ACTIVE
        params = dict(parameters or {})
        action = str(params.get("action") or "").strip().lower()

        if action in {"go_to", "navigate", "search", "click", "type", "fill_form", "smart_click", "smart_type", "press", "get_text", "list_tabs", "switch_tab"}:
            _BROWSER_ACTIVE = True

        # Normalize common model outputs into the browser controller's actual
        # smart actions. The base browser implementation already supports these.
        if action == "click" and not params.get("selector") and not params.get("text"):
            if params.get("description"):
                params["action"] = "smart_click"

        elif action == "type" and not params.get("selector"):
            if params.get("description"):
                params["action"] = "smart_type"

        elif action == "fill_form" and not params.get("fields"):
            if params.get("description") and params.get("text"):
                params["action"] = "smart_type"

        return original(
            parameters=params,
            response=response,
            player=player,
            session_memory=session_memory,
        )

    module.browser_control = browser_aware_control
    module._BRAHMA_BROWSER_BRIDGE = True


def _patch_computer_control() -> None:
    try:
        import actions.computer_control as module
        original = module.computer_control
    except Exception as exc:
        print(f"[BrowserBridge] computer_control patch skipped: {exc}")
        return

    if getattr(module, "_BRAHMA_BROWSER_BRIDGE", False):
        return

    def browser_aware_computer_control(parameters=None, response=None, player=None, session_memory=None):
        params = dict(parameters or {})
        action = str(params.get("action") or "").strip().lower()

        browser_actions = {
            "click": "smart_click",
            "smart_click": "smart_click",
            "type": "smart_type",
            "smart_type": "smart_type",
        }

        # If a browser session is active and the model selected the generic
        # computer-control tool for a webpage element, redirect it to the
        # shared browser controller. Coordinate-based desktop clicks remain
        # untouched.
        if _BROWSER_ACTIVE and action in browser_actions and _looks_like_web_element(params):
            try:
                from actions.browser_control import browser_control
                browser_action = browser_actions[action]
                browser_params = {
                    "action": browser_action,
                    "description": params.get("description") or params.get("text") or "",
                }
                if browser_action == "smart_type":
                    browser_params["text"] = str(params.get("text") or "")
                print(f"[BrowserBridge] Redirected computer_control -> browser_control ({browser_action})")
                return browser_control(
                    browser_params,
                    response=response,
                    player=player,
                    session_memory=session_memory,
                )
            except Exception as exc:
                print(f"[BrowserBridge] Browser redirect failed: {exc}")

        if _BROWSER_ACTIVE and action == "press" and params.get("key"):
            try:
                from actions.browser_control import browser_control
                print("[BrowserBridge] Redirected browser key press.")
                return browser_control(
                    {"action": "press", "key": params["key"]},
                    response=response,
                    player=player,
                    session_memory=session_memory,
                )
            except Exception as exc:
                print(f"[BrowserBridge] Browser press redirect failed: {exc}")

        return original(
            parameters=parameters,
            response=response,
            player=player,
            session_memory=session_memory,
        )

    module.computer_control = browser_aware_computer_control
    module._BRAHMA_BROWSER_BRIDGE = True


_patch_open_app()
_patch_browser_control()
_patch_computer_control()
print("[BrowserBridge] Shared browser interaction bridge ACTIVE.")
