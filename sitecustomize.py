"""Fast local routing for deterministic computer controls.

This hook normalizes model-generated computer_settings actions before they reach
legacy intent detection. It prevents retries for direct volume/brightness/window
commands and guarantees exact Windows volume percentages via pycaw.
"""
from __future__ import annotations

import platform
import re


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

    # Endpoints that quantize can land one point away. Retry the exact scalar.
    if actual != percent:
        volume.SetMasterVolumeLevelScalar(percent / 100.0, None)
        actual = round(volume.GetMasterVolumeLevelScalar() * 100)
    return actual


def _install():
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

        aliases = {
            "volume": "volume_set",
            "set": "volume_set",
            "set_volume": "volume_set",
            "setvolume": "volume_set",
            "volume_level": "volume_set",
        }
        if action in aliases:
            action = aliases[action]

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


_install()
