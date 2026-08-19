# Group 21: Async / awaitPromise

### 21.1 Await a promise-returning function
Navigate to `http://fixtures/async.html`. The page exposes `window.fetchPayload()` which returns a Promise. Call it and retrieve the resolved value (not the Promise wrapper).

**Verify**: Report includes the resolved payload value.

### 21.2 Await a promise resolving to an object
On the same page, call `window.fetchUser()` and retrieve the resolved object. Report the user's name field.

**Verify**: Report includes the user name from the resolved object.

---
