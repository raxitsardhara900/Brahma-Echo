"""PinchTab-first browser router with the existing Playwright controller as fallback."""
from __future__ import annotations

import importlib.util
import json
import os
from pathlib import Path
from typing import Any

from . import pinchtab_client


_BASE_DIR = Path(__file__).resolve().parent
_LEGACY_PATH = _BASE_DIR / "browser_control.py"


def _load_legacy():
    spec = importlib.util.spec_from_file_location("actions._browser_control_legacy", _LEGACY_PATH)
    if spec is None or spec.loader is None:
        raise ImportError(f"Cannot load legacy browser controller: {_LEGACY_PATH}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


_LEGACY = None


def _legacy_module():
    global _LEGACY
    if _LEGACY is None:
        _LEGACY = _load_legacy()
    return _LEGACY


def _pinchtab_enabled() -> bool:
    return os.environ.get("BRAHMA_PINCHTAB", "1").strip().lower() not in {"0", "false", "off", "no"}


def _fallback(parameters: dict[str, Any], response=None, player=None, session_memory=None) -> str:
    return _legacy_module().browser_control(
        parameters,
        response=response,
        player=player,
        session_memory=session_memory,
    )


def _active_tab_id() -> str | None:
    try:
        tabs = pinchtab_client.tabs()
        if not tabs:
            return None
        for tab in tabs:
            if tab.get("active") or tab.get("selected"):
                return str(tab.get("id") or tab.get("tabId") or "") or None
        first = tabs[0]
        return str(first.get("id") or first.get("tabId") or "") or None
    except Exception:
        return None


def browser_control(parameters: dict[str, Any], response=None, player=None, session_memory=None) -> str:
    """Use PinchTab for high-confidence operations; preserve Playwright fallback."""
    params = dict(parameters or {})
    action = str(params.get("action", "")).lower().strip()

    if not _pinchtab_enabled():
        return _fallback(params, response, player, session_memory)

    try:
        tab_id = params.get("tab_id") or params.get("tabId") or _active_tab_id()

        if action in {"server_start", "pinchtab_start"}:
            return pinchtab_client.browser_control({"action": "server_start"})

        if action in {"health", "pinchtab_health"}:
            return pinchtab_client.browser_control({"action": "health"})

        if action in {"go_to", "navigate"}:
            url = str(params.get("url", "")).strip()
            if not url:
                return "No URL provided."
            result = pinchtab_client.navigate(url, tab_id=tab_id, new_tab=bool(params.get("new_tab") or params.get("newTab")))
            return json.dumps(result, ensure_ascii=False)

        if action in {"tabs", "list_tabs"}:
            return json.dumps(pinchtab_client.tabs(), ensure_ascii=False)

        if action == "snapshot":
            return json.dumps(pinchtab_client.snapshot(tab_id), ensure_ascii=False)

        if action == "get_text":
            return pinchtab_client.text(tab_id)

        if action == "click" and tab_id:
            ref = str(params.get("ref", "")).strip()
            if ref:
                return json.dumps(pinchtab_client.click(str(tab_id), ref), ensure_ascii=False)
            # No stable PinchTab ref was supplied; let Playwright handle selector/text clicks.
            return _fallback(params, response, player, session_memory)

        if action == "fill" and tab_id:
            selector = str(params.get("selector", "")).strip()
            text = str(params.get("text", ""))
            if selector:
                return json.dumps(pinchtab_client.fill(str(tab_id), selector, text), ensure_ascii=False)
            return _fallback(params, response, player, session_memory)

        if action == "press" and tab_id:
            key = str(params.get("key", "Enter"))
            return json.dumps(pinchtab_client.press(str(tab_id), key), ensure_ascii=False)

        if action == "screenshot":
            output = params.get("output")
            return pinchtab_client.browser_control({
                "action": "screenshot",
                "tab_id": tab_id,
                "output": output,
            })

        # PinchTab adapter deliberately handles only operations with verified mappings.
        # Existing Playwright remains the compatibility path for richer/specialized actions.
        return _fallback(params, response, player, session_memory)

    except Exception as exc:
        print(f"[BrowserRouter] PinchTab failed: {exc}; using Playwright fallback")
        return _fallback(params, response, player, session_memory)
