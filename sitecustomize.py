"""Startup hook that makes deterministic computer controls local and fast.

Python automatically imports sitecustomize when this project directory is on sys.path.
The hook patches actions.computer_settings.computer_settings before main.py imports it.
"""

import re
import platform


def _patch():
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
            # One final exact write removes rounding/drift on some endpoints.
            endpoint.SetMasterVolumeLevelScalar(float(percent) / 100.0, None)
            actual = round(endpoint.GetMasterVolumeLevelScalar() * 100)
        return actual

    def _local(parameters, player):
        params = dict(parameters or {})
        action = str(params.get("action") or "").strip().lower().replace("-", "_").replace(" ", "_")
        description = str(params.get("description") or "").strip().lower()
        value = params.get("value")

        # Natural-language volume commands are handled without another AI call.
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

        # Delegate all other commands to the existing implementation unchanged.
        return original(parameters=params, player=player)

    cs.computer_settings = _local


_patch()
