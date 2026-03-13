package apidoc

// responseDef holds metadata for a single response.
type responseDef struct {
	statusCode  int
	body        any
	description string
	example     any
	examples    map[string]any
	ref         string // name of a reusable response
}

// Endpoint holds all documentation metadata for a single route.
type Endpoint struct {
	router       *Router
	method       string
	path         string
	summary      string
	description  string
	tags         []string
	operationID  string
	body         any
	bodyExample  any
	bodyExamples map[string]any
	responses    []responseDef
	params       []Param
	paramRefs    []string
	security     []string
	deprecated   bool
}

// Summary sets a short summary for this endpoint.
func (e *Endpoint) Summary(s string) *Endpoint {
	e.summary = s
	return e
}

// Description sets a detailed description for this endpoint.
func (e *Endpoint) Description(d string) *Endpoint {
	e.description = d
	return e
}

// Tags overrides the group's default tags.
func (e *Endpoint) Tags(tags ...string) *Endpoint {
	e.tags = tags
	return e
}

// OperationID sets a custom operation ID (auto-generated if omitted).
func (e *Endpoint) OperationID(id string) *Endpoint {
	e.operationID = id
	return e
}

// Body sets the request body DTO. The struct is reflected to build the schema.
func (e *Endpoint) Body(dto any) *Endpoint {
	e.body = dto
	return e
}

// BodyExample sets a single example for the request body.
func (e *Endpoint) BodyExample(ex any) *Endpoint {
	e.bodyExample = ex
	return e
}

// BodyExamples sets named examples for the request body.
func (e *Endpoint) BodyExamples(examples map[string]any) *Endpoint {
	e.bodyExamples = examples
	return e
}

// Response adds a response with a DTO schema.
func (e *Endpoint) Response(statusCode int, dto any, description string) *Endpoint {
	e.responses = append(e.responses, responseDef{
		statusCode:  statusCode,
		body:        dto,
		description: description,
	})
	return e
}

// ResponseRef references a reusable response registered with Router.AddResponse.
func (e *Endpoint) ResponseRef(statusCode int, name string) *Endpoint {
	e.responses = append(e.responses, responseDef{
		statusCode: statusCode,
		ref:        name,
	})
	return e
}

// ResponseExample adds a single example to a previously defined response.
func (e *Endpoint) ResponseExample(statusCode int, ex any) *Endpoint {
	for i := range e.responses {
		if e.responses[i].statusCode == statusCode {
			e.responses[i].example = ex
			return e
		}
	}
	return e
}

// ResponseExamples adds named examples to a previously defined response.
func (e *Endpoint) ResponseExamples(statusCode int, examples map[string]any) *Endpoint {
	for i := range e.responses {
		if e.responses[i].statusCode == statusCode {
			e.responses[i].examples = examples
			return e
		}
	}
	return e
}

// PathParam documents a path parameter.
func (e *Endpoint) PathParam(name, description string, example any) *Endpoint {
	e.params = append(e.params, Param{
		Name:        name,
		In:          "path",
		Description: description,
		Required:    true,
		Type:        "string",
		Example:     example,
	})
	return e
}

// QueryParam documents a query parameter.
func (e *Endpoint) QueryParam(name, typ, description string, example any) *Endpoint {
	e.params = append(e.params, Param{
		Name:        name,
		In:          "query",
		Description: description,
		Type:        typ,
		Example:     example,
	})
	return e
}

// HeaderParam documents a header parameter.
func (e *Endpoint) HeaderParam(name, typ, description string, example any) *Endpoint {
	e.params = append(e.params, Param{
		Name:        name,
		In:          "header",
		Description: description,
		Type:        typ,
		Example:     example,
	})
	return e
}

// ParamRef references a reusable parameter registered with Router.AddParam.
func (e *Endpoint) ParamRef(name string) *Endpoint {
	e.paramRefs = append(e.paramRefs, name)
	return e
}

// Security requires a named security scheme for this endpoint.
func (e *Endpoint) Security(scheme string) *Endpoint {
	e.security = append(e.security, scheme)
	return e
}

// Deprecated marks this endpoint as deprecated.
func (e *Endpoint) Deprecated() *Endpoint {
	e.deprecated = true
	return e
}
