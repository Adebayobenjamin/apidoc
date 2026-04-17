# apidoc

A thin wrapper around [Gin](https://github.com/gin-gonic/gin) that generates an OpenAPI 3.0 spec from your Go structs. No comment annotations, no code generation — just define routes and point to your DTOs.

## Installation

```bash
go get github.com/Adebayobenjamin/apidoc@v1.0.0
```

Then import in your code:

```go
import "github.com/Adebayobenjamin/apidoc"
```

Requires Go 1.23+ and Gin v1.10+. No other external dependencies.

## Quick Start

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/Adebayobenjamin/apidoc"
)

func main() {
    r := apidoc.New(gin.Default(), apidoc.Info{
        Title:       "My API",
        Description: "My awesome API.",
        Version:     "1.0.0",
    })

    r.Server("http://localhost:8080", "Local")

    r.GET("/ping", pingHandler).
        Summary("Health check").
        Response(200, nil, "Service is healthy")

    r.ServeDocs("/docs")
    r.Run(":8080")
}
```

Visit `http://localhost:8080/docs` to see the Scalar API reference UI.

## API Reference

### Creating a Router

Wrap a `gin.Engine` with API metadata:

```go
r := apidoc.New(gin.Default(), apidoc.Info{
    Title:       "Auth Service API",
    Description: "Authentication and authorization service.",
    Version:     "1.0.0",
    Contact:     &apidoc.Contact{Name: "Your Name", Email: "you@example.com"},
})
```

### Servers

```go
r.Server("http://localhost/api/v1", "Local")
r.Server("https://api.example.com/api/v1", "Production")
```

### Security Schemes

```go
r.BearerAuth("bearerAuth", "JWT access token from the login endpoint.")
```

### Tags

Register top-level tags with descriptions:

```go
r.AddTag("Authentication", "User authentication and account management endpoints")
r.AddTag("Password Management", "Password reset and recovery endpoints")
```

### Reusable Responses

Define once, reference everywhere:

```go
r.AddResponse("BadRequest", 400, ErrorResponse{}, "Bad request.")
r.AddResponse("Unauthorized", 401, ErrorResponse{}, "Missing or invalid token.")
```

### Reusable Parameters

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

### Defining Routes

`apidoc` supports two patterns for attaching documentation to routes. Both produce the same OpenAPI output — choose whichever fits your style, or mix them in the same project.

#### Option A: Builder Pattern

Chain metadata methods on the return value of `POST`, `GET`, etc.:

```go
auth.POST("/login", loginHandler).
    Summary("Login").
    Description("Authenticate a user and return tokens.").
    Body(LoginRequest{}).
    Response(200, LoginResponse{}, "Login successful").
    ResponseRef(400, "BadRequest")
```

#### Option B: Middleware Pattern

Pass documentation as a no-op middleware using `apidoc.Docs()`:

```go
auth.POST("/login",
    apidoc.Docs(
        apidoc.Summary("Login"),
        apidoc.Description("Authenticate a user and return tokens."),
        apidoc.Body(LoginRequest{}),
        apidoc.Response(200, LoginResponse{}, "Login successful"),
        apidoc.ResponseRef(400, "BadRequest"),
    ),
    loginHandler,
)
```

`apidoc.Docs()` returns a `gin.HandlerFunc` that does nothing at runtime. The router extracts the metadata during route registration.

This pattern is useful when you want to keep documentation in separate files:

```go
// docs/login.go
package docs

import (
    "github.com/Adebayobenjamin/apidoc"
    "github.com/gin-gonic/gin"
)

func Login() gin.HandlerFunc {
    return apidoc.Docs(
        apidoc.Summary("Login"),
        apidoc.Description("Authenticate a user and return tokens."),
        apidoc.Body(LoginRequest{}),
        apidoc.Response(200, LoginResponse{}, "Login successful"),
        apidoc.ResponseRef(400, "BadRequest"),
    )
}

// router.go
auth.POST("/login", docs.Login(), loginHandler)
```

#### Available Doc Options

These functions work with both `apidoc.Docs()` and as methods on `*Endpoint`:

| Function / Method | Description |
|---|---|
| `Summary(s)` | Short summary |
| `Description(d)` | Detailed description |
| `Tags(t...)` | Override group tags |
| `OperationID(id)` | Custom operation ID (auto-generated if omitted) |
| `Body(dto)` | Request body struct |
| `BodyExample(ex)` | Single example for the request body |
| `BodyExamples(map[string]any)` | Named examples (e.g. `"crmLogin"`, `"adminLogin"`) |
| `Response(code, dto, desc)` | Response with struct schema |
| `ResponseRef(code, name)` | Reference a reusable response |
| `PathParam(name, desc, example)` | Document a path parameter |
| `QueryParam(name, type, desc, example)` | Document a query parameter |
| `HeaderParam(name, type, desc, example)` | Document a header parameter |
| `ParamRef(name)` | Reference a reusable parameter |
| `Security(scheme)` | Require a security scheme |
| `Deprecated()` | Mark as deprecated |

### Groups

Groups inherit path prefixes. Use `.Tag()` to set a default tag for all endpoints in the group:

```go
v1 := r.Group("/api/v1")
auth := v1.Group("/auth").Tag("Authentication")

// Builder pattern
auth.POST("/login", loginHandler).
    Summary("Login").
    Body(LoginRequest{}).
    Response(200, LoginResponse{}, "Login successful")

// Middleware pattern
auth.POST("/forgot-password",
    apidoc.Docs(
        apidoc.Summary("Request password reset"),
        apidoc.Body(ForgotPasswordRequest{}),
        apidoc.Response(200, ForgotPasswordResponse{}, "OTP sent"),
    ),
    forgotHandler,
)
```

### Middleware

Middleware passes through to Gin — `apidoc` does not interfere:

```go
// On a group
auth.Use(middleware.RequiresAuth(jwtService, userService))

// Builder pattern with inline middleware
auth.POST("/validate", middleware.RequiresAuth(jwtService, userService), handler).
    Summary("Validate permissions").
    Security("bearerAuth")

// Middleware pattern with inline middleware
auth.POST("/validate",
    apidoc.Docs(
        apidoc.Summary("Validate permissions"),
        apidoc.Security("bearerAuth"),
    ),
    middleware.RequiresAuth(jwtService, userService),
    handler,
)
```

### Generating the Spec

```go
// Write to disk (for CI, version control, etc.)
r.SaveDocs("docs/openapi.json")

// Serve via Gin (Scalar UI + JSON spec)
r.ServeDocs("/docs")
```

`ServeDocs` registers two routes:
- `GET /docs` — Scalar API reference UI
- `GET /docs/openapi.json` — the generated OpenAPI spec

### Starting the Server

```go
r.Run(":8080")
```

You can also access the underlying Gin engine directly:

```go
engine := r.Engine()
```

## Operation IDs

Operation IDs are auto-generated from the HTTP method and path (e.g. `POST /auth/login` → `postAuthLogin`). Override with `.OperationID("custom")` or `apidoc.OperationID("custom")`.

## Schema Generation

Schemas are built by reflecting over the structs you pass to `.Body()` and `.Response()`.

### Struct Tags

```go
type LoginRequest struct {
    Email    string `json:"email"    binding:"required,email" description:"User email"    example:"john@co.com"`
    Password string `json:"password" binding:"required"       description:"User password" example:"Pass123!"`
}
```

| Tag | Purpose | OpenAPI Output |
|---|---|---|
| `json:"name"` | Field name | `properties.name` |
| `json:",omitempty"` | Optional field | Not in `required` array |
| `binding:"required"` | Required field | Added to `required` array |
| `description:"..."` | Field description | `description` |
| `example:"..."` | Example value | `example` |
| `enum:"a,b,c"` | Allowed values | `enum: ["a","b","c"]` |
| `rules:"..."` | Human-readable validation rules | Appended to `description` |

### Go Type Mapping

| Go Type | OpenAPI Type | Format |
|---|---|---|
| `string` | `string` | — |
| `int` / `int32` / `int64` | `integer` | `int32` / `int64` |
| `float32` / `float64` | `number` | `float` / `double` |
| `bool` | `boolean` | — |
| `uuid.UUID` | `string` | `uuid` |
| `time.Time` | `string` | `date-time` |
| `[]T` | `array` | items: T's schema |
| `*T` | T's schema | `nullable: true` |
| Nested struct | `$ref` | `#/components/schemas/StructName` |
| Custom string types | `string` | — |

### Validation Tag Mapping

Tags from `binding` or `validate` are translated automatically:

| Tag | OpenAPI |
|---|---|
| `required` | `required` array |
| `email` | `format: "email"` |
| `uuid` | `format: "uuid"` |
| `url` | `format: "uri"` |
| `min=N` (string) | `minLength: N` |
| `max=N` (string) | `maxLength: N` |
| `min=N` (number) | `minimum: N` |
| `max=N` (number) | `maximum: N` |
| `oneof=a b c` | `enum: ["a","b","c"]` |
| `len=N` | `minLength: N, maxLength: N` |

## Monorepo — Merging Multiple Specs

When you have multiple services each producing their own `openapi.json`, `Merge` combines them into a single spec:

```go
package main

import (
    "log"
    "github.com/Adebayobenjamin/apidoc"
)

func main() {
    combined, err := apidoc.Merge(apidoc.Info{
        Title:   "Platform API",
        Version: "1.0.0",
    },
        apidoc.SourceFile("../auth-service/docs/openapi.json"),
        apidoc.SourceFile("../user-service/docs/openapi.json"),
        apidoc.SourceFile("../billing-service/docs/openapi.json", "billing"),
    )
    if err != nil {
        log.Fatal(err)
    }

    combined.ServeDocs("/docs")
    combined.Run(":9000")
}
```

### Namespace

Each source has a namespace used to resolve schema-name collisions. Pass it explicitly to `SourceFile`:

```go
apidoc.SourceFile("../auth-service/docs/openapi.json", "auth")
```

Or let `apidoc` infer it from the parent directory (e.g. `../auth-service/docs/openapi.json` → `auth-service`).

### Collision Handling

| Conflict | Resolution |
|---|---|
| **Schema name** (e.g. both services define `LoginRequest`) | Prefixed with namespace → `AuthLoginRequest`, `UserServiceLoginRequest`. All `$ref`s get rewritten. Non-colliding schemas keep their original names. |
| **Path + method** (e.g. both services register `POST /health`) | Returns an error from `Merge` |
| **Tag name** | First source wins; duplicates are dropped |
| **Server URL** | Deduped |
| **Reusable response/parameter/security scheme** | Merged by name; later sources override |

### Attaching to an Existing Engine

If you want to serve merged docs alongside other routes:

```go
engine := gin.Default()
combined, _ := apidoc.MergeOn(engine, info,
    apidoc.SourceFile("auth/openapi.json"),
    apidoc.SourceFile("billing/openapi.json"),
)
combined.ServeDocs("/docs")
engine.Run(":8080")
```

## Scalar UI Configuration

Customize the docs UI appearance:

```go
r.ServeDocs("/docs", apidoc.ScalarConfig{
    Theme:             "kepler",
    DarkMode:          false,
    Layout:            "classic",
    DefaultHttpClient: "curl",
    HideModels:        false,
    ShowSidebar:       true,
    CustomCSS:         "body { font-family: monospace; }",
})
```

| Field | Type | Default | Description |
|---|---|---|---|
| `Theme` | `string` | `"purple"` | Scalar theme name |
| `DarkMode` | `bool` | `true` | Force dark mode |
| `Layout` | `string` | `"modern"` | `"modern"` or `"classic"` |
| `DefaultHttpClient` | `string` | `"curl"` | Default code sample language |
| `HideModels` | `bool` | `false` | Hide schema models section |
| `ShowSidebar` | `bool` | `true` | Show navigation sidebar |
| `CustomCSS` | `string` | `""` | Additional CSS to inject |

## Validation: Docs vs Runtime

`apidoc` does **not** execute runtime validation. It only reads struct tags to produce schema constraints.

| Layer | Responsibility | Owned by |
|---|---|---|
| Struct tags (`binding`, `validate`, `enum`) | Schema constraints in the spec | `apidoc` reads these |
| `Validate()` methods | Runtime enforcement in handlers | Your existing code |
| `rules` tag | Human-readable docs for complex rules | `apidoc` reads these |

For rules that tags can't express:

```go
type CreateAccountRequest struct {
    Email    string `json:"email" binding:"required,email" rules:"No personal email domains (gmail, yahoo, etc.)"`
    Password string `json:"password" binding:"required"    rules:"Min 8 chars, uppercase, lowercase, digit, and special char"`
}
```

## Full Example

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/Adebayobenjamin/apidoc"
)

type LoginRequest struct {
    Email    string `json:"email"    binding:"required,email" description:"User email"    example:"john@company.com"`
    Password string `json:"password" binding:"required"       description:"User password" example:"SecurePass123!"`
}

type LoginResponse struct {
    Token   string `json:"token"   description:"JWT access token"  example:"eyJhbGci..."`
    Refresh string `json:"refresh" description:"JWT refresh token" example:"eyJhbGci..."`
}

type ForgotPasswordRequest struct {
    Email string `json:"email" binding:"required,email" description:"Email to send reset OTP to" example:"john@company.com"`
}

type ForgotPasswordResponse struct {
    Sent bool `json:"sent" description:"Whether the OTP was sent" example:"true"`
}

type ErrorResponse struct {
    Status  string `json:"status"  example:"error"`
    Message string `json:"message" example:"invalid credentials"`
}

func main() {
    r := apidoc.New(gin.Default(), apidoc.Info{
        Title:   "Auth Service API",
        Version: "1.0.0",
        Contact: &apidoc.Contact{Name: "Your Name", Email: "you@example.com"},
    })

    r.Server("http://localhost:8080/api/v1", "Local")
    r.BearerAuth("bearerAuth", "JWT access token.")
    r.AddTag("Authentication", "User authentication endpoints")
    r.AddResponse("BadRequest", 400, ErrorResponse{}, "Bad request.")

    auth := r.Group("/api/v1/auth").Tag("Authentication")

    // Builder pattern — chain methods after the route
    auth.POST("/login", loginHandler).
        Summary("Login").
        Description("Authenticate a user and return tokens.").
        Body(LoginRequest{}).
        Response(200, LoginResponse{}, "Login successful").
        ResponseRef(400, "BadRequest")

    // Middleware pattern — pass docs as a handler
    auth.POST("/forgot-password",
        apidoc.Docs(
            apidoc.Summary("Request password reset"),
            apidoc.Description("Sends an OTP to the user's email."),
            apidoc.Body(ForgotPasswordRequest{}),
            apidoc.Response(200, ForgotPasswordResponse{}, "OTP sent"),
            apidoc.ResponseRef(400, "BadRequest"),
        ),
        forgotPasswordHandler,
    )

    r.ServeDocs("/docs")
    r.Run(":8080")
}
```
