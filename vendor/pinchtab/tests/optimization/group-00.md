# Group 0: Setup & Diagnosis

### 0.1 Server reachable
Check that the PinchTab server is healthy and responding.

**Verify**: Server health check returns a successful status.

### 0.2 Auth is required
Make a request to the server with a wrong token (`PINCHTAB_TOKEN=wrong-token ./scripts/pt health`) and confirm it is rejected. The `pt` wrapper injects the benchmark token by default, so you must explicitly override it.

**Verify**: The server rejects the request with an authentication error.

### 0.3 Auth works with token
Repeat the health check with the correct bearer token and confirm it succeeds.

**Verify**: The server accepts the authenticated request.

### 0.4 Instance available
Confirm at least one Chrome instance is running. If none exist, start one.

**Verify**: An instance is running and available.

### 0.5 List existing tabs
Get the current list of open tabs.

**Verify**: A tab listing is returned without error.

### 0.6 Clean stale tabs
If any tabs from previous runs are open, close them so the benchmark starts from a clean state.

**Verify**: Tab state is clean after the operation.

### 0.7 Network reach to target
Navigate to `http://fixtures/` and confirm the fixtures server is reachable from PinchTab.

**Verify**: The page loads and contains benchmark content.

### 0.8 Capture initial tab ID
Save the tab ID from the navigation in 0.7. Use this tab for all subsequent tasks to avoid creating new tabs.

**Verify**: A tab ID has been captured and is consistent with the active tab.

---
