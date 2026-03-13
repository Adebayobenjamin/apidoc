package apidoc

import (
	"reflect"
	"strconv"
	"strings"
)

// schemaCollector tracks all referenced schemas so they can be placed
// under components/schemas in the final spec.
type schemaCollector struct {
	schemas map[string]map[string]any
}

func newSchemaCollector() *schemaCollector {
	return &schemaCollector{schemas: make(map[string]map[string]any)}
}

// resolve returns the OpenAPI schema for the given value.
// Structs are added to the collector and returned as a $ref.
func (sc *schemaCollector) resolve(v any) map[string]any {
	if v == nil {
		return nil
	}
	t := reflect.TypeOf(v)
	return sc.resolveType(t)
}

func (sc *schemaCollector) resolveType(t reflect.Type) map[string]any {
	// Dereference pointer.
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Known special types.
	if schema, ok := knownType(t); ok {
		return schema
	}

	switch t.Kind() {
	case reflect.Struct:
		name := t.Name()
		if _, exists := sc.schemas[name]; !exists {
			// Placeholder to prevent infinite recursion.
			sc.schemas[name] = nil
			sc.schemas[name] = sc.structToSchema(t)
		}
		return map[string]any{"$ref": "#/components/schemas/" + name}

	case reflect.Slice, reflect.Array:
		items := sc.resolveType(t.Elem())
		return map[string]any{"type": "array", "items": items}

	case reflect.Map:
		additional := sc.resolveType(t.Elem())
		return map[string]any{"type": "object", "additionalProperties": additional}

	default:
		typ, format := goKindToOpenAPI(t.Kind())
		schema := map[string]any{"type": typ}
		if format != "" {
			schema["format"] = format
		}
		return schema
	}
}

func (sc *schemaCollector) structToSchema(t reflect.Type) map[string]any {
	properties := map[string]any{}
	var required []string

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields.
		if !field.IsExported() {
			continue
		}

		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}

		name, jsonOpts := parseJSONTag(jsonTag)
		if name == "" {
			name = field.Name
		}

		prop := sc.fieldToProperty(field)

		// Required: from binding tag, unless omitempty is set.
		binding := field.Tag.Get("binding")
		if containsTagValue(binding, "required") && !containsOption(jsonOpts, "omitempty") {
			required = append(required, name)
		}

		// Description tag.
		if desc := field.Tag.Get("description"); desc != "" {
			prop["description"] = desc
		}

		// Rules tag — appended to description.
		if rules := field.Tag.Get("rules"); rules != "" {
			if desc, ok := prop["description"].(string); ok && desc != "" {
				prop["description"] = desc + ". " + rules
			} else {
				prop["description"] = rules
			}
		}

		// Example tag.
		if ex := field.Tag.Get("example"); ex != "" {
			prop["example"] = convertExample(ex, field.Type)
		}

		// Enum tag.
		if enum := field.Tag.Get("enum"); enum != "" {
			prop["enum"] = strings.Split(enum, ",")
		}

		// Validation constraints from binding/validate tags.
		applyValidationTags(prop, field)

		properties[name] = prop
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func (sc *schemaCollector) fieldToProperty(field reflect.StructField) map[string]any {
	ft := field.Type
	isPointer := ft.Kind() == reflect.Ptr
	if isPointer {
		ft = ft.Elem()
	}

	var prop map[string]any

	if schema, ok := knownType(ft); ok {
		prop = schema
	} else if ft.Kind() == reflect.Struct {
		prop = sc.resolveType(ft)
	} else if ft.Kind() == reflect.Slice || ft.Kind() == reflect.Array {
		items := sc.resolveType(ft.Elem())
		prop = map[string]any{"type": "array", "items": items}
	} else if ft.Kind() == reflect.Map {
		additional := sc.resolveType(ft.Elem())
		prop = map[string]any{"type": "object", "additionalProperties": additional}
	} else {
		typ, format := goKindToOpenAPI(ft.Kind())
		prop = map[string]any{"type": typ}
		if format != "" {
			prop["format"] = format
		}
	}

	if isPointer {
		prop["nullable"] = true
	}

	return prop
}

// knownType checks for well-known types that map to specific OpenAPI schemas.
func knownType(t reflect.Type) (map[string]any, bool) {
	pkg := t.PkgPath()
	name := t.Name()

	switch {
	case pkg == "github.com/google/uuid" && name == "UUID":
		return map[string]any{"type": "string", "format": "uuid"}, true
	case pkg == "time" && name == "Time":
		return map[string]any{"type": "string", "format": "date-time"}, true
	}

	// Types whose underlying kind is string (e.g. models.Password, CommChannel).
	if t.Kind() == reflect.String && pkg != "" && name != "" {
		return map[string]any{"type": "string"}, true
	}

	return nil, false
}

func goKindToOpenAPI(k reflect.Kind) (string, string) {
	switch k {
	case reflect.String:
		return "string", ""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		return "integer", "int32"
	case reflect.Int64:
		return "integer", "int64"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return "integer", "int32"
	case reflect.Uint64:
		return "integer", "int64"
	case reflect.Float32:
		return "number", "float"
	case reflect.Float64:
		return "number", "double"
	case reflect.Bool:
		return "boolean", ""
	default:
		return "string", ""
	}
}

// applyValidationTags reads binding/validate tags and sets OpenAPI constraints.
func applyValidationTags(prop map[string]any, field reflect.StructField) {
	tags := field.Tag.Get("binding") + "," + field.Tag.Get("validate")

	for _, part := range strings.Split(tags, ",") {
		part = strings.TrimSpace(part)

		switch {
		case part == "email":
			prop["format"] = "email"
		case part == "uuid":
			prop["format"] = "uuid"
		case part == "url":
			prop["format"] = "uri"
		case strings.HasPrefix(part, "min="):
			n, err := strconv.Atoi(strings.TrimPrefix(part, "min="))
			if err == nil {
				typ, _ := prop["type"].(string)
				if typ == "string" {
					prop["minLength"] = n
				} else if typ == "integer" || typ == "number" {
					prop["minimum"] = n
				}
			}
		case strings.HasPrefix(part, "max="):
			n, err := strconv.Atoi(strings.TrimPrefix(part, "max="))
			if err == nil {
				typ, _ := prop["type"].(string)
				if typ == "string" {
					prop["maxLength"] = n
				} else if typ == "integer" || typ == "number" {
					prop["maximum"] = n
				}
			}
		case strings.HasPrefix(part, "len="):
			n, err := strconv.Atoi(strings.TrimPrefix(part, "len="))
			if err == nil {
				prop["minLength"] = n
				prop["maxLength"] = n
			}
		case strings.HasPrefix(part, "oneof="):
			values := strings.Fields(strings.TrimPrefix(part, "oneof="))
			prop["enum"] = values
		}
	}
}

// parseJSONTag splits a json tag into the field name and remaining options.
func parseJSONTag(tag string) (string, string) {
	if idx := strings.Index(tag, ","); idx != -1 {
		return tag[:idx], tag[idx+1:]
	}
	return tag, ""
}

func containsTagValue(tag, value string) bool {
	for _, v := range strings.Split(tag, ",") {
		if strings.TrimSpace(v) == value {
			return true
		}
	}
	return false
}

func containsOption(opts, value string) bool {
	for _, v := range strings.Split(opts, ",") {
		if strings.TrimSpace(v) == value {
			return true
		}
	}
	return false
}

// convertExample attempts to convert a string example to the appropriate Go type.
func convertExample(s string, t reflect.Type) any {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return n
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if n, err := strconv.ParseUint(s, 10, 64); err == nil {
			return n
		}
	case reflect.Float32, reflect.Float64:
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
	case reflect.Bool:
		if b, err := strconv.ParseBool(s); err == nil {
			return b
		}
	}
	return s
}
