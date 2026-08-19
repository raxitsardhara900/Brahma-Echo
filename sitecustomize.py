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
                return f"Laptop brightness increased to {actual}%."
            return original({**params, "action": "brightness_up"}, response=response, player=player, session_memory=session_memory)

        if action in {"brightness_down", "brightness_decrease"}:
            if platform.system() == "Windows":
                actual = _windows_brightness_step(-10)
                return f"Laptop brightness decreased to {actual}%."
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
