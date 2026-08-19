"""Stable browser bridge loaded by Python's usercustomize hook.

The key rule is that opening a website and interacting with that website must
use the SAME actions.browser_control implementation/session.  We therefore
restore the real browser controller after sitecustomize's desktop routing
hooks have loaded, then add only small argument/command normalisation around it.
"""
from __future__ import annotations

import importlib
import os
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


def _looks_like_web_element(parameters: dict[str, Any]) -> bool:
    if parameters.get("x") is not None or parameters.get("y") is not None:
        return False

    description = str(
        parameters.get("description")
        or parameters.get("text")
        or ""
    ).lower()

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
    return bool(description) and any(term in description for term in browser_terms)


def _restore_real_browser_controller():
    """Undo sitecustomize's system-browser shim for browser_control only."""
    try:
        import actions.browser_control as module
        return importlib.reload(module)
    except Exception as exc:
        print(f"[BrowserBridge] Could not restore browser controller: {exc}")
        return None


def _pinchtab_result(parameters, response, player, session_memory):
    """Use PinchTab only when explicitly selected by BRAHMA_BROWSER_BACKEND."""
    try:
        from actions import pinchtab_client as pt
        action = str(parameters.get("action") or "").strip().lower()

        supported = {
            "go_to", "navigate", "tabs", "list_tabs", "snapshot", "get_text",
            "click", "fill", "press", "screenshot", "server_start", "health"
        }
        if action not in supported:
            return None

        # PinchTab's click/fill API needs a tab id/ref or selector. Natural-language
        # descriptions are handled by the normal browser controller instead.
        if action == "click" and not (parameters.get("tab_id") or parameters.get("tabId")):
            return None
        if action == "fill" and not (parameters.get("tab_id") or parameters.get("tabId")):
            return None

        result = pt.browser_control(parameters)
        if player:
            player.write_log(f"[PinchTab] {action}")
        else:
            print(f"[PinchTab] {action}")
        return result
    except Exception as exc:
        msg = f"[PinchTab] {action} unavailable: {exc}; using normal browser controller."
        if player:
            player.write_log(msg)
        else:
            print(msg)
        return None


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
                global _BROWSER_ACTIVE
                _BROWSER_ACTIVE = True
                result = browser_control(
                    {"action": "go_to", "url": url},
                    response=response,
                    player=player,
                    session_memory=session_memory,
                )
                print(f"[BrowserBridge] Shared browser opened: {url}")
                return result or f"Opened {url} in the browser."
            except Exception as exc:
                print(f"[BrowserBridge] Website open failed: {exc}")

        return original(
            parameters=parameters,
            response=response,
            player=player,
            session_memory=session_memory,
        )

    module.open_app = browser_aware_open_app
    module._BRAHMA_BROWSER_BRIDGE = True


def _patch_browser_control() -> None:
    global _BROWSER_ACTIVE

    module = _restore_real_browser_controller()
    if module is None:
        return

    original = module.browser_control

    def browser_aware_control(parameters=None, response=None, player=None, session_memory=None):
        global _BROWSER_ACTIVE
        params = dict(parameters or {})
        action = str(params.get("action") or "").strip().lower()

        if action in {
            "go_to", "navigate", "search", "click", "type", "fill_form",
            "smart_click", "smart_type", "press", "get_text", "list_tabs",
            "switch_tab", "back", "forward", "refresh", "reload"
        }:
            _BROWSER_ACTIVE = True

        # Explicit PinchTab mode is preserved, but normal natural-language
        # interaction remains on the same persistent browser session.
        if os.environ.get("BRAHMA_BROWSER_BACKEND", "system").strip().lower() == "pinchtab":
            result = _pinchtab_result(params, response, player, session_memory)
            if result is not None:
                return result

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

        if _BROWSER_ACTIVE and action in {"click", "smart_click"} and _looks_like_web_element(params):
            try:
                from actions.browser_control import browser_control
                return browser_control(
                    {
                        "action": "smart_click",
                        "description": params.get("description") or params.get("text") or "",
                    },
                    response=response,
                    player=player,
                    session_memory=session_memory,
                )
            except Exception as exc:
                print(f"[BrowserBridge] computer click redirect failed: {exc}")

        if _BROWSER_ACTIVE and action in {"type", "smart_type"} and _looks_like_web_element(params):
            try:
                from actions.browser_control import browser_control
                return browser_control(
                    {
                        "action": "smart_type",
                        "description": params.get("description") or params.get("text") or "",
                        "text": str(params.get("text") or ""),
                    },
                    response=response,
                    player=player,
                    session_memory=session_memory,
                )
            except Exception as exc:
                print(f"[BrowserBridge] computer type redirect failed: {exc}")

        if _BROWSER_ACTIVE and action == "press" and params.get("key"):
            try:
                from actions.browser_control import browser_control
                return browser_control(
                    {"action": "press", "key": params["key"]},
                    response=response,
                    player=player,
                    session_memory=session_memory,
                )
            except Exception as exc:
                print(f"[BrowserBridge] browser press redirect failed: {exc}")

        return original(
            parameters=parameters,
            response=response,
            player=player,
            session_memory=session_memory,
        )

    module.computer_control = browser_aware_computer_control
    module._BRAHMA_BROWSER_BRIDGE = True


_patch_browser_control()
_patch_open_app()
_patch_computer_control()
print("[BrowserBridge] Shared browser session ACTIVE.")
