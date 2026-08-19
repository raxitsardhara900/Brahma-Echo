# JARVIS-AGI in Brahma Echo

The supplied JARVIS-AGI source is intended to live under `vendor/jarvis_agi`.
Brahma Echo remains the primary application. The original JARVIS `main.py`
is not started automatically because it owns an independent infinite voice loop
and provider stack.

Use `actions/jarvis_agi_adapter.py` for lazy access to individual JARVIS
components. This avoids microphone, audio, provider, and global-state conflicts.

The complete JARVIS source bundle is provided separately as
`Brahma-Echo-JARVIS-Addon.zip` for local merge because the GitHub connector used
here accepts UTF-8 text files, not arbitrary binary archive uploads.
