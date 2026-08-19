"""Optional adapter for the vendored JARVIS-AGI subsystem.

Brahma Echo remains the primary runtime. The original JARVIS main.py is not
started automatically because it owns its own infinite voice loop.
"""
from __future__ import annotations
import sys
from pathlib import Path
from typing import Any

BASE_DIR = Path(__file__).resolve().parent.parent
JARVIS_ROOT = BASE_DIR / "vendor" / "jarvis_agi"


def available() -> bool:
    return (JARVIS_ROOT / "IMPORTS.py").exists()


def _prepare_imports() -> None:
    root = str(JARVIS_ROOT)
    if root not in sys.path:
        sys.path.insert(0, root)


def get_component(module_path: str) -> Any:
    if not available():
        raise FileNotFoundError(f"JARVIS-AGI source not found: {JARVIS_ROOT}")
    module_path = str(module_path).strip()
    if module_path in {"main", "JARVIS-AGI-main"}:
        raise ValueError("Do not start JARVIS main.py from Brahma Echo.")
    _prepare_imports()
    return __import__(module_path, fromlist=["*"])


def system_theme(theme: int) -> Any:
    mod = get_component("TOOLS.SYSTEM_SETTINGS.system_theme")
    return mod.WindowsThemeManager().set_theme(int(theme))


def taskbar_alignment(alignment: int) -> Any:
    mod = get_component("TOOLS.SYSTEM_SETTINGS.taskbar")
    return mod.TaskbarCustomizer().set_alignment(int(alignment))


def set_temperature_display(enabled: int = 1) -> Any:
    mod = get_component("TOOLS.SYSTEM_SETTINGS.taskbar")
    return mod.TaskbarCustomizer().set_temperature_display(int(enabled))


def metadata() -> dict[str, Any]:
    return {
        "available": available(),
        "root": str(JARVIS_ROOT),
        "entrypoint": str(JARVIS_ROOT / "main.py"),
        "autostart": False,
    }
