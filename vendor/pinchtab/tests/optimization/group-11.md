# Group 11: State Persistence & Page Reload

### 11.1 Add an item and verify after page reload
Go to `http://fixtures/spa.html?reset=1` to start with a clean state. Add a task titled "Persistent Task Test". Then navigate away to `http://fixtures/` and back to `http://fixtures/spa.html` (without the reset param) to check whether the task persisted.

**Verify**: After navigating back, is "Persistent Task Test" in the task list?
- A) Yes, the task is still in the list
- B) No, the task is gone
- C) Other

### 11.2 Logout and log back in
From the logged-in dashboard, click Sign Out. Then log in again with username "benchmark" and password "test456".

**Verify**: After logging back in, what do you see?
- A) The dashboard is shown
- B) Login failed or an error appeared
- C) Other

---
