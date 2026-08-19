"""Stable browser bridge for Brahma Echo.

Browser websites and their follow-up interactions must share one browser
session. PinchTab is therefore selected automatically for browser automation;
non-browser desktop controls remain handled by the existing sitecustomize
hooks.
"""
from __future__ import annotations

import importlib
import os

# Do not require the user to export an environment variable manually.
os.environ["BRAHMA_BROWSER_BACKEND"] = "pinchtab"


def _activate_pinchtab_browser() -> None:
    try:
        import actions.browser_control as module
        module = importlib.reload(module)
        print("[BrowserBridge] PinchTab browser automation ACTIVE.")
    except Exception as exc:
        print(f"[BrowserBridge] PinchTab activation failed: {exc}")


def _patch_open_app_websites() -> None:
    try:
        import actions.open_app as module
        original = module.open_app
    except Exception:
        return

    if getattr(module, "_BRAHMA_PINCHTAB_OPEN_PATCH", False):
        return

    website_map = {
        "youtube": "https://www.youtube.com",
        "google": "https://www.google.com",
        "gmail": "https://mail.google.com",
        "openai": "https://openai.com",
        "chatgpt": "https://chatgpt.com",
        "github": "https://github.com",
        "instagram": "https://www.instagram.com",
        "facebook": "https://www.facebook.com",
        "linkedin": "https://www.linkedin.com",
        "reddit": "https://www.reddit.com",
        "amazon": "https://www.amazon.in",
        "flipkart": "https://www.flipkart.com",
        "twitter": "https://x.com",
        "x": "https://x.com",
    }

    def patched_open_app(parameters=None, response=None, player=None, session_memory=None):
        params = dict(parameters or {})
        name = str(params.get("app_name") or "").strip().lower()
        url = website_map.get(name)
        if url:
            try:
                from actions.browser_control import browser_control
                result = browser_control(
                    {"action": "go_to", "url": url},
                    response=response,
                    player=player,
                    session_memory=session_memory,
                )
                print(f"[BrowserBridge] Website opened through PinchTab: {url}")
                return result
            except Exception as exc:
                print(f"[BrowserBridge] Website routing failed: {exc}")
        return original(parameters=parameters, response=response, player=player, session_memory=session_memory)

    module.open_app = patched_open_app
    module._BRAHMA_PINCHTAB_OPEN_PATCH = True


_activate_pinchtab_browser()
_patch_open_app_websites()
