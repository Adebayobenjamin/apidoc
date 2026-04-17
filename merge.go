package apidoc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// Source is an input to Merge. It carries a namespace used to resolve schema
// name collisions and the parsed OpenAPI spec.
type Source struct {
	Namespace string
	Spec      map[string]any
	loadErr   error
}

// SourceFile loads an OpenAPI JSON spec from disk. If namespace is empty,
// it is inferred from the parent directory's base name (e.g. a file at
// "../auth-service/docs/openapi.json" yields namespace "auth-service").
//
// Any load error is deferred until Merge runs so SourceFile can be used
// inline in a variadic Merge call.
func SourceFile(path string, namespace ...string) Source {
	ns := ""
	if len(namespace) > 0 {
		ns = namespace[0]
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Source{Namespace: ns, loadErr: fmt.Errorf("apidoc: failed to read %s: %w", path, err)}
	}

	var spec map[string]any
	if err := json.Unmarshal(data, &spec); err != nil {
		return Source{Namespace: ns, loadErr: fmt.Errorf("apidoc: failed to parse %s: %w", path, err)}
	}

	if ns == "" {
		ns = inferNamespace(path)
	}
	return Source{Namespace: ns, Spec: spec}
}

// Merge combines multiple OpenAPI specs into a single Router. Schema name
// collisions across sources are resolved by prefixing the schema with a
// title-cased namespace. Path collisions across sources return an error.
// Tags, servers, and reusable components are merged by name; later sources
// override earlier ones.
//
// The returned Router wraps a fresh gin.Engine; call ServeDocs or SaveDocs
// on it. Use MergeOn to attach to an existing engine.
func Merge(info Info, sources ...Source) (*Router, error) {
	return MergeOn(gin.New(), info, sources...)
}

// MergeOn is like Merge but attaches to an existing gin.Engine.
func MergeOn(engine *gin.Engine, info Info, sources ...Source) (*Router, error) {
	for _, s := range sources {
		if s.loadErr != nil {
			return nil, s.loadErr
		}
	}

	r := New(engine, info)

	// Detect schema-name collisions across sources.
	schemaOwners := map[string]int{}
	for _, src := range sources {
		for name := range extractSchemas(src.Spec) {
			schemaOwners[name]++
		}
	}

	// Build per-source rename maps for collisions only.
	renameMaps := map[int]map[string]string{}
	for i, src := range sources {
		renames := map[string]string{}
		for name := range extractSchemas(src.Spec) {
			if schemaOwners[name] > 1 {
				renames[name] = prefixName(src.Namespace, name)
			}
		}
		renameMaps[i] = renames
	}

	merged := map[string]any{
		"openapi": "3.0.3",
		"info":    r.buildInfo(),
	}

	paths := map[string]any{}
	schemas := map[string]any{}
	responses := map[string]any{}
	parameters := map[string]any{}
	securitySchemes := map[string]any{}
	var tags []map[string]any
	var servers []map[string]any

	seenPathMethod := map[string]string{} // "METHOD path" -> namespace
	seenTag := map[string]bool{}
	seenServer := map[string]bool{}

	for i, src := range sources {
		renames := renameMaps[i]

		// Paths — error on method-level collisions across sources.
		if srcPaths, ok := src.Spec["paths"].(map[string]any); ok {
			for p, item := range srcPaths {
				itemMap, ok := item.(map[string]any)
				if !ok {
					continue
				}
				rewritten := rewriteRefs(itemMap, renames).(map[string]any)

				existing, exists := paths[p].(map[string]any)
				if !exists {
					existing = map[string]any{}
				}
				for method, op := range rewritten {
					key := strings.ToUpper(method) + " " + p
					if owner, seen := seenPathMethod[key]; seen {
						return nil, fmt.Errorf("apidoc: path collision %s in sources %q and %q", key, owner, src.Namespace)
					}
					seenPathMethod[key] = src.Namespace
					existing[method] = op
				}
				paths[p] = existing
			}
		}

		// Schemas.
		for name, schema := range extractSchemas(src.Spec) {
			newName := name
			if nn, ok := renames[name]; ok {
				newName = nn
			}
			schemas[newName] = rewriteRefs(schema, renames)
		}

		// Other components — merged by name.
		if comps, ok := src.Spec["components"].(map[string]any); ok {
			if m, ok := comps["responses"].(map[string]any); ok {
				for name, v := range m {
					responses[name] = rewriteRefs(v, renames)
				}
			}
			if m, ok := comps["parameters"].(map[string]any); ok {
				for name, v := range m {
					parameters[name] = rewriteRefs(v, renames)
				}
			}
			if m, ok := comps["securitySchemes"].(map[string]any); ok {
				for name, v := range m {
					securitySchemes[name] = v
				}
			}
		}

		// Tags — dedupe by name.
		if srcTags, ok := src.Spec["tags"].([]any); ok {
			for _, t := range srcTags {
				tag, ok := t.(map[string]any)
				if !ok {
					continue
				}
				name, _ := tag["name"].(string)
				if name == "" || seenTag[name] {
					continue
				}
				seenTag[name] = true
				tags = append(tags, tag)
			}
		}

		// Servers — dedupe by URL.
		if srcServers, ok := src.Spec["servers"].([]any); ok {
			for _, s := range srcServers {
				server, ok := s.(map[string]any)
				if !ok {
					continue
				}
				url, _ := server["url"].(string)
				if url == "" || seenServer[url] {
					continue
				}
				seenServer[url] = true
				servers = append(servers, server)
			}
		}
	}

	components := map[string]any{}
	if len(schemas) > 0 {
		components["schemas"] = schemas
	}
	if len(responses) > 0 {
		components["responses"] = responses
	}
	if len(parameters) > 0 {
		components["parameters"] = parameters
	}
	if len(securitySchemes) > 0 {
		components["securitySchemes"] = securitySchemes
	}

	merged["paths"] = paths
	if len(components) > 0 {
		merged["components"] = components
	}
	if len(tags) > 0 {
		merged["tags"] = tags
	}
	if len(servers) > 0 {
		merged["servers"] = servers
	}

	r.prebuiltSpec = merged
	return r, nil
}

func extractSchemas(spec map[string]any) map[string]any {
	components, ok := spec["components"].(map[string]any)
	if !ok {
		return nil
	}
	schemas, _ := components["schemas"].(map[string]any)
	return schemas
}

func inferNamespace(path string) string {
	// "../auth-service/docs/openapi.json" → "auth-service"
	dir := filepath.Dir(filepath.Dir(path))
	return filepath.Base(dir)
}

func prefixName(namespace, name string) string {
	if namespace == "" {
		return name
	}
	parts := strings.FieldsFunc(namespace, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		if len(p) > 1 {
			b.WriteString(p[1:])
		}
	}
	b.WriteString(name)
	return b.String()
}

// rewriteRefs walks v and rewrites any "$ref" strings pointing to renamed
// schemas under #/components/schemas/.
func rewriteRefs(v any, renames map[string]string) any {
	if len(renames) == 0 {
		return v
	}
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, vv := range val {
			if k == "$ref" {
				if s, ok := vv.(string); ok {
					const prefix = "#/components/schemas/"
					if strings.HasPrefix(s, prefix) {
						name := strings.TrimPrefix(s, prefix)
						if newName, ok := renames[name]; ok {
							out[k] = prefix + newName
							continue
						}
					}
				}
				out[k] = vv
			} else {
				out[k] = rewriteRefs(vv, renames)
			}
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, vv := range val {
			out[i] = rewriteRefs(vv, renames)
		}
		return out
	}
	return v
}
