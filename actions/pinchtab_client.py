"""Brahma Echo adapter for a local PinchTab HTTP server."""
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


def _refresh_token() -> str:
    global PINCHTAB_TOKEN
    PINCHTAB_TOKEN = _load_config_token()
    return PINCHTAB_TOKEN


def _headers() -> dict[str, str]:
    h = {"Content-Type": "application/json"}
    if PINCHTAB_TOKEN:
        h["Authorization"] = f"Bearer {PINCHTAB_TOKEN}"
    return h


def _request(method: str, path: str, **kwargs: Any) -> requests.Response:
    timeout = kwargs.pop("timeout", 30)
    return requests.request(method, PINCHTAB_URL + path, headers=_headers(), timeout=timeout, **kwargs)


def _request_json(method: str, path: str, **kwargs: Any) -> Any:
    r = _request(method, path, **kwargs)
    if r.status_code == 401:
        _refresh_token()
        r = _request(method, path, **kwargs)
    r.raise_for_status()
    try:
        return r.json()
    except ValueError:
        return r.text


def health() -> dict[str, Any]:
    data = _request_json("GET", "/health", timeout=5)
    return data if isinstance(data, dict) else {"result": data}


def is_running() -> bool:
    try:
        health()
        return True
    except Exception:
        return False


def start_server(wait_seconds: float = 15.0) -> None:
    try:
        health()
        return
    except requests.HTTPError as exc:
        if exc.response is not None and exc.response.status_code == 401:
            _refresh_token()
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
        _refresh_token()
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
    except requests.HTTPError as exc:
        if exc.response is not None and exc.response.status_code == 401:
            _refresh_token()
            health()
        else:
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
    data = _request_json("POST", "/navigate", json=payload)
    return data if isinstance(data, dict) else {"result": data}


def search(query: str, engine: str = "google", tab_id: str | None = None) -> dict[str, Any]:
    from urllib.parse import quote_plus
    engines = {
        "google": f"https://www.google.com/search?q={quote_plus(query)}",
        "bing": f"https://www.bing.com/search?q={quote_plus(query)}",
        "duckduckgo": f"https://duckduckgo.com/?q={quote_plus(query)}",
    }
    return navigate(engines.get(engine.lower(), engines["google"]), tab_id=tab_id)


def tabs() -> list[dict[str, Any]]:
    _ensure_server()
    data = _request_json("GET", "/tabs")
    if isinstance(data, list):
        return data
    return data.get("tabs", []) if isinstance(data, dict) else []


def _resolve_tab_id(tab_id: str | None = None) -> str:
    explicit = str(tab_id or "").strip()
    if explicit:
        return explicit

    current_tabs = tabs()
    active = [t for t in current_tabs if str(t.get("status", "")).lower() == "active"]
    if active:
        return str(active[0].get("id"))
    if current_tabs:
        return str(current_tabs[0].get("id"))
    raise RuntimeError("PinchTab has no browser tabs. Open a page before performing browser interaction.")


def snapshot(tab_id: str | None = None, interactive: bool = True, compact: bool = True) -> Any:
    _ensure_server()
    resolved_tab = _resolve_tab_id(tab_id) if tab_id else None
    path = "/snapshot" if not resolved_tab else f"/tabs/{resolved_tab}/snapshot"
    params = {"interactive": str(interactive).lower(), "compact": str(compact).lower()}
    return _request_json("GET", path, params=params)


def text(tab_id: str | None = None, mode: str = "readability") -> str:
    _ensure_server()
    resolved_tab = _resolve_tab_id(tab_id) if tab_id else None
    path = "/text" if not resolved_tab else f"/tabs/{resolved_tab}/text"
    data = _request_json("GET", path, params={"mode": mode})
    if isinstance(data, dict):
        return str(data.get("text", json.dumps(data, ensure_ascii=False)))
    return str(data)


def action(tab_id: str, kind: str, **params: Any) -> Any:
    _ensure_server()
    resolved_tab = _resolve_tab_id(tab_id)
    payload = {"kind": kind, **params}
    return _request_json("POST", f"/tabs/{resolved_tab}/action", json=payload)


def click(tab_id: str, ref: str) -> Any:
    return action(tab_id, "click", ref=ref)


def fill(tab_id: str, selector: str | None = None, text_value: str = "", ref: str | None = None) -> Any:
    if ref:
        return action(tab_id, "fill", ref=ref, text=text_value)
    return action(tab_id, "fill", selector=selector or "", text=text_value)


def press(tab_id: str, key: str) -> Any:
    return action(tab_id, "press", key=key)


def screenshot(tab_id: str | None = None, output: Path | None = None) -> bytes:
    _ensure_server()
    resolved_tab = _resolve_tab_id(tab_id) if tab_id else None
    path = "/screenshot" if not resolved_tab else f"/tabs/{resolved_tab}/screenshot"
    r = _request("GET", path, params={"raw": "true"})
    if r.status_code == 401:
        _refresh_token()
        r = _request("GET", path, params={"raw": "true"})
    r.raise_for_status()
    data = r.content
    if output:
        output.parent.mkdir(parents=True, exist_ok=True)
        output.write_bytes(data)
    return data


def _node_values(node: dict[str, Any]) -> str:
    values = (
        node.get("name"), node.get("label"), node.get("text"),
        node.get("placeholder"), node.get("title"), node.get("value"),
        node.get("role"), node.get("tag"),
    )
    return " ".join(str(v) for v in values if v).lower()


def _node_is_editable(node: dict[str, Any]) -> bool:
    role = str(node.get("role") or "").lower()
    tag = str(node.get("tag") or "").lower()
    return role in {
        "textbox", "searchbox", "combobox", "spinbutton",
    } or tag in {"input", "textarea"}


def _find_ref(tab_id: str, description: str, editable_only: bool = False) -> tuple[str | None, dict[str, Any] | None]:
    resolved_tab = _resolve_tab_id(tab_id)
    snap = snapshot(resolved_tab, interactive=True, compact=False)
    nodes = snap.get("nodes", []) if isinstance(snap, dict) else []
    query = str(description or "").strip().lower()
    if not query:
        return None, None

    candidates = []
    for node in nodes:
        ref = node.get("ref")
        if not ref:
            continue
        editable = _node_is_editable(node)
        if editable_only and not editable:
            continue

        hay = _node_values(node)
        score = 0
        role = str(node.get("role") or "").lower()
        tag = str(node.get("tag") or "").lower()

        if query == str(node.get("name") or "").lower():
            score += 100
        if query == str(node.get("label") or "").lower():
            score += 90
        if query == str(node.get("placeholder") or "").lower():
            score += 95
        if query in hay:
            score += 60

        query_words = [w for w in query.split() if len(w) > 2]
        score += sum(10 for w in query_words if w in hay)

        # Semantic role preference.
        if editable:
            score += 25
        if "search" in query:
            if role in {"searchbox", "textbox", "combobox"}:
                score += 80
            elif role == "search":
                # A search landmark/container is useful for click, but should
                # never outrank the actual editable search field for typing.
                score += 10
            if tag == "input":
                score += 70
        if "email" in query and ("email" in hay or "mail" in hay):
            score += 60
        if any(word in query for word in ("password", "passcode")) and role in {"textbox", "combobox"}:
            score += 20
        if "button" in query and role == "button":
            score += 40

        if score:
            candidates.append((score, ref, node))

    candidates.sort(key=lambda item: item[0], reverse=True)
    if candidates:
        _, ref, node = candidates[0]
        return ref, node
    return None, None


def smart_click(tab_id: str | None, description: str) -> Any:
    resolved_tab = _resolve_tab_id(tab_id)
    ref, node = _find_ref(resolved_tab, description, editable_only=False)
    if not ref:
        raise RuntimeError(f"PinchTab could not find element: {description}")
    result = click(resolved_tab, ref)
    return {"target": node, "ref": ref, "tabId": resolved_tab, "result": result}


def smart_type(tab_id: str | None, description: str, text_value: str) -> Any:
    resolved_tab = _resolve_tab_id(tab_id)
    ref, node = _find_ref(resolved_tab, description, editable_only=True)
    if not ref:
        raise RuntimeError(f"PinchTab could not find editable input: {description}")
    result = fill(resolved_tab, text_value=text_value, ref=ref)
    return {"target": node, "ref": ref, "tabId": resolved_tab, "result": result}


def browser_control(parameters: dict[str, Any]) -> str:
    action_name = str(parameters.get("action", "")).lower().strip()
    tab_id = parameters.get("tab_id") or parameters.get("tabId")

    if action_name in {"go_to", "navigate"}:
        return json.dumps(navigate(str(parameters.get("url", "")), tab_id=tab_id), ensure_ascii=False)
    if action_name == "search":
        return json.dumps(search(str(parameters.get("query", "")), str(parameters.get("engine", "google")), tab_id), ensure_ascii=False)
    if action_name in {"tabs", "list_tabs"}:
        return json.dumps(tabs(), ensure_ascii=False)
    if action_name == "snapshot":
        return json.dumps(snapshot(tab_id), ensure_ascii=False)
    if action_name == "get_text":
        return text(tab_id)
    if action_name == "click":
        resolved_tab = _resolve_tab_id(tab_id)
        return json.dumps(click(resolved_tab, str(parameters.get("ref", ""))), ensure_ascii=False)
    if action_name == "fill":
        resolved_tab = _resolve_tab_id(tab_id)
        return json.dumps(
            fill(resolved_tab, parameters.get("selector"), str(parameters.get("text", "")), ref=parameters.get("ref")),
            ensure_ascii=False,
        )
    if action_name == "smart_click":
        return json.dumps(smart_click(tab_id, str(parameters.get("description", ""))), ensure_ascii=False)
    if action_name == "smart_type":
        return json.dumps(smart_type(tab_id, str(parameters.get("description", "")), str(parameters.get("text", ""))), ensure_ascii=False)
    if action_name == "press":
        resolved_tab = _resolve_tab_id(tab_id)
        return json.dumps(press(resolved_tab, str(parameters.get("key", "Enter"))), ensure_ascii=False)
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
