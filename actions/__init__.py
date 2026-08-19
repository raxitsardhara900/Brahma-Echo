"""Action modules for Brahma AI.

Browser automation is a core Brahma capability. Keep PinchTab selected for
browser-control requests by default so the runtime does not silently fall back
to a separate Playwright/system-browser session. The backend can still be
changed explicitly with BRAHMA_BROWSER_BACKEND when needed.
"""

import os

os.environ.setdefault("BRAHMA_BROWSER_BACKEND", "pinchtab")
