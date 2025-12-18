This is a reformatted, professional `README.md` file based on the content provided. It uses standard Markdown practices to make the guide readable and easy to navigate.

---

# JetBrains HTTP Client Guide

The **JetBrains HTTP Client** allows you to create, edit, and execute HTTP requests directly within your IDE (IntelliJ IDEA, WebStorm, PyCharm, etc.) using simple `.http` or `.rest` files.

These files are **text-based** and **version-controllable**, making them a powerful alternative to GUI-based tools like Postman for team collaboration.

---

## 🛠 Core Syntax

- **Separators**: Use `###` to separate distinct requests within a single file.
- **Variables**: Use `{{variableName}}` to inject environment or dynamic variables.
- **Scripting**: Use `> {% ... %}` blocks to execute JavaScript for response handling, assertions, or data extraction.

---

## ⚡ Quick Start: The Cheatsheet

Create a file named `requests.http` and paste the following example to see authentication, token extraction, and data reuse in action.

```http
@hostname = api.example.com

### 1. Simple GET Request
GET https://{{hostname}}/health
Accept: application/json

### 2. POST Request (Login & Token Extraction)
# @name login_request
POST https://{{hostname}}/auth/login
Content-Type: application/json

{
  "username": "admin",
  "password": "secret123"
}

> {% 
    // SCRIPT: Extract token from response and save to global variable
    client.test("Request executed successfully", function() {
        client.assert(response.status === 200, "Response status is not 200");
    });

    // Save the 'token' field from JSON body to 'auth_token' variable
    client.global.set("auth_token", response.body.token);
    client.log("Token saved: " + response.body.token);
%}

### 3. Authenticated Request (Using Extracted Data)
GET https://{{hostname}}/users/me
Authorization: Bearer {{auth_token}}
Accept: application/json

> {%
    client.test("User is admin", function() {
        client.assert(response.body.role === "ADMIN", "User is not admin");
    });
%}
```

---

## 📜 Scripting & Data Extraction

The HTTP Client includes a JavaScript runtime for pre-request and response handling. You can access the `client` and `response` objects inside the scripting blocks.

### Managing Variables
| Action | Syntax |
| :--- | :--- |
| **Save Variable** | `client.global.set("myVar", response.body.id);` |
| **Read Variable** | `client.global.get("myVar");` |

### Accessing Data
*   **JSON Body**: Use `response.body` to access parsed JSON (e.g., `response.body.users[0].name`).
*   **Headers**: Use `response.headers.valueOf("Content-Type")`.

---

## ✅ Testing & Assertions

You can write automated tests that execute immediately after a request. Results are displayed in the **Tests** tab of the IDE's Run tool window.

```javascript
client.test("Validation Check", function() {
    // 1. Check Status Code
    client.assert(response.status === 200, "Expected 200 OK");
    
    // 2. Check Content Type
    var type = response.contentType.mimeType;
    client.assert(type === "application/json", "Expected JSON");
    
    // 3. Check Response Body Content
    client.assert(response.body.items.length > 0, "List should not be empty");
});
```

---

## 💡 Benefits of .http Files
- **Version Control**: Keep your API tests in Git alongside your source code.
- **Speed**: No need to switch windows to an external application.
- **Documentation**: Acts as living documentation for your API endpoints.
- **Environment Support**: Easily switch between `development`, `staging`, and `production` using `http-client.env.json` files.