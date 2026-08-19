"""Startup hooks for Brahma Echo.

- Keeps deterministic local computer controls fast.
- Makes PinchTab the primary browser backend while retaining Playwright fallback.
"""

import importlib.util
import os
import re
import platform
import sys
from pathlib import Path


def _patch_computer_settings():
    try:
        import actions.computer_settings as cs
    except Exception:
        return

    original = cs.computer_settings

    def _percent(value):
        if value is None:
            return None
        text = str(value).strip().replace(",", ".")
        m = re.search(r"(?<!\d)(\d{1,3})(?:\s*%|\b)", text)
        if not m:
            return None
        return max(0, min(100, int(m.group(1))))

    def _set_windows_volume(percent):
        from ctypes import POINTER, cast
        from comtypes import CLSCTX_ALL
        from pycaw.pycaw import AudioUtilities, IAudioEndpointVolume

        device = AudioUtilities.GetSpeakers()
        interface = device.Activate(IAudioEndpointVolume._iid_, CLSCTX_ALL, None)
        endpoint = cast(interface, POINTER(IAudioEndpointVolume))
        endpoint.SetMasterVolumeLevelScalar(float(percent) / 100.0, None)
        actual = round(endpoint.GetMasterVolumeLevelScalar() * 100)
        if actual != int(percent):
            endpoint.SetMasterVolumeLevelScalar(float(percent) / 100.0, None)
            actual = round(endpoint.GetMasterVolumeLevelScalar() * 100)
        return actual

    def _local(parameters, player=None):
        params = dict(parameters or {})
        action = str(params.get("action") or "").strip().lower().replace("-", "_").replace(" ", "_")
        description = str(params.get("description") or "").strip().lower()
        value = params.get("value")

        if not action and description:
            if any(w in description for w in ("mute", "silence")) and "unmute" not in description:
                action = "mute"
            elif any(w in description for w in ("unmute", "sound on", "audio on")):
                action = "unmute"
            elif any(w in description for w in ("volume", "sound", "audio")):
                if any(w in description for w in ("up", "increase", "raise", "higher", "louder")):
                    action = "volume_up"
                elif any(w in description for w in ("down", "decrease", "lower", "reduce", "quieter")):
                    action = "volume_down"
                else:
                    value = _percent(description)
                    if value is not None:
                        action = "volume_set"

        aliases = {
            "volume": "volume_set",
            "set": "volume_set",
            "set_volume": "volume_set",
            "setvolume": "volume_set",
            "volume_level": "volume_set",
        }
        action = aliases.get(action, action)

        if action == "volume_set":
            target = _percent(value if value is not None else description)
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
        if action in direct:
            direct[action]()
            return f"Done: {action}."

        return original(parameters=params, player=player)

    cs.computer_settings = _local


def _install_pinchtab_browser_router():
    if os.environ.get("BRAHMA_PINCHTAB", "1").strip().lower() in {"0", "false", "off", "no"}:
        return
    try:
        base = Path(__file__).resolve().parent
        router_path = base / "actions" / "pinchtab_browser_router.py"
        if not router_path.exists():
            return
        spec = importlib.util.spec_from_file_location("actions._pinchtab_browser_router", router_path)
        if spec is None or spec.loader is None:
            return
        router = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(router)
        sys.modules["actions.browser_control"] = router
        print("[PinchTab] Browser router installed (PinchTab primary, Playwright fallback)")
    except Exception as exc:
        print(f"[PinchTab] Browser router unavailable; using original Playwright controller: {exc}")


_patch_computer_settings()
_install_pinchtab_browser_router()
