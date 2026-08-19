import asyncio
import os
import threading
import concurrent.futures
import platform
import shutil
import subprocess
from pathlib import Path
from playwright.async_api import async_playwright, TimeoutError as PlaywrightTimeout


def _log(message: str) -> None:
    try:
        print(message.encode("ascii", "replace").decode("ascii"))
    except UnicodeEncodeError:
        print(message.encode("ascii", "replace").decode("ascii"))


def _get_default_browser_id() -> str:
    """Returns raw default browser identifier string for current OS."""
    system = platform.system()
    try:
        if system == "Windows":
            import winreg
            key = winreg.OpenKey(
                winreg.HKEY_CURRENT_USER,
                r"Software\Microsoft\Windows\Shell\Associations\UrlAssociations\http\UserChoice"
            )
            prog_id = winreg.QueryValueEx(key, "ProgId")[0].lower()
            winreg.CloseKey(key)
            return prog_id
        elif system == "Darwin":
            result = subprocess.run(
                ["defaults", "read", "com.apple.LaunchServices/com.apple.launchservices.secure", "LSHandlers"],
                capture_output=True, text=True, timeout=5
            )
            return result.stdout.lower()
        elif system == "Linux":
            result = subprocess.run(
                ["xdg-settings", "get", "default-web-browser"],
                capture_output=True, text=True, timeout=5
            )
            return result.stdout.lower()
    except Exception:
        pass
    return ""


_BROWSER_BINARIES = {
    "Windows": {
        "opera": ["opera.exe"], "brave": ["brave.exe"], "vivaldi": ["vivaldi.exe"],
        "chrome": ["chrome.exe"], "firefox": ["firefox.exe"],
    },
    "Darwin": {
        "opera": ["opera"], "brave": ["brave browser", "brave"], "vivaldi": ["vivaldi"],
        "chrome": ["google chrome", "google-chrome"], "firefox": ["firefox"],
    },
    "Linux": {
        "opera": ["opera", "opera-stable"], "brave": ["brave-browser", "brave"],
        "vivaldi": ["vivaldi-stable", "vivaldi"],
        "chrome": ["google-chrome", "google-chrome-stable", "chromium-browser", "chromium"],
        "firefox": ["firefox"],
    },
}


def _get_opera_executable() -> str | None:
    if platform.system() != "Windows":
        return None
    try:
        import winreg
        candidate_keys = [
            r"SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\opera.exe",
            r"SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\launcher.exe",
            r"SOFTWARE\Clients\StartMenuInternet\OperaStable\shell\open\command",
            r"SOFTWARE\Clients\StartMenuInternet\OperaGXStable\shell\open\command",
        ]
        for key_path in candidate_keys:
            for hive in [winreg.HKEY_LOCAL_MACHINE, winreg.HKEY_CURRENT_USER]:
                try:
                    key = winreg.OpenKey(hive, key_path)
                    val = winreg.QueryValue(key, None)
                    winreg.CloseKey(key)
                    exe = val.strip().strip('"').split('"')[0].split(" --")[0].strip()
                    if exe and Path(exe).exists():
                        _log(f"[Browser] Opera found via registry: {exe}")
                        return exe
                except Exception:
                    continue
    except Exception:
        pass
    return None


def _find_browser_executable(prog_id: str) -> tuple:
    system = platform.system()
    os_bins = _BROWSER_BINARIES.get(system, {})
    if any(x in prog_id for x in ["firefox", "mozilla"]):
        return "firefox", None, None, False
    if "safari" in prog_id:
        return "webkit", None, None, False
    if "edge" in prog_id:
        return "chromium", None, "msedge", False
    if "opera" in prog_id:
        exe = _get_opera_executable()
        if exe:
            return "chromium", exe, None, True
        for binary in os_bins.get("opera", []):
            path = shutil.which(binary)
            if path:
                return "chromium", path, None, True
    browser_patterns = {"brave": ["brave"], "vivaldi": ["vivaldi"], "chrome": ["chrome"]}
    for browser_name, patterns in browser_patterns.items():
        if not any(p in prog_id for p in patterns):
            continue
        for binary in os_bins.get(browser_name, []):
            path = shutil.which(binary)
            if path:
                _log(f"[Browser] Found {browser_name} at: {path}")
                return "chromium", path, None, False
    if "chrome" in prog_id or not prog_id:
        return "chromium", None, "chrome", False
    return "chromium", None, None, False


class _BrowserThread:
    def __init__(self):
        self._loop = None
        self._thread = None
        self._ready = threading.Event()
        self._playwright = None
        self._browser = None
        self._context = None
        self._page = None
        self._pages = []
        self._engine_name = "chromium"
        self._exe_path = None
        self._channel = None
        self._is_opera = False

    def start(self):
        if self._thread and self._thread.is_alive():
            return
        self._thread = threading.Thread(target=self._run_loop, daemon=True, name="BrowserThread")
        self._thread.start()
        self._ready.wait(timeout=15)

    def _run_loop(self):
        self._loop = asyncio.new_event_loop()
        asyncio.set_event_loop(self._loop)
        self._loop.run_until_complete(self._init())
        self._ready.set()
        self._loop.run_forever()

    async def _init(self):
        self._playwright = await async_playwright().start()

    def run(self, coro, timeout: int = 30):
        if not self._loop:
            raise RuntimeError("BrowserThread not started.")
        future = asyncio.run_coroutine_threadsafe(coro, self._loop)
        return future.result(timeout=timeout)

    async def _launch_browser_if_needed(self):
        if self._browser and self._browser.is_connected():
            return
        prog_id = _get_default_browser_id()
        self._engine_name, self._exe_path, self._channel, self._is_opera = _find_browser_executable(prog_id)
        engine = getattr(self._playwright, self._engine_name)
        chromium_args = ["--start-maximized"]
        if self._is_opera:
            chromium_args += ["--disable-features=OperaPrivacyMode", "--no-private"]
        launch_kwargs = {"headless": False}
        if self._engine_name == "chromium":
            launch_kwargs["args"] = chromium_args
        if self._exe_path:
            launch_kwargs["executable_path"] = self._exe_path
        elif self._channel:
            launch_kwargs["channel"] = self._channel
        try:
            self._browser = await engine.launch(**launch_kwargs)
            _log(f"[Browser] Launched ({self._engine_name}{' / ' + self._channel if self._channel else ''}{' / ' + self._exe_path if self._exe_path else ''})")
        except Exception as e:
            _log(f"[Browser] Launch failed ({e}), falling back to built-in Chromium")
            self._browser = await self._playwright.chromium.launch(headless=False, args=["--start-maximized"])

    async def _get_page(self):
        await self._launch_browser_if_needed()
        if self._context is None:
            self._context = await self._browser.new_context(
                viewport=None,
                user_agent=(
                    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
                    "AppleWebKit/537.36 (KHTML, like Gecko) "
                    "Chrome/120.0.0.0 Safari/537.36"
                )
            )
        if self._page is None or self._page.is_closed():
            self._page = await self._context.new_page()
            if self._page not in self._pages:
                self._pages.append(self._page)
        return self._page

    async def _new_tab(self, url: str | None = None) -> str:
        await self._launch_browser_if_needed()
        if self._context is None:
            self._context = await self._browser.new_context(viewport=None)
        page = await self._context.new_page()
        if page not in self._pages:
            self._pages.append(page)
        self._page = page
        if url:
            return await self._go_to(url)
        return f"Opened new tab ({len(self._pages)} total)."

    async def _switch_tab(self, index: int = 1) -> str:
        await self._launch_browser_if_needed()
        if not self._pages:
            await self._get_page()
        idx = max(1, index) - 1
        if idx >= len(self._pages):
            return f"Tab {index} does not exist."
        page = self._pages[idx]
        if page.is_closed():
            return f"Tab {index} is already closed."
        self._page = page
        try:
            await page.bring_to_front()
        except Exception:
            pass
        return f"Switched to tab {index}: {page.url}"

    async def _list_tabs(self) -> str:
        await self._launch_browser_if_needed()
        if not self._pages and self._page:
            self._pages = [self._page]
        if not self._pages:
            return "No tabs open."
        rows = []
        for i, page in enumerate(self._pages, 1):
            if page.is_closed():
                continue
            title = await page.title() if page.url else ""
            rows.append(f"{i}. {title or 'Untitled'} | {page.url or 'about:blank'}")
        return "\n".join(rows) if rows else "No open tabs."

    async def _back(self) -> str:
        page = await self._get_page()
        try:
            await page.go_back(wait_until="domcontentloaded", timeout=10000)
            return f"Back: {page.url}"
        except Exception as e:
            return f"Back error: {e}"

    async def _forward(self) -> str:
        page = await self._get_page()
        try:
            await page.go_forward(wait_until="domcontentloaded", timeout=10000)
            return f"Forward: {page.url}"
        except Exception as e:
            return f"Forward error: {e}"

    async def _reload(self) -> str:
        page = await self._get_page()
        try:
            await page.reload(wait_until="domcontentloaded", timeout=15000)
            return f"Reloaded: {page.url}"
        except Exception as e:
            return f"Reload error: {e}"

    async def _go_to(self, url: str) -> str:
        if not url.startswith("http"):
            url = "https://" + url
        page = await self._get_page()
        try:
            await page.goto(url, wait_until="domcontentloaded", timeout=15000)
            return f"Opened: {page.url}"
        except PlaywrightTimeout:
            return f"Timeout loading: {url}"
        except Exception as e:
            return f"Navigation error: {e}"

    async def _search(self, query: str, engine: str = "google") -> str:
        from urllib.parse import quote_plus
        engines = {
            "google": f"https://www.google.com/search?q={quote_plus(query)}",
            "bing": f"https://www.bing.com/search?q={quote_plus(query)}",
            "duckduckgo": f"https://duckduckgo.com/?q={quote_plus(query)}",
        }
        url = engines.get(engine.lower(), engines["google"])
        return await self._go_to(url)

    async def _click(self, selector=None, text=None) -> str:
        page = await self._get_page()
        try:
            if text:
                await page.get_by_text(text, exact=False).first.click(timeout=8000)
                return f"Clicked: '{text}'"
            elif selector:
                await page.click(selector, timeout=8000)
                return f"Clicked: {selector}"
            return "No selector or text provided."
        except PlaywrightTimeout:
            return "Element not found or not clickable."
        except Exception as e:
            return f"Click error: {e}"

    async def _type(self, selector=None, text: str = "", clear_first: bool = True) -> str:
        page = await self._get_page()
        try:
            element = page.locator(selector).first if selector else page.locator(":focus")
            if clear_first:
                await element.clear()
            await element.type(text, delay=50)
            return "Text typed."
        except Exception as e:
            return f"Type error: {e}"

    async def _scroll(self, direction: str = "down", amount: int = 500) -> str:
        page = await self._get_page()
        try:
            y = amount if direction == "down" else -amount
            await page.mouse.wheel(0, y)
            return f"Scrolled {direction}."
        except Exception as e:
            return f"Scroll error: {e}"

    async def _press(self, key: str) -> str:
        page = await self._get_page()
        try:
            await page.keyboard.press(key)
            return f"Pressed: {key}"
        except Exception as e:
            return f"Key error: {e}"

    async def _get_text(self) -> str:
        page = await self._get_page()
        try:
            text = await page.inner_text("body")
            return text[:4000] if len(text) > 4000 else text
        except Exception as e:
            return f"Could not get page text: {e}"

    async def _fill_form(self, fields: dict) -> str:
        page = await self._get_page()
        results = []
        for selector, value in fields.items():
            try:
                el = page.locator(selector).first
                await el.clear()
                await el.type(str(value), delay=40)
                results.append(f"✓ {selector}")
            except Exception as e:
                results.append(f"✗ {selector}: {e}")
        return "Form filled: " + ", ".join(results)

    async def _smart_click(self, description: str) -> str:
        page = await self._get_page()
        desc_lower = description.lower()
        role_hints = {
            "button": ["button", "buton", "btn"],
            "link": ["link", "bağlantı"],
            "searchbox": ["search", "arama"],
            "textbox": ["input", "field", "alan"],
        }
        for role, keywords in role_hints.items():
            if any(k in desc_lower for k in keywords):
                try:
                    await page.get_by_role(role).first.click(timeout=5000)
                    return f"Clicked ({role}): '{description}'"
                except Exception:
                    pass
        for func in (
            lambda: page.get_by_text(description, exact=False).first.click(timeout=5000),
            lambda: page.get_by_placeholder(description, exact=False).first.click(timeout=5000),
        ):
            try:
                await func()
                return f"Clicked: '{description}'"
            except Exception:
                pass
        return f"Could not find: '{description}'"

    async def _smart_type(self, description: str, text: str) -> str:
        page = await self._get_page()
        for method, locator in [
            ("placeholder", page.get_by_placeholder(description, exact=False)),
            ("label", page.get_by_label(description, exact=False)),
            ("role", page.get_by_role("textbox")),
        ]:
            try:
                el = locator.first
                await el.clear()
                await el.type(text, delay=50)
                return f"Typed into ({method}): '{description}'"
            except Exception:
                continue
        return f"Could not find input: '{description}'"

    async def _close_browser(self) -> str:
        if self._browser:
            await self._browser.close()
            self._browser = None
            self._context = None
            self._page = None
            self._pages = []
        if self._playwright:
            await self._playwright.stop()
            self._playwright = None
        return "Browser closed."


_bt = _BrowserThread()
_bt_started = False
_bt_lock = threading.Lock()


def _ensure_started():
    global _bt_started
    with _bt_lock:
        if not _bt_started:
            _bt.start()
            _bt_started = True


def _dispatch_pinchtab(parameters: dict) -> str | None:
    """Route browser interaction to PinchTab by default; direct navigation remains system-browser unless PinchTab is explicitly selected."""
    action = str((parameters or {}).get("action", "")).lower().strip()
    forced_pinchtab = os.environ.get("BRAHMA_BROWSER_BACKEND", "").strip().lower() == "pinchtab"

    supported = {
        "go_to", "navigate", "search", "click", "fill", "smart_click", "smart_type",
        "snapshot", "get_text", "press", "screenshot", "list_tabs", "tabs", "health", "server_start"
    }
    if action not in supported:
        return None

    # Browser actions are now PinchTab-first. This prevents search/click/type/press
    # requests from silently falling back to a separate Playwright Chrome session.
    # Plain navigation keeps the system/default browser unless PinchTab is explicitly selected.
    interaction_actions = {
        "search", "click", "fill", "smart_click", "smart_type", "press",
        "snapshot", "get_text", "screenshot", "list_tabs", "tabs", "health", "server_start"
    }
    if action not in interaction_actions and not forced_pinchtab:
        return None

    try:
        from actions import pinchtab_client as pt
        result = pt.browser_control(parameters)
        _log(f"[PinchTab] primary browser action: {action}")
        return result
    except Exception as exc:
        _log(f"[PinchTab] action failed: {action}: {exc}")
        return None


def browser_control(parameters: dict, response=None, player=None, session_memory=None) -> str:
    """Browser controller; PinchTab handles interaction/search by default and can own navigation when selected."""
    routed = _dispatch_pinchtab(parameters or {})
    if routed is not None:
        if player:
            player.write_log(f"[pinchtab] {routed[:60]}")
        return routed

    _ensure_started()
    action = (parameters or {}).get("action", "").lower().strip()
    result = "Unknown action."
    try:
        if action in {"go_to", "navigate"}:
            result = _bt.run(_bt._go_to(parameters.get("url", "")))
        elif action == "search":
            result = _bt.run(_bt._search(parameters.get("query", ""), parameters.get("engine", "google")))
        elif action == "click":
            result = _bt.run(_bt._click(selector=parameters.get("selector"), text=parameters.get("text")))
        elif action == "type":
            result = _bt.run(_bt._type(selector=parameters.get("selector"), text=parameters.get("text", ""), clear_first=parameters.get("clear_first", True)))
        elif action == "scroll":
            result = _bt.run(_bt._scroll(direction=parameters.get("direction", "down"), amount=parameters.get("amount", 500)))
        elif action == "fill_form":
            result = _bt.run(_bt._fill_form(parameters.get("fields", {})))
        elif action == "smart_click":
            result = _bt.run(_bt._smart_click(parameters.get("description", "")))
        elif action == "smart_type":
            result = _bt.run(_bt._smart_type(parameters.get("description", ""), parameters.get("text", "")))
        elif action == "get_text":
            result = _bt.run(_bt._get_text())
        elif action == "press":
            result = _bt.run(_bt._press(parameters.get("key", "Enter")))
        elif action in {"open_tab", "new_tab"}:
            result = _bt.run(_bt._new_tab(parameters.get("url")))
        elif action == "switch_tab":
            result = _bt.run(_bt._switch_tab(int(parameters.get("tab", 1))))
        elif action == "list_tabs":
            result = _bt.run(_bt._list_tabs())
        elif action == "back":
            result = _bt.run(_bt._back())
        elif action == "forward":
            result = _bt.run(_bt._forward())
        elif action in {"refresh", "reload"}:
            result = _bt.run(_bt._reload())
        elif action == "close":
            result = _bt.run(_bt._close_browser())
        else:
            result = f"Unknown action: {action}"
    except concurrent.futures.TimeoutError:
        result = "Browser action timed out."
    except Exception as e:
        result = f"Browser error: {e}"

    _log(f"[Browser] {result[:80]}")
    if player:
        player.write_log(f"[browser] {result[:60]}")
    return result