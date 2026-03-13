# apidoc — Design Document

A thin wrapper around Gin that collects route metadata and generates an OpenAPI 3.0 spec from your Go structs. No comments, no code generation step — just define routes and point to your DTOs.

---

## Problem

Most Go doc tools (swaggo, go-swagger) rely on comment annotations above handlers. This means:
- Docs live in comments, not in code
- Comments drift out of sync with actual request/response types
- You maintain two sources of truth

## Solution

`apidoc` wraps `gin.Engine` so that every route registration also captures documentation metadata. Schemas are generated via **reflection** on your existing DTOs — the structs _are_ the docs.

---

## Core API

### 1. Create a Router

```go
r := apidoc.New(gin.Default(), apidoc.Info{
    Title:       "Auth Service API",
    Description: "Authentication and authorization service.",
    Version:     "1.0.0",
    Contact:     &apidoc.Contact{Name: "BFree Africa", Email: "support@bfree.africa"},
})
```

### 2. Add Servers

```go
r.Server("http://localhost/api/v1", "Local")
r.Server("https://api.example.com/api/v1", "Production")
```

### 3. Add Security Schemes

```go
r.BearerAuth("bearerAuth", "JWT access token from the login endpoint.")
```

### 4. Register Reusable Responses

```go
r.AddResponse("BadRequest", 400, dtos.ErrorResponse{}, "Bad request.")
r.AddResponse("Unauthorized", 401, dtos.ErrorResponse{}, "Missing or invalid token.")
```

### 5. Register Reusable Parameters

```go
r.AddParam("XUserDomain", apidoc.Param{
    Name:        "X-User-Domain",
    In:          "header",
    Description: "User domain — crm or admin.",
    Required:    true,
    Type:        "string",
    Enum:        []string{"admin", "crm"},
    Example:     "crm",
})
```

Then reference from any endpoint:
```go
auth.POST("/login", handler).
    ParamRef("XUserDomain")
```

### 6. Add Tags with Descriptions

```go
r.AddTag("Authentication", "User authentication and account management endpoints")
r.AddTag("Password Management", "Password reset and recovery endpoints")
r.AddTag("OTP", "One-time password verification endpoints")
```

Tags added here appear in the spec's top-level `tags` array with descriptions. When a group uses `.Tag("Authentication")`, it references the same tag.

### 7. Define Routes (same as Gin, with chained metadata)

```go
auth := r.Group("/auth").Tag("Authentication")

auth.POST("/login", authHandler.Login).
    Summary("Login").
    Description("Authenticate a user and return tokens.").
    Body(dtos.LoginRequest{}).
    Response(200, dtos.LoginCrmResponse{}, "Login successful").
    ResponseRef(400, "BadRequest").
    ResponseRef(422, "UnprocessableEntity")
```

### 8. Groups

```go
v1 := r.Group("/api/v1")
auth := v1.Group("/auth").Tag("Authentication")
passwords := v1.Group("/auth").Tag("Password Management")
```

- Groups inherit the parent's path prefix
- `.Tag()` sets a default tag for all endpoints in the group
- Endpoints can override tags with `.Tags("Custom")`

### 9. Middleware (pass-through to Gin)

```go
auth.Use(middleware.RequiresAuth(jwtService, userService))

// Or inline, same as Gin:
auth.POST("/validate", middleware.RequiresAuth(jwtService, userService), handler).
    Summary("Validate permissions").
    Security("bearerAuth")
```

### 10. Generate Docs & Serve UI

```go
// Write the spec to disk (e.g. for CI, version control)
r.SaveDocs("docs/openapi.json")

// Serve Scalar UI + spec on the running server
r.ServeDocs("/docs")
```

`ServeDocs("/docs")` registers two Gin routes:
- `GET /docs` → Scalar API reference UI (loaded from CDN)
- `GET /docs/openapi.json` → the generated OpenAPI spec

This replaces the need for a separate `docs/` package with embedded files.

**Default Scalar config:** purple theme, dark mode, modern layout, curl as default HTTP client.

Override with:
```go
r.ServeDocs("/docs", apidoc.ScalarConfig{
    Theme:    "kepler",
    DarkMode: false,
    Layout:   "classic",
})
```

`ScalarConfig` options:

| Field | Type | Default | Description |
|---|---|---|---|
| `Theme` | `string` | `"purple"` | Scalar theme name |
| `DarkMode` | `bool` | `true` | Force dark mode |
| `Layout` | `string` | `"modern"` | `"modern"` or `"classic"` |
| `DefaultHttpClient` | `string` | `"curl"` | Default code sample language |
| `HideModels` | `bool` | `false` | Hide schema models section |
| `ShowSidebar` | `bool` | `true` | Show navigation sidebar |
| `CustomCSS` | `string` | `""` | Additional CSS to inject |

### 11. Start Server (pass-through to Gin)

```go
r.Run(":8080")
```

---

## Endpoint Builder — Full API

Every route method (`POST`, `GET`, `PUT`, `DELETE`, `PATCH`) returns an `*Endpoint` for chaining:

| Method | Description |
|---|---|
| `.Summary(s)` | Short summary of the endpoint |
| `.Description(d)` | Detailed description |
| `.Tags(t...)` | Override group tags |
| `.OperationID(id)` | Custom operation ID (auto-generated if omitted) |
| `.Body(dto)` | Request body struct |
| `.BodyExample(ex)` | Single example for the request body |
| `.BodyExamples(map[string]any)` | Named examples for the request body (e.g. `"crmLogin"`, `"adminLogin"`) |
| `.Response(code, dto, desc)` | Response with struct schema |
| `.ResponseRef(code, name)` | Reference a reusable response |
| `.ResponseExample(code, ex)` | Single example for a response |
| `.ResponseExamples(code, map[string]any)` | Named examples for a response |
| `.PathParam(name, desc, example)` | Document a path parameter |
| `.QueryParam(name, type, desc, example)` | Document a query parameter |
| `.HeaderParam(name, type, desc, example)` | Document a header parameter |
| `.ParamRef(name)` | Reference a reusable parameter |
| `.Security(scheme)` | Require a security scheme |
| `.Deprecated()` | Mark as deprecated |

---

## Schema Generation via Reflection

Schemas are built by reflecting over the structs you pass to `.Body()` and `.Response()`. No manual schema definitions needed.

### Struct Tags Supported

```go
type LoginRequest struct {
    Email    string `json:"email"    binding:"required,email" description:"User email"    example:"john@co.com"`
    Password string `json:"password" binding:"required"       description:"User password" example:"Pass123!"`
}
```

| Tag | Purpose | OpenAPI output |
|---|---|---|
| `json:"name"` | Field name in schema | `properties.name` |
| `json:",omitempty"` | Field is optional | Not in `required` array |
| `binding:"required"` | Field is required | Added to `required` array |
| `binding:"email"` | Email format | `format: "email"` |
| `description:"..."` | Field description | `description` |
| `example:"..."` | Field example value | `example` |
| `enum:"a,b,c"` | Allowed values | `enum: ["a","b","c"]` |

### Go Type Mapping

| Go Type | OpenAPI Type | Format |
|---|---|---|
| `string` | `string` | — |
| `int`, `int32`, `int64` | `integer` | `int32` / `int64` |
| `float32`, `float64` | `number` | `float` / `double` |
| `bool` | `boolean` | — |
| `uuid.UUID` | `string` | `uuid` |
| `time.Time` | `string` | `date-time` |
| `[]T` | `array` | items: T's schema |
| `*T` | T's schema | `nullable: true` |
| Nested struct | `$ref` | `#/components/schemas/StructName` |

### Validation Tag Mapping

Tags from `binding` or `validate` are translated:

| Tag | OpenAPI |
|---|---|
| `required` | `required` array |
| `email` | `format: "email"` |
| `uuid` | `format: "uuid"` |
| `min=N` (string) | `minLength: N` |
| `max=N` (string) | `maxLength: N` |
| `min=N` (number) | `minimum: N` |
| `max=N` (number) | `maximum: N` |
| `oneof=a b c` | `enum: ["a","b","c"]` |
| `len=N` | `minLength: N, maxLength: N` |

---

## Package Structure

```
apidoc/
├── DESIGN.md       # This document
├── apidoc.go       # Router, Group, New(), Server(), SaveDocs(), Run()
├── endpoint.go     # Endpoint builder (fluent chain methods)
├── schema.go       # Struct → OpenAPI schema via reflection
├── generate.go     # Assembles the full OpenAPI spec and writes JSON
└── scalar.go       # ServeDocs(), ScalarConfig, Scalar HTML template
```

### Dependencies

- `github.com/gin-gonic/gin` — wraps the router
- Standard library only (`reflect`, `encoding/json`, `os`, `strings`)
- **No** kin-openapi or other OpenAPI libraries — we output raw JSON

---

## Validation — Docs vs Runtime

`apidoc` does **not** run your `Validate()` methods or execute any runtime validation. It only reads struct tags to produce schema constraints in the spec.

| Layer | Responsibility | Owned by |
|---|---|---|
| **Struct tags** (`binding`, `validate`, `enum`) | Schema constraints in the spec | `apidoc` reads these |
| **`Validate()` methods** | Runtime enforcement in handlers | Your existing code |
| **`rules` tag** | Human-readable docs for complex rules tags can't express | `apidoc` reads these |

For validation rules that struct tags can't express, use the `rules` tag:

```go
type InitiateAccountCreationRequest struct {
    Email    string `json:"email" binding:"required,email" rules:"No personal email domains (gmail, yahoo, etc.)"`
    Password string `json:"password" binding:"required"    rules:"Min 8 chars, uppercase, lowercase, digit, and special char"`
}
```

The `rules` tag value is appended to the field's `description` in the OpenAPI schema.

---

## What It Is NOT

- **Not a routing engine** — Gin handles all routing, middleware, and request handling
- **Not a runtime validator** — your existing `binding` tags and `Validate()` methods still do that
- **Not a comment parser** — no `// @Summary` annotations

---

## Migration Path (auth-service example)

**Before:**
```go
func AuthRouter(router *gin.RouterGroup, ...) {
    authRouter := router.Group("/auth")
    authRouter.POST("/login", authHandler.Login)
    authRouter.POST("/forgot-password", authHandler.ForgotPassword)
}
```

**After:**
```go
func AuthRouter(router *apidoc.Group, ...) {
    auth := router.Group("/auth").Tag("Authentication")

    auth.POST("/login", authHandler.Login).
        Summary("Login").
        Description("Authenticate a user and return tokens.").
        Body(dtos.LoginRequest{}).
        Response(200, dtos.LoginCrmResponse{}, "Login successful").
        ResponseRef(401, "Unauthorized")

    auth.POST("/forgot-password", authHandler.ForgotPassword).
        Summary("Request password reset").
        Body(dtos.ForgotPasswordRequest{}).
        Response(200, dtos.ForgotPasswordResponse{}, "OTP sent").
        ResponseRef(400, "BadRequest")
}
```

The only change: `*gin.RouterGroup` → `*apidoc.Group`, and chain metadata after each route.

---

## Decisions

1. **Scalar UI** — `ServeDocs()` handles both spec serving and Scalar UI. No separate `docs/` package needed.
2. **operationID** — Auto-generated from method + path (e.g. `POST /auth/login` → `postAuthLogin`). Can be overridden with `.OperationID("custom")`.
3. **Group tags** — Child groups use their own tags only, not inherited from parent. Keeps things explicit.
