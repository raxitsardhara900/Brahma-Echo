"""Fast local routing for deterministic desktop controls.

Defaults:
- Windows volume/brightness are handled locally on this PC.
- Normal browser navigation uses the operating-system default browser.
- PinchTab remains available as an explicit opt-in backend via
  BRAHMA_BROWSER_BACKEND=pinchtab, with Playwright fallback.
"""
from __future__ import annotations

import os
import platform
import re
import subprocess


def _parse_percent(value) -> int | None:
    if value is None:
        return None
    text = str(value).strip().replace(",", ".")
    match = re.search(r"(?<!\d)(\d{1,3})(?:\s*%|\b)", text)
    if not match:
        return None
    return max(0, min(100, int(match.group(1))))


def _set_windows_volume(percent: int) -> int:
    from ctypes import POINTER, cast
    from comtypes import CLSCTX_ALL
    from pycaw.pycaw import AudioUtilities, IAudioEndpointVolume

    device = AudioUtilities.GetSpeakers()
    endpoint = device.Activate(IAudioEndpointVolume._iid_, CLSCTX_ALL, None)
    volume = cast(endpoint, POINTER(IAudioEndpointVolume))
    volume.SetMasterVolumeLevelScalar(percent / 100.0, None)
    actual = round(volume.GetMasterVolumeLevelScalar() * 100)
    if actual != percent:
        volume.SetMasterVolumeLevelScalar(percent / 100.0, None)
        actual = round(volume.GetMasterVolumeLevelScalar() * 100)
    return actual


def _windows_brightness_get() -> int | None:
    script = (
        "$b=(Get-CimInstance -Namespace root/WMI -ClassName WmiMonitorBrightness)"
        ".CurrentBrightness; Write-Output $b"
    )
    try:
        result = subprocess.run(
            ["powershell.exe", "-NoProfile", "-Command", script],
            capture_output=True, text=True, timeout=5
        )
        if result.returncode == 0:
            return int(result.stdout.strip().splitlines()[-1])
    except Exception:
        pass
    return None


def _windows_brightness_set(percent: int) -> int:
    percent = max(0, min(100, int(percent)))
    script = (
        "$m=Get-CimInstance -Namespace root/WMI -ClassName WmiMonitorBrightnessMethods;"
        "$m.WmiSetBrightness(1," + str(percent) + ");"
        "$b=(Get-CimInstance -Namespace root/WMI -ClassName WmiMonitorBrightness).CurrentBrightness;"
        "Write-Output $b"
    )
    result = subprocess.run(
        ["powershell.exe", "-NoProfile", "-Command", script],
        capture_output=True, text=True, timeout=5
    )
    if result.returncode != 0:
        raise RuntimeError(result.stderr.strip() or "Windows brightness API failed")
    try:
        return int(result.stdout.strip().splitlines()[-1])
    except Exception:
        return percent


def _windows_brightness_step(delta: int) -> int:
    current = _windows_brightness_get()
    if current is None:
        current = 50
    return _windows_brightness_set(current + int(delta))


def _install_computer_settings() -> None:
    try:
        import actions.computer_settings as cs
    except Exception:
        return

    original = cs.computer_settings

    def fast_computer_settings(parameters=None, response=None, player=None, session_memory=None):
        params = dict(parameters or {})
        raw_action = str(params.get("action") or "").strip().lower()
        action = raw_action.replace("-", "_").replace(" ", "_")
        description = str(params.get("description") or "").strip().lower()
        value = params.get("value")

        volume_aliases = {
            "volume": "volume_set",
            "set": "volume_set",
            "set_volume": "volume_set",
            "setvolume": "volume_set",
            "volume_level": "volume_set",
        }
        brightness_aliases = {
            "brightness": "brightness_set",
            "set_brightness": "brightness_set",
            "setbrightness": "brightness_set",
            "screen_brightness": "brightness_set",
            "screenbrightness": "brightness_set",
        }
        if action in volume_aliases:
            action = volume_aliases[action]
        elif action in brightness_aliases:
            action = brightness_aliases[action]

        if action == "volume_set":
            target = _parse_percent(value)
            if target is None:
                target = _parse_percent(description)
            if target is None:
                return "Please specify a volume from 0 to 100 percent."
            if platform.system() == "Windows":
                actual = _set_windows_volume(target)
            else:
                cs.volume_set(target)
                actual = target
            if player:
                player.write_log(f"[Settings] volume_set {actual}%")
            return f"Volume set to {actual}%."

        if action == "brightness_set":
            target = _parse_percent(value)
            if target is None:
                target = _parse_percent(description)
            if target is None:
                return "Please specify brightness from 0 to 100 percent."
            if platform.system() == "Windows":
                actual = _windows_brightness_set(target)
                if player:
                    player.write_log(f"[Settings] laptop_brightness_set {actual}%")
                return f"Laptop brightness set to {actual}%."
            return original({**params, "action": "brightness_up"}, response=response, player=player, session_memory=session_memory)

        if not action and description:
            if any(x in description for x in ("brightness", "screen brightness", "display brightness", "laptop brightness")):
                target = _parse_percent(description)
                if target is not None and platform.system() == "Windows":
                    actual = _windows_brightness_set(target)
                    if player:
                        player.write_log(f"[Settings] laptop_brightness_set {actual}%")
                    return f"Laptop brightness set to {actual}%"
                if any(x in description for x in ("up", "increase", "raise", "higher", "brighter")):
                    action = "brightness_up"
                elif any(x in description for x in ("down", "decrease", "lower", "reduce", "dimmer")):
                    action = "brightness_down"

        if action in {"brightness_up", "brightness_increase"}:
            if platform.system() == "Windows":
                actual = _windows_brightness_step(+10)
                return f"Laptop brightness increased to {actual}%"
            return original({**params, "action": "brightness_up"}, response=response, player=player, session_memory=session_memory)

        if action in {"brightness_down", "brightness_decrease"}:
            if platform.system() == "Windows":
                actual = _windows_brightness_step(-10)
                return f"Laptop brightness decreased to {actual}%"
            return original({**params, "action": "brightness_down"}, response=response, player=player, session_memory=session_memory)

        if not action and description:
            if "unmute" in description or "sound on" in description or "audio on" in description:
                action = "unmute"
            elif "mute" in description or "silence" in description:
                action = "mute"
            elif any(x in description for x in ("volume", "sound", "audio")):
                if any(x in description for x in ("up", "increase", "raise", "higher", "louder")):
                    action = "volume_up"
                elif any(x in description for x in ("down", "decrease", "lower", "reduce", "quieter")):
                    action = "volume_down"
                else:
                    target = _parse_percent(description)
                    if target is not None:
                        action = "volume_set"
                        if platform.system() == "Windows":
                            actual = _set_windows_volume(target)
                        else:
                            cs.volume_set(target)
                            actual = target
                        if player:
                            player.write_log(f"[Settings] volume_set {actual}%")
                        return f"Volume set to {actual}%."

        direct = {
            "volume_up": cs.volume_up,
            "volume_down": cs.volume_down,
            "mute": cs.volume_mute,
            "unmute": cs.volume_mute,
            "toggle_mute": cs.volume_mute,
            "brightness_up": cs.brightness_up,
            "brightness_down": cs.brightness_down,
            "full_screen": cs.full_screen,
            "fullscreen": cs.full_screen,
            "maximize": cs.maximize_window,
            "minimize": cs.minimize_window,
            "close_window": cs.close_window,
            "close_app": cs.close_app,
            "refresh_page": cs.refresh_page,
            "reload": cs.refresh_page,
            "new_tab": cs.new_tab,
            "close_tab": cs.close_tab,
            "next_tab": cs.next_tab,
            "prev_tab": cs.prev_tab,
            "show_desktop": cs.show_desktop,
            "task_manager": cs.open_task_manager,
            "lock_screen": cs.lock_screen,
            "open_settings": cs.open_system_settings,
            "file_explorer": cs.open_file_explorer,
            "open_run": cs.open_run,
        }
        func = direct.get(action)
        if func:
            func()
            return f"Done: {action}."

        return original(params, response=response, player=player, session_memory=session_memory)

    cs.computer_settings = fast_computer_settings


def _install_pinchtab_browser_routing() -> None:
    """PinchTab is opt-in; normal browser actions use the system default browser."""
    try:
        import actions.browser_control as bc
        from actions import pinchtab_client as pt
    except Exception:
        return

    original = bc.browser_control
    backend = os.environ.get("BRAHMA_BROWSER_BACKEND", "system").strip().lower()
    if backend != "pinchtab":
        print("[Browser] System default browser mode active (PinchTab opt-in).")
        return

    supported = {
        "go_to", "navigate", "tabs", "list_tabs", "snapshot", "get_text",
        "click", "fill", "press", "screenshot", "server_start", "health"
    }

    def routed_browser_control(parameters=None, response=None, player=None, session_memory=None):
        params = dict(parameters or {})
        action = str(params.get("action") or "").strip().lower()
        if action in supported:
            try:
                result = pt.browser_control(params)
                if player:
                    player.write_log(f"[PinchTab] primary browser action: {action}")
                else:
                    print(f"[PinchTab] primary browser action: {action}")
                return result
            except Exception as exc:
                msg = f"[PinchTab] unavailable for {action}: {exc}; using system browser fallback."
                if player:
                    player.write_log(msg)
                else:
                    print(msg)
        return original(params, response=response, player=player, session_memory=session_memory)

    bc.browser_control = routed_browser_control


_install_computer_settings()
_install_pinchtab_browser_routing()

# BRAHMA_LOCAL_DEVICE_ROUTING_V2
# ------------------------------------------------------------
# 1. Laptop brightness is always local when:
#    - user explicitly says laptop/screen/display/PC/computer
#    - OR no smart-home devices are connected
# 2. Normal browser search/navigation uses Windows default browser.
# 3. PinchTab remains opt-in via BRAHMA_BROWSER_BACKEND=pinchtab.
# ------------------------------------------------------------

def _install_local_laptop_routing():
    import re
    import webbrowser

    try:
        from smart_home.service import SmartHomeService

        def _local_brightness_execute(self, command):
            text = str(command or "").strip().lower()
            brightness_words = (
                "brightness", "brighten", "brighter", "dim", "dimmer",
                "screen brightness", "display brightness",
            )
            local_words = (
                "laptop", "my laptop", "screen", "my screen", "display",
                "my display", "computer", "my computer", "pc", "my pc",
                "this pc", "windows",
            )
            increase_words = (
                "up", "increase", "raise", "higher", "brighter",
                "increase brightness", "brightness up", "badhao", "badhado",
            )
            decrease_words = (
                "down", "decrease", "lower", "reduce", "dimmer", "dim",
                "low", "brightness down", "brightness low", "kam", "ghata", "ghatao",
            )

            if not any(word in text for word in brightness_words):
                return None
            try:
                has_smart_devices = self.device_count() > 0
            except Exception:
                has_smart_devices = False
            explicitly_local = any(word in text for word in local_words)
            if not explicitly_local and has_smart_devices:
                return None

            from actions.computer_settings import computer_settings
            number_match = re.search(r"(?<!\d)(\d{1,3})(?:\s*%)?(?!\d)", text)
            if number_match:
                value = max(0, min(100, int(number_match.group(1))))
                result = computer_settings({"action": "brightness_set", "value": value, "description": text})
            elif any(word in text for word in decrease_words):
                result = computer_settings({"action": "brightness_down", "description": text})
            elif any(word in text for word in increase_words):
                result = computer_settings({"action": "brightness_up", "description": text})
            else:
                return {
                    "action": "laptop_brightness",
                    "targets": ["This PC"],
                    "detail": "Please specify brightness percentage or say increase/decrease.",
                    "count": 1,
                }

            return {
                "action": "laptop_brightness",
                "targets": ["This PC"],
                "detail": str(result),
                "count": 1,
            }

        SmartHomeService.execute_command = _local_brightness_execute
    except Exception as _exc:
        print(f"[LocalBrightness] Smart-home bridge patch skipped: {_exc}")

    try:
        import actions.browser_control as browser_module
        original_browser_control = browser_module.browser_control

        def _system_default_browser_control(parameters=None, response=None, player=None, session_memory=None):
            params = dict(parameters or {})
            action = str(params.get("action") or "").strip().lower()
            backend = os.environ.get("BRAHMA_BROWSER_BACKEND", "system").strip().lower()

            if backend == "pinchtab":
                return original_browser_control(params, response=response, player=player, session_memory=session_memory)

            if action == "search":
                query = str(params.get("query") or "").strip()
                if not query:
                    return "Search query is empty."
                from urllib.parse import quote_plus
                url = "https://www.google.com/search?q=" + quote_plus(query)
                opened = _brahma_open_default_browser_once(url)
                message = (
                    f"Opened search in system default browser: {query}"
                    if opened else f"Already open in system default browser: {query}"
                )
                if player:
                    player.write_log("[Browser] " + message)
                return message

            if action in {"go_to", "navigate"}:
                url = str(params.get("url") or "").strip()
                if not url:
                    return "URL is empty."
                if not url.startswith(("http://", "https://")):
                    url = "https://" + url
                opened = _brahma_open_default_browser_once(url)
                message = (
                    f"Opened URL in system default browser: {url}"
                    if opened else f"Already open in system default browser: {url}"
                )
                if player:
                    player.write_log("[Browser] " + message)
                return message

            return original_browser_control(params, response=response, player=player, session_memory=session_memory)

        browser_module.browser_control = _system_default_browser_control
    except Exception as _exc:
        print(f"[DefaultBrowser] Patch skipped: {_exc}")


_install_local_laptop_routing()

# ============================================================
# BRAHMA LAPTOP BRIGHTNESS BACKEND V3
# Uses screen_brightness_control for the actual laptop display.
# Falls back to existing WMI implementation if unavailable.
# ============================================================

def _install_sbc_brightness_backend():
    global _windows_brightness_get
    global _windows_brightness_set
    global _windows_brightness_step

    try:
        import screen_brightness_control as sbc

        def _sbc_brightness_get():
            try:
                value = sbc.get_brightness(display=0)
                if isinstance(value, (list, tuple)):
                    if not value:
                        return None
                    value = value[0]
                return int(round(float(value)))
            except Exception as exc:
                print(f"[Brightness] SBC get failed: {exc}")
                return None

        def _sbc_brightness_set(percent):
            percent = max(0, min(100, int(percent)))
            sbc.set_brightness(percent, display=0)
            actual = _sbc_brightness_get()
            if actual is None:
                return percent
            return actual

        def _sbc_brightness_step(delta):
            current = _sbc_brightness_get()
            if current is None:
                current = 50
            target = max(0, min(100, current + int(delta)))
            return _sbc_brightness_set(target)

        _windows_brightness_get = _sbc_brightness_get
        _windows_brightness_set = _sbc_brightness_set
        _windows_brightness_step = _sbc_brightness_step
        print("[Brightness] SBC laptop backend ACTIVE.")
        print("[Brightness] Target display: 0")
    except Exception as exc:
        print(f"[Brightness] SBC unavailable, keeping WMI backend: {exc}")


_install_sbc_brightness_backend()

# ============================================================
# Existing session-level duplicate protection is kept as a fallback.
# The function below is intentionally redefined afterwards so actual
# browser-tab enumeration runs first for the user's real browser.
# ============================================================

_BRAHMA_OPENED_DEFAULT_BROWSER_URLS = set()


def _browser_title_patterns():
    prog_id = ""
    try:
        prog_id = _get_default_browser_id()
    except Exception:
        pass
    patterns = []
    if "edge" in prog_id:
        patterns.append(r".*Microsoft Edge$")
    if "chrome" in prog_id:
        patterns.append(r".*Google Chrome$")
    if "brave" in prog_id:
        patterns.append(r".*Brave$")
    if "opera" in prog_id:
        patterns.append(r".*Opera(?: GX)?$")
    if "vivaldi" in prog_id:
        patterns.append(r".*Vivaldi$")
    if "firefox" in prog_id or "mozilla" in prog_id:
        patterns.append(r".*Mozilla Firefox$")
    if not patterns:
        patterns = [
            r".*Google Chrome$", r".*Microsoft Edge$", r".*Brave$",
            r".*Opera(?: GX)?$", r".*Vivaldi$", r".*Mozilla Firefox$",
        ]
    return patterns


def _normalize_browser_url(url: str) -> str:
    from urllib.parse import urlsplit, urlunsplit
    value = str(url or "").strip()
    if not value:
        return ""
    if not value.startswith(("http://", "https://")):
        value = "https://" + value
    try:
        parts = urlsplit(value)
        return urlunsplit((
            parts.scheme.lower(),
            parts.netloc.lower(),
            (parts.path or "/").rstrip("/") or "/",
            parts.query,
            "",
        ))
    except Exception:
        return value.rstrip("/").lower()


def _enumerate_default_browser_tabs():
    if platform.system() != "Windows":
        return []

    try:
        from pywinauto import Desktop
    except Exception as exc:
        print(f"[BrowserEnum] pywinauto unavailable: {exc}")
        return []

    desktop = Desktop(backend="uia")
    windows = []
    for pattern in _browser_title_patterns():
        try:
            windows.extend(desktop.windows(title_re=pattern, control_type="Pane"))
        except Exception:
            pass

    results = []
    seen = set()
    for window in windows:
        try:
            handle = getattr(window, "handle", id(window))
            if handle in seen:
                continue
            seen.add(handle)

            address_bars = window.descendants(title="Address and search bar", control_type="Edit")
            if not address_bars:
                continue
            address_bar = address_bars[0]
            try:
                original_url = _normalize_browser_url(address_bar.get_value())
            except Exception:
                original_url = ""

            tabs = window.descendants(control_type="TabItem")
            for index, tab in enumerate(tabs, 1):
                try:
                    tab.click_input()
                    url = _normalize_browser_url(address_bar.get_value())
                    results.append({
                        "window": window,
                        "tab": tab,
                        "index": index,
                        "title": tab.window_text(),
                        "url": url,
                    })
                except Exception:
                    continue

            if original_url:
                for item in results:
                    if item["window"] is window and item["url"] == original_url:
                        try:
                            item["tab"].click_input()
                        except Exception:
                            pass
                        break
        except Exception:
            continue

    return results


def _focus_existing_browser_url(target_url: str) -> bool:
    target = _normalize_browser_url(target_url)
    if not target:
        return False

    tabs = _enumerate_default_browser_tabs()
    for item in tabs:
        if item.get("url") == target:
            try:
                item["tab"].click_input()
            except Exception:
                pass
            print(f"[BrowserEnum] Existing tab found and focused: {target_url}")
            return True
    return False


def _brahma_open_default_browser_once(url: str) -> bool:
    import webbrowser

    normalized = _normalize_browser_url(url)
    if not normalized:
        return False

    try:
        if _focus_existing_browser_url(url):
            _BRAHMA_OPENED_DEFAULT_BROWSER_URLS.add(normalized)
            return False
    except Exception as exc:
        print(f"[BrowserEnum] Enumeration failed; using session cache: {exc}")

    if normalized in _BRAHMA_OPENED_DEFAULT_BROWSER_URLS:
        print(f"[Browser] Duplicate prevented: {url}")
        return False

    try:
        webbrowser.open(url, new=0)
        _BRAHMA_OPENED_DEFAULT_BROWSER_URLS.add(normalized)
        print(f"[Browser] Opened in default browser: {url}")
        return True
    except Exception as exc:
        _BRAHMA_OPENED_DEFAULT_BROWSER_URLS.discard(normalized)
        print(f"[Browser] Default browser open failed: {exc}")
        return False
