"""Brahma Echo adapter for a local PinchTab HTTP server.

This adapter keeps the PinchTab runtime isolated from the Python app. It can
start the bundled Go server when its binary exists, then exposes a small set of
high-value browser operations for Brahma Echo.
"""
from __future__ import annotations

import json
import os
import subprocess
import time
from pathlib import Path
from typing import Any

import requests


BASE_DIR = Path(__file__).resolve().parent.parent
PINCHTAB_ROOT = BASE_DIR / "vendor" / "pinchtab"
PINCHTAB_BIN = PINCHTAB_ROOT / "bin" / ("pinchtab.exe" if os.name == "nt" else "pinchtab")
PINCHTAB_URL = os.environ.get("PINCHTAB_URL", "http://127.0.0.1:9867").rstrip("/")


def _load_config_token() -> str:
    """Load PinchTab's generated server token when env var is not set.

    PinchTab 0.8+ generates a token in %APPDATA%/pinchtab/config.json.
    The Python adapter must use that same token or every authenticated
    request appears as a false 'server not running' (HTTP 401).
    """
    env_token = os.environ.get("PINCHTAB_TOKEN", "").strip()
    if env_token:
        return env_token

    if os.name == "nt":
        config_path = Path(os.environ.get("APPDATA", "")) / "pinchtab" / "config.json"
    else:
        config_path = Path.home() / ".config" / "pinchtab" / "config.json"

    try:
        data = json.loads(config_path.read_text(encoding="utf-8"))
        return str((data.get("server") or {}).get("token") or "").strip()
    except Exception:
        return ""


PINCHTAB_TOKEN = _load_config_token()


def _headers() -> dict[str, str]:
    h = {"Content-Type": "application/json"}
    if PINCHTAB_TOKEN:
        h["Authorization"] = f"Bearer {PINCHTAB_TOKEN}"
    return h


def _request(method: str, path: str, **kwargs: Any) -> requests.Response:
    timeout = kwargs.pop("timeout", 30)
    return requests.request(
        method,
        PINCHTAB_URL + path,
        headers=_headers(),
        timeout=timeout,
        **kwargs,
    )


def health() -> dict[str, Any]:
    r = _request("GET", "/health", timeout=5)
    r.raise_for_status()
    return r.json()


def is_running() -> bool:
    try:
        return health() is not None
    except requests.HTTPError as exc:
        # 401 means the server is reachable but credentials are wrong.
        # It must not trigger another PinchTab server launch.
        if exc.response is not None and exc.response.status_code == 401:
            raise RuntimeError(
                "PinchTab server is reachable but authentication failed. "
                "Refresh PINCHTAB_TOKEN from %APPDATA%\\pinchtab\\config.json."
            ) from exc
        return False
    except Exception:
        return False


def start_server(wait_seconds: float = 15.0) -> None:
    try:
        health()
        return
    except requests.HTTPError as exc:
        if exc.response is not None and exc.response.status_code == 401:
            # The server is already running; do not spawn duplicate servers.
            global PINCHTAB_TOKEN
            PINCHTAB_TOKEN = _load_config_token()
            health()
            return
    except Exception:
        pass

    if not PINCHTAB_BIN.exists():
        raise FileNotFoundError(
            f"PinchTab binary not found at {PINCHTAB_BIN}. Run scripts\\setup_pinchtab.ps1 first."
        )

    creationflags = 0
    if os.name == "nt":
        creationflags = subprocess.CREATE_NEW_PROCESS_GROUP | subprocess.CREATE_NO_WINDOW

    subprocess.Popen(
        [str(PINCHTAB_BIN), "server"],
        cwd=str(PINCHTAB_ROOT),
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        creationflags=creationflags,
    )

    deadline = time.time() + wait_seconds
    last_error: Exception | None = None

    while time.time() < deadline:
        # Refresh token on every retry because PinchTab may generate it at first startup.
        global PINCHTAB_TOKEN
        PINCHTAB_TOKEN = _load_config_token()
        try:
            health()
            return
        except Exception as exc:
            last_error = exc
            time.sleep(0.4)

    raise RuntimeError(f"PinchTab server did not become ready: {last_error}")


def _ensure_server() -> None:
    try:
        health()
        return
    except requests.HTTPError as exc:
        if exc.response is not None and exc.response.status_code == 401:
            global PINCHTAB_TOKEN
            PINCHTAB_TOKEN = _load_config_token()
            health()
            return
        raise
    except Exception:
        start_server()


def navigate(url: str, tab_id: str | None = None, new_tab: bool = False) -> dict[str, Any]:
    _ensure_server()
    payload: dict[str, Any] = {"url": url}
    if tab_id:
        payload["tabId"] = tab_id
    if new_tab:
        payload["newTab"] = True
    r = _request("POST", "/navigate", json=payload)
    r.raise_for_status()
    return r.json() if r.content else {}


def tabs() -> list[dict[str, Any]]:
    _ensure_server()
    r = _request("GET", "/tabs")
    r.raise_for_status()
    data = r.json()
    return data if isinstance(data, list) else data.get("tabs", [])


def snapshot(tab_id: str | None = None, interactive: bool = True, compact: bool = True) -> Any:
    _ensure_server()
    path = "/snapshot"
    if tab_id:
        path = f"/tabs/{tab_id}/snapshot"
    params = {
        "interactive": str(interactive).lower(),
        "compact": str(compact).lower(),
    }
    r = _request("GET", path, params=params)
    r.raise_for_status()
    try:
        return r.json()
    except ValueError:
        return r.text


def text(tab_id: str | None = None, mode: str = "readability") -> str:
    _ensure_server()
    path = "/text" if not tab_id else f"/tabs/{tab_id}/text"
    r = _request("GET", path, params={"mode": mode})
    r.raise_for_status()
    try:
        data = r.json()
        return data.get("text", json.dumps(data, ensure_ascii=False)) if isinstance(data, dict) else str(data)
    except ValueError:
        return r.text


def action(tab_id: str, kind: str, **params: Any) -> Any:
    _ensure_server()
    payload = {"kind": kind, **params}
    r = _request("POST", f"/tabs/{tab_id}/action", json=payload)
    r.raise_for_status()
    try:
        return r.json()
    except ValueError:
        return r.text


def click(tab_id: str, ref: str) -> Any:
    return action(tab_id, "click", ref=ref)


def fill(tab_id: str, selector: str, text_value: str) -> Any:
    return action(tab_id, "fill", selector=selector, text=text_value)


def press(tab_id: str, key: str) -> Any:
    return action(tab_id, "press", key=key)


def screenshot(tab_id: str | None = None, output: Path | None = None) -> bytes:
    _ensure_server()
    path = "/screenshot" if not tab_id else f"/tabs/{tab_id}/screenshot"
    r = _request("GET", path, params={"raw": "true"})
    r.raise_for_status()
    data = r.content
    if output:
        output.parent.mkdir(parents=True, exist_ok=True)
        output.write_bytes(data)
    return data


def browser_control(parameters: dict[str, Any]) -> str:
    """Natural wrapper for Brahma's browser tool dispatch."""
    action_name = str(parameters.get("action", "")).lower().strip()
    tab_id = parameters.get("tab_id") or parameters.get("tabId")

    if action_name in {"go_to", "navigate"}:
        return json.dumps(
            navigate(str(parameters.get("url", "")), tab_id=tab_id),
            ensure_ascii=False,
        )
    if action_name in {"tabs", "list_tabs"}:
        return json.dumps(tabs(), ensure_ascii=False)
    if action_name == "snapshot":
        return json.dumps(snapshot(tab_id), ensure_ascii=False)
    if action_name == "get_text":
        return text(tab_id)
    if action_name == "click":
        return json.dumps(
            click(str(tab_id), str(parameters.get("ref", ""))),
            ensure_ascii=False,
        )
    if action_name == "fill":
        return json.dumps(
            fill(
                str(tab_id),
                str(parameters.get("selector", "")),
                str(parameters.get("text", "")),
            ),
            ensure_ascii=False,
        )
    if action_name == "press":
        return json.dumps(
            press(str(tab_id), str(parameters.get("key", "Enter"))),
            ensure_ascii=False,
        )
    if action_name == "screenshot":
        out = parameters.get("output")
        screenshot(tab_id, Path(out) if out else None)
        return f"Screenshot saved: {out}" if out else "Screenshot captured."
    if action_name == "server_start":
        start_server()
        return "PinchTab server is ready."
    if action_name == "health":
        return json.dumps(health(), ensure_ascii=False)
    return f"Unknown PinchTab action: {action_name}"
