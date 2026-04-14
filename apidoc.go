package apidoc

import "github.com/gin-gonic/gin"

// Info holds top-level API metadata.
type Info struct {
	Title       string
	Description string
	Version     string
	Contact     *Contact
}

// Contact holds contact information for the API.
type Contact struct {
	Name  string
	Email string
}

// serverDef is an OpenAPI server entry.
type serverDef struct {
	URL         string
	Description string
}

// tagDef is an OpenAPI tag with an optional description.
type tagDef struct {
	Name        string
	Description string
}

// securitySchemeDef describes a security scheme (currently only bearer).
type securitySchemeDef struct {
	Name         string
	Type         string
	Scheme       string
	BearerFormat string
	Description  string
}

// reusableResponse is a named response that endpoints can reference.
type reusableResponse struct {
	StatusCode  int
	Body        any
	Description string
}

// Param describes a path, query, or header parameter.
type Param struct {
	Name        string
	In          string // "path", "query", "header"
	Description string
	Required    bool
	Type        string
	Format      string
	Example     any
	Enum        []string
}

// Router wraps a gin.Engine and collects OpenAPI metadata.
type Router struct {
	engine          *gin.Engine
	info            Info
	servers         []serverDef
	tags            []tagDef
	securitySchemes []securitySchemeDef
	responses       map[string]reusableResponse
	params          map[string]Param
	endpoints       []*Endpoint
}

// Group wraps a gin.RouterGroup and collects OpenAPI metadata.
type Group struct {
	router   *Router
	ginGroup *gin.RouterGroup
	prefix   string
	tag      string
}

// New creates an apidoc Router wrapping the given gin.Engine.
func New(engine *gin.Engine, info Info) *Router {
	return &Router{
		engine:    engine,
		info:      info,
		responses: make(map[string]reusableResponse),
		params:    make(map[string]Param),
	}
}

// Engine returns the underlying gin.Engine.
func (r *Router) Engine() *gin.Engine {
	return r.engine
}

// Run starts the HTTP server (pass-through to gin.Engine).
func (r *Router) Run(addr ...string) error {
	return r.engine.Run(addr...)
}

// Server adds a server entry to the spec.
func (r *Router) Server(url, description string) *Router {
	r.servers = append(r.servers, serverDef{URL: url, Description: description})
	return r
}

// BearerAuth adds a bearer token security scheme.
func (r *Router) BearerAuth(name, description string) *Router {
	r.securitySchemes = append(r.securitySchemes, securitySchemeDef{
		Name:         name,
		Type:         "http",
		Scheme:       "bearer",
		BearerFormat: "JWT",
		Description:  description,
	})
	return r
}

// AddResponse registers a reusable response that endpoints can reference with ResponseRef.
func (r *Router) AddResponse(name string, statusCode int, body any, description string) *Router {
	r.responses[name] = reusableResponse{
		StatusCode:  statusCode,
		Body:        body,
		Description: description,
	}
	return r
}

// AddParam registers a reusable parameter that endpoints can reference with ParamRef.
func (r *Router) AddParam(name string, param Param) *Router {
	r.params[name] = param
	return r
}

// AddTag registers a top-level tag with a description.
func (r *Router) AddTag(name, description string) *Router {
	r.tags = append(r.tags, tagDef{Name: name, Description: description})
	return r
}

// Group creates a route group with a shared path prefix.
func (r *Router) Group(path string, handlers ...gin.HandlerFunc) *Group {
	return &Group{
		router:   r,
		ginGroup: r.engine.Group(path, handlers...),
		prefix:   path,
	}
}

// Use adds middleware to the underlying gin.Engine.
func (r *Router) Use(middleware ...gin.HandlerFunc) *Router {
	r.engine.Use(middleware...)
	return r
}

func (r *Router) addEndpoint(method, path string, tags []string, handlers []gin.HandlerFunc) *Endpoint {
	ep := &Endpoint{
		router: r,
		method: method,
		path:   path,
		tags:   tags,
	}
	// Extract doc metadata from any Docs() middleware in the handler chain.
	applyDocOptions(ep, handlers)
	r.endpoints = append(r.endpoints, ep)
	return ep
}

// HTTP method helpers on Router.

func (r *Router) POST(path string, handlers ...gin.HandlerFunc) *Endpoint {
	r.engine.POST(path, handlers...)
	return r.addEndpoint("POST", path, nil, handlers)
}

func (r *Router) GET(path string, handlers ...gin.HandlerFunc) *Endpoint {
	r.engine.GET(path, handlers...)
	return r.addEndpoint("GET", path, nil, handlers)
}

func (r *Router) PUT(path string, handlers ...gin.HandlerFunc) *Endpoint {
	r.engine.PUT(path, handlers...)
	return r.addEndpoint("PUT", path, nil, handlers)
}

func (r *Router) DELETE(path string, handlers ...gin.HandlerFunc) *Endpoint {
	r.engine.DELETE(path, handlers...)
	return r.addEndpoint("DELETE", path, nil, handlers)
}

func (r *Router) PATCH(path string, handlers ...gin.HandlerFunc) *Endpoint {
	r.engine.PATCH(path, handlers...)
	return r.addEndpoint("PATCH", path, nil, handlers)
}

// Tag sets the default tag for all endpoints in this group.
func (g *Group) Tag(name string) *Group {
	g.tag = name
	return g
}

// Group creates a sub-group with a shared path prefix.
func (g *Group) Group(path string, handlers ...gin.HandlerFunc) *Group {
	return &Group{
		router:   g.router,
		ginGroup: g.ginGroup.Group(path, handlers...),
		prefix:   g.prefix + path,
	}
}

// Use adds middleware to the underlying gin.RouterGroup.
func (g *Group) Use(middleware ...gin.HandlerFunc) *Group {
	g.ginGroup.Use(middleware...)
	return g
}

func (g *Group) addEndpoint(method, path string, handlers []gin.HandlerFunc) *Endpoint {
	var tags []string
	if g.tag != "" {
		tags = []string{g.tag}
	}
	fullPath := g.prefix + path
	ep := &Endpoint{
		router: g.router,
		method: method,
		path:   fullPath,
		tags:   tags,
	}
	// Extract doc metadata from any Docs() middleware in the handler chain.
	applyDocOptions(ep, handlers)
	g.router.endpoints = append(g.router.endpoints, ep)
	return ep
}

// HTTP method helpers on Group.

func (g *Group) POST(path string, handlers ...gin.HandlerFunc) *Endpoint {
	g.ginGroup.POST(path, handlers...)
	return g.addEndpoint("POST", path, handlers)
}

func (g *Group) GET(path string, handlers ...gin.HandlerFunc) *Endpoint {
	g.ginGroup.GET(path, handlers...)
	return g.addEndpoint("GET", path, handlers)
}

func (g *Group) PUT(path string, handlers ...gin.HandlerFunc) *Endpoint {
	g.ginGroup.PUT(path, handlers...)
	return g.addEndpoint("PUT", path, handlers)
}

func (g *Group) DELETE(path string, handlers ...gin.HandlerFunc) *Endpoint {
	g.ginGroup.DELETE(path, handlers...)
	return g.addEndpoint("DELETE", path, handlers)
}

func (g *Group) PATCH(path string, handlers ...gin.HandlerFunc) *Endpoint {
	g.ginGroup.PATCH(path, handlers...)
	return g.addEndpoint("PATCH", path, handlers)
}
