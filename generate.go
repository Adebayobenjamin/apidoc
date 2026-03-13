package apidoc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// SaveDocs generates the OpenAPI spec and writes it to the given path.
func (r *Router) SaveDocs(path string) error {
	spec := r.generateSpec()

	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return fmt.Errorf("apidoc: failed to marshal spec: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("apidoc: failed to create directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("apidoc: failed to write file: %w", err)
	}

	return nil
}

func (r *Router) generateSpec() map[string]any {
	sc := newSchemaCollector()

	spec := map[string]any{
		"openapi": "3.0.3",
		"info":    r.buildInfo(),
	}

	if len(r.servers) > 0 {
		spec["servers"] = r.buildServers()
	}

	if len(r.tags) > 0 {
		spec["tags"] = r.buildTags()
	}

	spec["paths"] = r.buildPaths(sc)
	spec["components"] = r.buildComponents(sc)

	return spec
}

func (r *Router) buildInfo() map[string]any {
	info := map[string]any{
		"title":   r.info.Title,
		"version": r.info.Version,
	}
	if r.info.Description != "" {
		info["description"] = r.info.Description
	}
	if r.info.Contact != nil {
		contact := map[string]any{}
		if r.info.Contact.Name != "" {
			contact["name"] = r.info.Contact.Name
		}
		if r.info.Contact.Email != "" {
			contact["email"] = r.info.Contact.Email
		}
		info["contact"] = contact
	}
	return info
}

func (r *Router) buildServers() []map[string]any {
	servers := make([]map[string]any, len(r.servers))
	for i, s := range r.servers {
		servers[i] = map[string]any{
			"url":         s.URL,
			"description": s.Description,
		}
	}
	return servers
}

func (r *Router) buildTags() []map[string]any {
	tags := make([]map[string]any, len(r.tags))
	for i, t := range r.tags {
		tag := map[string]any{"name": t.Name}
		if t.Description != "" {
			tag["description"] = t.Description
		}
		tags[i] = tag
	}
	return tags
}

func (r *Router) buildPaths(sc *schemaCollector) map[string]any {
	paths := map[string]any{}

	for _, ep := range r.endpoints {
		openAPIPath := ginPathToOpenAPI(ep.path)

		if _, ok := paths[openAPIPath]; !ok {
			paths[openAPIPath] = map[string]any{}
		}
		pathItem := paths[openAPIPath].(map[string]any)
		pathItem[strings.ToLower(ep.method)] = r.buildOperation(ep, sc)
	}

	return paths
}

func (r *Router) buildOperation(ep *Endpoint, sc *schemaCollector) map[string]any {
	op := map[string]any{}

	if len(ep.tags) > 0 {
		op["tags"] = ep.tags
	}
	if ep.summary != "" {
		op["summary"] = ep.summary
	}
	if ep.description != "" {
		op["description"] = ep.description
	}

	// Operation ID.
	opID := ep.operationID
	if opID == "" {
		opID = generateOperationID(ep.method, ep.path)
	}
	op["operationId"] = opID

	if ep.deprecated {
		op["deprecated"] = true
	}

	// Security.
	if len(ep.security) > 0 {
		sec := make([]map[string]any, len(ep.security))
		for i, s := range ep.security {
			sec[i] = map[string]any{s: []string{}}
		}
		op["security"] = sec
	}

	// Parameters.
	params := r.buildParams(ep)
	if len(params) > 0 {
		op["parameters"] = params
	}

	// Request body.
	if ep.body != nil {
		op["requestBody"] = r.buildRequestBody(ep, sc)
	}

	// Responses.
	if len(ep.responses) > 0 {
		op["responses"] = r.buildResponses(ep, sc)
	}

	return op
}

func (r *Router) buildParams(ep *Endpoint) []any {
	var params []any

	// Referenced params.
	for _, name := range ep.paramRefs {
		params = append(params, map[string]any{
			"$ref": "#/components/parameters/" + name,
		})
	}

	// Inline params.
	for _, p := range ep.params {
		param := map[string]any{
			"name": p.Name,
			"in":   p.In,
		}
		if p.Description != "" {
			param["description"] = p.Description
		}
		if p.Required {
			param["required"] = true
		}

		schema := map[string]any{"type": p.Type}
		if p.Format != "" {
			schema["format"] = p.Format
		}
		if len(p.Enum) > 0 {
			schema["enum"] = p.Enum
		}
		param["schema"] = schema

		if p.Example != nil {
			param["example"] = p.Example
		}

		params = append(params, param)
	}

	return params
}

func (r *Router) buildRequestBody(ep *Endpoint, sc *schemaCollector) map[string]any {
	schema := sc.resolve(ep.body)

	content := map[string]any{
		"schema": schema,
	}

	// Single example.
	if ep.bodyExample != nil {
		content["example"] = ep.bodyExample
	}

	// Named examples.
	if len(ep.bodyExamples) > 0 {
		examples := map[string]any{}
		for name, val := range ep.bodyExamples {
			examples[name] = map[string]any{"value": val}
		}
		content["examples"] = examples
	}

	return map[string]any{
		"required": true,
		"content": map[string]any{
			"application/json": content,
		},
	}
}

func (r *Router) buildResponses(ep *Endpoint, sc *schemaCollector) map[string]any {
	responses := map[string]any{}

	for _, resp := range ep.responses {
		key := fmt.Sprintf("%d", resp.statusCode)

		// Referenced response.
		if resp.ref != "" {
			responses[key] = map[string]any{
				"$ref": "#/components/responses/" + resp.ref,
			}
			continue
		}

		respObj := map[string]any{
			"description": resp.description,
		}

		if resp.body != nil {
			content := map[string]any{
				"schema": sc.resolve(resp.body),
			}
			if resp.example != nil {
				content["example"] = resp.example
			}
			if len(resp.examples) > 0 {
				examples := map[string]any{}
				for name, val := range resp.examples {
					examples[name] = map[string]any{"value": val}
				}
				content["examples"] = examples
			}
			respObj["content"] = map[string]any{
				"application/json": content,
			}
		}

		responses[key] = respObj
	}

	return responses
}

func (r *Router) buildComponents(sc *schemaCollector) map[string]any {
	components := map[string]any{}

	// Schemas from reflection.
	if len(sc.schemas) > 0 {
		components["schemas"] = sc.schemas
	}

	// Reusable responses.
	if len(r.responses) > 0 {
		responses := map[string]any{}
		for name, resp := range r.responses {
			respObj := map[string]any{
				"description": resp.Description,
			}
			if resp.Body != nil {
				respObj["content"] = map[string]any{
					"application/json": map[string]any{
						"schema": sc.resolve(resp.Body),
					},
				}
			}
			responses[name] = respObj
		}
		components["responses"] = responses
	}

	// Reusable parameters.
	if len(r.params) > 0 {
		params := map[string]any{}
		for name, p := range r.params {
			param := map[string]any{
				"name": p.Name,
				"in":   p.In,
			}
			if p.Description != "" {
				param["description"] = p.Description
			}
			if p.Required {
				param["required"] = true
			}
			schema := map[string]any{"type": p.Type}
			if p.Format != "" {
				schema["format"] = p.Format
			}
			if len(p.Enum) > 0 {
				schema["enum"] = p.Enum
			}
			param["schema"] = schema
			if p.Example != nil {
				param["example"] = p.Example
			}
			params[name] = param
		}
		components["parameters"] = params
	}

	// Security schemes.
	if len(r.securitySchemes) > 0 {
		schemes := map[string]any{}
		for _, s := range r.securitySchemes {
			scheme := map[string]any{
				"type":   s.Type,
				"scheme": s.Scheme,
			}
			if s.BearerFormat != "" {
				scheme["bearerFormat"] = s.BearerFormat
			}
			if s.Description != "" {
				scheme["description"] = s.Description
			}
			schemes[s.Name] = scheme
		}
		components["securitySchemes"] = schemes
	}

	return components
}

// ginPathToOpenAPI converts Gin's :param syntax to OpenAPI's {param} syntax.
func ginPathToOpenAPI(path string) string {
	re := regexp.MustCompile(`:(\w+)`)
	return re.ReplaceAllString(path, `{$1}`)
}

// generateOperationID builds an operation ID from the HTTP method and path.
// e.g. POST /auth/login → postAuthLogin
func generateOperationID(method, path string) string {
	// Remove leading slash and parameter placeholders.
	clean := strings.TrimPrefix(path, "/")
	clean = regexp.MustCompile(`[:/{}]`).ReplaceAllString(clean, " ")
	clean = strings.TrimSpace(clean)

	parts := strings.Fields(clean)
	var b strings.Builder
	b.WriteString(strings.ToLower(method))

	for _, part := range parts {
		if len(part) > 0 {
			b.WriteString(strings.ToUpper(part[:1]) + part[1:])
		}
	}

	return b.String()
}
