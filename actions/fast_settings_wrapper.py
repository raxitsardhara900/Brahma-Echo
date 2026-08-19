import re
from . import computer_settings_legacy as _legacy


# Keep all existing computer actions, but make deterministic Windows volume handling
# and common command parsing local so simple controls do not require another AI call.

def _windows_volume_set(percent: int):
    percent = max(0, min(100, int(percent)))
    if _legacy._OS != "Windows":
        return _legacy.volume_set(percent)

    try:
        from ctypes import cast, POINTER
        from comtypes import CLSCTX_ALL
        from pycaw.pycaw import AudioUtilities, IAudioEndpointVolume

        device = AudioUtilities.GetSpeakers()
        interface = device.Activate(IAudioEndpointVolume._iid_, CLSCTX_ALL, None)
        endpoint = cast(interface, POINTER(IAudioEndpointVolume))

        # Scalar is the correct Windows Core Audio range: 0.0 .. 1.0.
        endpoint.SetMasterVolumeLevelScalar(percent / 100.0, None)
        actual = round(endpoint.GetMasterVolumeLevelScalar() * 100)
        if abs(actual - percent) > 1:
            raise RuntimeError(f"Windows readback was {actual}% after requesting {percent}%")
        return actual
    except Exception as exc:
        print(f"[Settings] Direct scalar volume set failed: {exc}")
        raise


def _normalize_percent(value):
    if value is None:
        return None
    match = re.search(r"(?<!\d)(\d{1,3})(?:\s*%|\b)", str(value))
    if not match:
        return None
    return max(0, min(100, int(match.group(1))))


def _local_action(description):
    text = (description or "").strip().lower()
    if not text:
        return None

    if any(x in text for x in ("mute", "silence")):
        return "mute", None
    if any(x in text for x in ("unmute", "sound on", "audio on")):
        return "unmute", None

    if any(x in text for x in ("volume", "sound", "audio")):
        if any(x in text for x in ("up", "increase", "raise", "higher", "louder")):
            return "volume_up", None
        if any(x in text for x in ("down", "decrease", "lower", "reduce", "quieter")):
            return "volume_down", None
        value = _normalize_percent(text)
        if value is not None:
            return "volume_set", value

    if "brightness" in text:
        if any(x in text for x in ("up", "increase", "raise", "higher")):
            return "brightness_up", None
        if any(x in text for x in ("down", "decrease", "lower", "reduce")):
            return "brightness_down", None
        value = _normalize_percent(text)
        if value is not None:
            return "brightness_set", value

    simple = {
        "fullscreen": "full_screen", "full screen": "full_screen",
        "maximize": "maximize", "minimize": "minimize",
        "close window": "close_window", "close app": "close_app",
        "refresh": "refresh_page", "reload": "refresh_page",
        "new tab": "new_tab", "close tab": "close_tab",
        "next tab": "next_tab", "previous tab": "prev_tab",
        "show desktop": "show_desktop", "task manager": "task_manager",
        "lock screen": "lock_screen", "settings": "open_settings",
        "file explorer": "file_explorer", "run dialog": "open_run",
    }
    for phrase, action in simple.items():
        if phrase in text:
            return action, None

    return None


def computer_settings(parameters=None, response=None, player=None, session_memory=None):
    params = dict(parameters or {})
    raw_action = str(params.get("action") or "").strip().lower().replace("-", "_").replace(" ", "_")
    description = str(params.get("description") or "").strip()
    value = params.get("value")

    local = _local_action(description) if not raw_action else None
    if local:
        raw_action, value = local

    # Normalize model-generated aliases.
    aliases = {
        "volume": "volume_set",
        "set": "volume_set",
        "set_volume": "volume_set",
        "setvolume": "volume_set",
        "volume_level": "volume_set",
    }
    action = aliases.get(raw_action, raw_action)

    if action in ("volume_set", "volume", "set_volume"):
        percent = _normalize_percent(value if value is not None else description)
        if percent is None:
            return "Please specify a volume percentage from 0 to 100."
        try:
            actual = _windows_volume_set(percent)
            if player:
                player.write_log(f"[Settings] volume_set {actual}%")
            return f"Volume set to {actual}%."
        except Exception as exc:
            return f"Could not set volume to {percent}%: {exc}"

    # Handle direct common actions without sending another request to OpenRouter.
    direct = {
        "volume_up": _legacy.volume_up,
        "volume_down": _legacy.volume_down,
        "mute": _legacy.volume_mute,
        "unmute": _legacy.volume_mute,
        "toggle_mute": _legacy.volume_mute,
        "brightness_up": _legacy.brightness_up,
        "brightness_down": _legacy.brightness_down,
        "full_screen": _legacy.full_screen,
        "minimize": _legacy.minimize_window,
        "maximize": _legacy.maximize_window,
        "close_window": _legacy.close_window,
        "close_app": _legacy.close_app,
        "refresh_page": _legacy.refresh_page,
        "reload": _legacy.refresh_page,
        "new_tab": _legacy.new_tab,
        "close_tab": _legacy.close_tab,
        "next_tab": _legacy.next_tab,
        "prev_tab": _legacy.prev_tab,
        "show_desktop": _legacy.show_desktop,
        "task_manager": _legacy.open_task_manager,
        "lock_screen": _legacy.lock_screen,
        "open_settings": _legacy.open_system_settings,
        "file_explorer": _legacy.open_file_explorer,
        "open_run": _legacy.open_run,
    }
    if action in direct:
        try:
            direct[action]()
            return f"Done: {action}."
        except Exception as exc:
            return f"Action failed ({action}): {exc}"

    # Preserve all other existing commands exactly as before.
    return _legacy.computer_settings(
        parameters=params,
        response=response,
        player=player,
        session_memory=session_memory,
    )
