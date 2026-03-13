# apidoc

A thin wrapper around [Gin](https://github.com/gin-gonic/gin) that generates an OpenAPI 3.0 spec from your Go structs. No comment annotations, no code generation — just define routes and point to your DTOs.

## Installation

```go
import "github.com/Adebayobenjamin/apidoc"
```

No external dependencies beyond Gin and the Go standard library.

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

Routes work exactly like Gin, with chained metadata:

```go
r.POST("/login", loginHandler).
    Summary("Login").
    Description("Authenticate a user and return tokens.").
    Body(LoginRequest{}).
    Response(200, LoginResponse{}, "Login successful").
    ResponseRef(400, "BadRequest")
```

### Groups

Groups inherit path prefixes. Use `.Tag()` to set a default tag for all endpoints in the group:

```go
v1 := r.Group("/api/v1")
auth := v1.Group("/auth").Tag("Authentication")

auth.POST("/login", loginHandler).
    Summary("Login").
    Body(LoginRequest{}).
    Response(200, LoginResponse{}, "Login successful")

auth.POST("/forgot-password", forgotHandler).
    Summary("Request password reset").
    Body(ForgotPasswordRequest{}).
    Response(200, ForgotPasswordResponse{}, "OTP sent")
```

### Middleware

Middleware passes through to Gin — `apidoc` does not interfere:

```go
// On a group
auth.Use(middleware.RequiresAuth(jwtService, userService))

// Or inline on a route
auth.POST("/validate", middleware.RequiresAuth(jwtService, userService), handler).
    Summary("Validate permissions").
    Security("bearerAuth")
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

## Endpoint Builder

Every route method (`POST`, `GET`, `PUT`, `DELETE`, `PATCH`) returns an `*Endpoint` for chaining:

| Method | Description |
|---|---|
| `.Summary(s)` | Short summary |
| `.Description(d)` | Detailed description |
| `.Tags(t...)` | Override group tags |
| `.OperationID(id)` | Custom operation ID (auto-generated if omitted) |
| `.Body(dto)` | Request body struct |
| `.BodyExample(ex)` | Single example for the request body |
| `.BodyExamples(map[string]any)` | Named examples (e.g. `"crmLogin"`, `"adminLogin"`) |
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

**Operation IDs** are auto-generated from the HTTP method and path (e.g. `POST /auth/login` → `postAuthLogin`). Override with `.OperationID("custom")`.

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

type ErrorResponse struct {
    Status  string `json:"status"  example:"error"`
    Message string `json:"message" example:"invalid credentials"`
}

func main() {
    r := apidoc.New(gin.Default(), apidoc.Info{
        Title:   "Auth Service API",
        Version: "1.0.0",
        Contact: &apidoc.Contact{Name: "BFree Africa", Email: "support@bfree.africa"},
    })

    r.Server("http://localhost:8080/api/v1", "Local")
    r.BearerAuth("bearerAuth", "JWT access token.")
    r.AddTag("Authentication", "User authentication endpoints")
    r.AddResponse("BadRequest", 400, ErrorResponse{}, "Bad request.")

    auth := r.Group("/api/v1/auth").Tag("Authentication")

    auth.POST("/login", loginHandler).
        Summary("Login").
        Description("Authenticate a user and return tokens.").
        Body(LoginRequest{}).
        Response(200, LoginResponse{}, "Login successful").
        ResponseRef(400, "BadRequest")

    r.ServeDocs("/docs")
    r.SaveDocs("docs/openapi.json")
    r.Run(":8080")
}
```
