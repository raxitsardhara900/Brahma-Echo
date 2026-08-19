# Group 4: Authentication

### 4.1 Fill login form
Navigate to `http://fixtures/login.html` and fill in username="testuser" and password="wrongpass".

**Verify**: Both fields contain the entered values.

### 4.2 Submit with validation errors
Click the "Sign In" button to submit with invalid credentials.

**Verify**: Error message "INVALID_CREDENTIALS_ERROR" appears.

### 4.3 Successful login
Clear the fields, enter username="admin" and password="password123", then submit.

**Verify**: Login succeeds and shows "VERIFY_LOGIN_SUCCESS_DASHBOARD".

---
