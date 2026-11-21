package apidoc

import (
	"reflect"

	"github.com/gin-gonic/gin"
)

// DocOption is a function that applies documentation metadata to an Endpoint.
type DocOption func(*Endpoint)

// docMiddleware is a gin.HandlerFunc that carries doc metadata.
// It is a no-op at runtime — it does nothing when requests come in.
// The Router/Group detects it during route registration and extracts the options.
type docMiddleware struct {
	options []DocOption
}

// marker interface so we can detect it in the handler chain.
type docCarrier interface {
	docOptions() []DocOption
}

func (d *docMiddleware) docOptions() []DocOption {
	return d.options
}

// Docs returns a gin.HandlerFunc that carries documentation metadata.
// It is a no-op at runtime. The apidoc Router extracts the metadata
// during route registration to build the OpenAPI spec.
//
// Usage:
//
//	auth.POST("/login",
//	    apidoc.Docs(
//	        apidoc.Summary("Login"),
//	        apidoc.Body(dtos.LoginRequest{}),
//	        apidoc.Response(200, dtos.LoginResponse{}, "Success"),
//	    ),
//	    handler,
//	)
func Docs(opts ...DocOption) gin.HandlerFunc {
	dm := &docMiddleware{options: opts}
	// Return a closure that is a no-op at runtime but holds a reference
	// to the docMiddleware so we can extract it via the carrier map.
	handler := func(c *gin.Context) {
		c.Next()
	}
	// Register this handler → docMiddleware mapping so route registration can find it.
	registerDocCarrier(handler, dm)
	return handler
}

// Global registry mapping handler function pointers to their doc metadata.
// This is necessary because gin.HandlerFunc is a func type and can't implement interfaces.
var docCarriers = map[uintptr]*docMiddleware{}

func registerDocCarrier(handler gin.HandlerFunc, dm *docMiddleware) {
	ptr := handlerPtr(handler)
	docCarriers[ptr] = dm
}

func findDocCarrier(handler gin.HandlerFunc) *docMiddleware {
	ptr := handlerPtr(handler)
	return docCarriers[ptr]
}

// handlerPtr returns a comparable pointer for a gin.HandlerFunc.
func handlerPtr(handler gin.HandlerFunc) uintptr {
	return reflect.ValueOf(handler).Pointer()
}

// applyDocOptions applies all DocOptions from a docMiddleware to an Endpoint.
func applyDocOptions(ep *Endpoint, handlers []gin.HandlerFunc) {
	for _, h := range handlers {
		if dm := findDocCarrier(h); dm != nil {
			for _, opt := range dm.options {
				opt(ep)
			}
		}
	}
}

// --- Option functions ---

// Summary returns a DocOption that sets the endpoint summary.
func Summary(s string) DocOption {
	return func(e *Endpoint) { e.summary = s }
}

// Description returns a DocOption that sets the endpoint description.
func Description(d string) DocOption {
	return func(e *Endpoint) { e.description = d }
}

// Tags returns a DocOption that overrides the endpoint tags.
func Tags(tags ...string) DocOption {
	return func(e *Endpoint) { e.tags = tags }
}

// OperationID returns a DocOption that sets a custom operation ID.
func OperationID(id string) DocOption {
	return func(e *Endpoint) { e.operationID = id }
}

// Body returns a DocOption that sets the request body DTO.
func Body(dto any) DocOption {
	return func(e *Endpoint) { e.body = dto }
}

// BodyExample returns a DocOption that sets a single request body example.
func BodyExample(ex any) DocOption {
	return func(e *Endpoint) { e.bodyExample = ex }
}

// BodyExamples returns a DocOption that sets named request body examples.
func BodyExamples(examples map[string]any) DocOption {
	return func(e *Endpoint) { e.bodyExamples = examples }
}

// Response returns a DocOption that adds a response with a DTO schema.
func Response(statusCode int, dto any, description string) DocOption {
	return func(e *Endpoint) {
		e.responses = append(e.responses, responseDef{
			statusCode:  statusCode,
			body:        dto,
			description: description,
		})
	}
}

// ResponseRef returns a DocOption that references a reusable response.
func ResponseRef(statusCode int, name string) DocOption {
	return func(e *Endpoint) {
		e.responses = append(e.responses, responseDef{
			statusCode: statusCode,
			ref:        name,
		})
	}
}

// PathParam returns a DocOption that documents a path parameter.
func PathParam(name, description string, example any) DocOption {
	return func(e *Endpoint) {
		e.params = append(e.params, Param{
			Name:        name,
			In:          "path",
			Description: description,
			Required:    true,
			Type:        "string",
			Example:     example,
		})
	}
}

// QueryParam returns a DocOption that documents a query parameter.
func QueryParam(name, typ, description string, example any) DocOption {
	return func(e *Endpoint) {
		e.params = append(e.params, Param{
			Name:        name,
			In:          "query",
			Description: description,
			Type:        typ,
			Example:     example,
		})
	}
}

// HeaderParam returns a DocOption that documents a header parameter.
func HeaderParam(name, typ, description string, example any) DocOption {
	return func(e *Endpoint) {
		e.params = append(e.params, Param{
			Name:        name,
			In:          "header",
			Description: description,
			Type:        typ,
			Example:     example,
		})
	}
}

// ParamRef returns a DocOption that references a reusable parameter.
func ParamRef(name string) DocOption {
	return func(e *Endpoint) {
		e.paramRefs = append(e.paramRefs, name)
	}
}

// Security returns a DocOption that requires a named security scheme.
func Security(scheme string) DocOption {
	return func(e *Endpoint) {
		e.security = append(e.security, scheme)
	}
}

// Deprecated returns a DocOption that marks the endpoint as deprecated.
func Deprecated() DocOption {
	return func(e *Endpoint) { e.deprecated = true }
}
