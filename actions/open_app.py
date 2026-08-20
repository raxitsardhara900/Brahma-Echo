# actions/open_app.py
# Brahma AI - Cross-Platform App Launcher

import time
import subprocess
import platform
import shutil
import re
import webbrowser
import os

try:
    import psutil
    _PSUTIL = True
except ImportError:
    _PSUTIL = False

_APP_ALIASES = {
    "whatsapp": {"Windows": "WhatsApp", "Darwin": "WhatsApp", "Linux": "whatsapp"},
    "chrome": {"Windows": "chrome", "Darwin": "Google Chrome", "Linux": "google-chrome"},
    "google chrome": {"Windows": "chrome", "Darwin": "Google Chrome", "Linux": "google-chrome"},
    "firefox": {"Windows": "firefox", "Darwin": "Firefox", "Linux": "firefox"},
    "spotify": {"Windows": "Spotify", "Darwin": "Spotify", "Linux": "spotify"},
    "vscode": {"Windows": "code", "Darwin": "Visual Studio Code", "Linux": "code"},
    "visual studio code": {"Windows": "code", "Darwin": "Visual Studio Code", "Linux": "code"},
    "discord": {"Windows": "Discord", "Darwin": "Discord", "Linux": "discord"},
    "telegram": {"Windows": "Telegram", "Darwin": "Telegram", "Linux": "telegram"},
    "instagram": {"Windows": "Instagram", "Darwin": "Instagram", "Linux": "instagram"},
    "tiktok": {"Windows": "TikTok", "Darwin": "TikTok", "Linux": "TikTok"},
    "notepad": {"Windows": "notepad.exe", "Darwin": "TextEdit", "Linux": "gedit"},
    "calculator": {"Windows": "calc.exe", "Darwin": "Calculator", "Linux": "gnome-calculator"},
    "terminal": {"Windows": "cmd.exe", "Darwin": "Terminal", "Linux": "gnome-terminal"},
    "cmd": {"Windows": "cmd.exe", "Darwin": "Terminal", "Linux": "bash"},
    "explorer": {"Windows": "explorer.exe", "Darwin": "Finder", "Linux": "nautilus"},
    "file explorer": {"Windows": "explorer.exe", "Darwin": "Finder", "Linux": "nautilus"},
    "paint": {"Windows": "mspaint.exe", "Darwin": "Preview", "Linux": "gimp"},
    "word": {"Windows": "winword", "Darwin": "Microsoft Word", "Linux": "libreoffice --writer"},
    "excel": {"Windows": "excel", "Darwin": "Microsoft Excel", "Linux": "libreoffice --calc"},
    "powerpoint": {"Windows": "powerpnt", "Darwin": "Microsoft PowerPoint", "Linux": "libreoffice --impress"},
    "vlc": {"Windows": "vlc", "Darwin": "VLC", "Linux": "vlc"},
    "zoom": {"Windows": "Zoom", "Darwin": "zoom.us", "Linux": "zoom"},
    "slack": {"Windows": "Slack", "Darwin": "Slack", "Linux": "slack"},
    "steam": {"Windows": "steam", "Darwin": "steam", "Linux": "steam"},
    "task manager": {"Windows": "taskmgr.exe", "Darwin": "Activity Monitor", "Linux": "gnome-system-monitor"},
    "settings": {"Windows": "ms-settings:", "Darwin": "System Preferences", "Linux": "gnome-control-center"},
    "powershell": {"Windows": "powershell.exe", "Darwin": "Terminal", "Linux": "bash"},
    "edge": {"Windows": "msedge", "Darwin": "Microsoft Edge", "Linux": "microsoft-edge"},
    "brave": {"Windows": "brave", "Darwin": "Brave Browser", "Linux": "brave-browser"},
    "obsidian": {"Windows": "Obsidian", "Darwin": "Obsidian", "Linux": "obsidian"},
    "notion": {"Windows": "Notion", "Darwin": "Notion", "Linux": "notion"},
    "blender": {"Windows": "blender", "Darwin": "Blender", "Linux": "blender"},
    "capcut": {"Windows": "CapCut", "Darwin": "CapCut", "Linux": "capcut"},
    "postman": {"Windows": "Postman", "Darwin": "Postman", "Linux": "postman"},
    "figma": {"Windows": "Figma", "Darwin": "Figma", "Linux": "figma"},
}

_WEBSITE_URLS = {
    "youtube": "https://www.youtube.com",
    "youtube.com": "https://www.youtube.com",
    "google": "https://www.google.com",
    "google.com": "https://www.google.com",
    "gmail": "https://mail.google.com",
    "gmail.com": "https://mail.google.com",
    "openai": "https://openai.com",
    "openai.com": "https://openai.com",
    "chatgpt": "https://chatgpt.com",
    "chatgpt.com": "https://chatgpt.com",
    "github": "https://github.com",
    "github.com": "https://github.com",
    "openrouter": "https://openrouter.ai",
    "openrouter.ai": "https://openrouter.ai",
    "instagram": "https://www.instagram.com",
    "instagram.com": "https://www.instagram.com",
    "facebook": "https://www.facebook.com",
    "facebook.com": "https://www.facebook.com",
    "linkedin": "https://www.linkedin.com",
    "linkedin.com": "https://www.linkedin.com",
    "reddit": "https://www.reddit.com",
    "reddit.com": "https://www.reddit.com",
    "amazon": "https://www.amazon.in",
    "amazon.in": "https://www.amazon.in",
    "flipkart": "https://www.flipkart.com",
    "flipkart.com": "https://www.flipkart.com",
    "x": "https://x.com",
    "twitter": "https://x.com",
    "twitter.com": "https://x.com",
}


def _looks_like_url(value: str) -> bool:
    value = str(value or "").strip()
    if not value:
        return False
    if value.startswith(("http://", "https://")):
        return True
    return bool(re.fullmatch(r"[A-Za-z0-9.-]+\.[A-Za-z]{2,}(/.*)?", value))


def _website_url(raw: str) -> str | None:
    key = str(raw or "").strip().lower()
    if not key:
        return None
    if key in _WEBSITE_URLS:
        return _WEBSITE_URLS[key]
    if _looks_like_url(key):
        return key if key.startswith(("http://", "https://")) else "https://" + key
    return None


def _open_in_default_browser(raw: str, player=None) -> str | None:
    url = _website_url(raw)
    if not url:
        return None

    # Windows: os.startfile(url) delegates to the user's actual default URL
    # handler. This is more deterministic than webbrowser.open() when an
    # embedded/registered browser handler is present.
    if platform.system() == "Windows":
        try:
            os.startfile(url)
            print(f"[Browser] Opened in system default browser: {url}")
            if player:
                player.write_log(f"[Browser] Opened in system default browser: {url}")
            return f"Opened {raw} in the system default browser."
        except Exception as exc:
            print(f"[Browser] Windows default handler failed for {raw}: {exc}")

    try:
        opened = webbrowser.open(url, new=0, autoraise=True)
        if opened:
            print(f"[Browser] Opened in system default browser: {url}")
            if player:
                player.write_log(f"[Browser] Opened in system default browser: {url}")
            return f"Opened {raw} in the system default browser."
    except Exception as exc:
        print(f"[Browser] Default browser open failed for {raw}: {exc}")
    return None


def _normalize(raw: str) -> str:
    system = platform.system()
    key = str(raw or "").lower().strip()
    if key in _APP_ALIASES:
        return _APP_ALIASES[key].get(system, raw)
    for alias_key, os_map in _APP_ALIASES.items():
        if alias_key in key or key in alias_key:
            return os_map.get(system, raw)
    return raw


def _launch_windows(app_name: str) -> bool:
    try:
        import pyautogui
        pyautogui.PAUSE = 0.1
        pyautogui.press("win")
        time.sleep(0.6)
        pyautogui.write(app_name, interval=0.05)
        time.sleep(0.8)
        pyautogui.press("enter")
        time.sleep(3.0)
        return True
    except Exception as exc:
        print(f"[open_app] Windows launch failed: {exc}")
        return False


def _launch_macos(app_name: str) -> bool:
    try:
        result = subprocess.run(["open", "-a", app_name], capture_output=True, timeout=8)
        return result.returncode == 0
    except Exception:
        return False


def _launch_linux(app_name: str) -> bool:
    binary = shutil.which(app_name) or shutil.which(app_name.lower()) or shutil.which(app_name.lower().replace(" ", "-"))
    if binary:
        try:
            subprocess.Popen([binary], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            return True
        except Exception:
            pass
    try:
        subprocess.run(["xdg-open", app_name], capture_output=True, timeout=5)
        return True
    except Exception:
        return False


_OS_LAUNCHERS = {"Windows": _launch_windows, "Darwin": _launch_macos, "Linux": _launch_linux}


def open_app(parameters=None, response=None, player=None, session_memory=None) -> str:
    app_name = str((parameters or {}).get("app_name", "")).strip()
    if not app_name:
        return "Please specify which application to open, sir."

    website_result = _open_in_default_browser(app_name, player=player)
    if website_result is not None:
        return website_result

    system = platform.system()
    launcher = _OS_LAUNCHERS.get(system)
    if launcher is None:
        return f"Unsupported OS: {system}"

    normalized = _normalize(app_name)
    print(f"[open_app] Launching: {app_name} -> {normalized} ({system})")
    if player:
        player.write_log(f"[open_app] {app_name}")

    try:
        if launcher(normalized):
            return f"Opened {app_name} successfully, sir."
        if normalized != app_name and launcher(app_name):
            return f"Opened {app_name} successfully, sir."
        return f"I tried to open {app_name}, sir, but couldn't confirm it launched."
    except Exception as exc:
        return f"Failed to open {app_name}, sir: {exc}"
