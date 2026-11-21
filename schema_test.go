package apidoc

import (
	"testing"

	"github.com/gin-gonic/gin"
)

type profile struct {
	Name string  `json:"name"`
	Bio  *string `json:"bio"`
	Nick string  `json:"nick,omitempty"`
}

type signInResponse struct {
	Token   string   `json:"token"`
	Expires int64    `json:"expires,omitzero"`
	User    *profile `json:"user,omitempty"`
}

type signInRequest struct {
	Email string `json:"email" binding:"required"`
	Code  string `json:"code"`
}

type echo struct {
	ID   string `json:"id" binding:"required"`
	Note string `json:"note"`
}

type problem struct {
	Code string `json:"code"`
}

func noop(*gin.Context) {}

func specSchemas(t *testing.T, register func(r *Router)) map[string]map[string]any {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := New(gin.New(), Info{Title: "t", Version: "1"})
	register(r)
	components, ok := r.generateSpec()["components"].(map[string]any)
	if !ok {
		t.Fatal("spec has no components")
	}
	schemas, ok := components["schemas"].(map[string]map[string]any)
	if !ok {
		t.Fatal("spec has no component schemas")
	}
	return schemas
}

func requiredOf(t *testing.T, schemas map[string]map[string]any, name string) []string {
	t.Helper()
	schema, ok := schemas[name]
	if !ok {
		t.Fatalf("no schema %q", name)
	}
	if schema["required"] == nil {
		return nil
	}
	req, ok := schema["required"].([]string)
	if !ok {
		t.Fatalf("%s.required is %T", name, schema["required"])
	}
	return req
}

func assertRequired(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("required = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("required = %v, want %v", got, want)
		}
	}
}

func TestRequiredFields(t *testing.T) {
	t.Run("a response marks every field without omitempty or omitzero required, nested types too", func(t *testing.T) {
		schemas := specSchemas(t, func(r *Router) {
			r.GET("/me", noop).Response(200, signInResponse{}, "ok")
		})
		assertRequired(t, requiredOf(t, schemas, "signInResponse"), "token")
		assertRequired(t, requiredOf(t, schemas, "profile"), "name", "bio")
	})

	t.Run("a request keeps the binding rule", func(t *testing.T) {
		schemas := specSchemas(t, func(r *Router) {
			r.POST("/sign-in", noop).Body(signInRequest{})
		})
		assertRequired(t, requiredOf(t, schemas, "signInRequest"), "email")
	})

	t.Run("a type used as both request and response keeps the request rule", func(t *testing.T) {
		for name, register := range map[string]func(r *Router){
			"request first":  func(r *Router) { r.POST("/echo", noop).Body(echo{}).Response(200, echo{}, "ok") },
			"response first": func(r *Router) { r.GET("/echo", noop).Response(200, echo{}, "ok"); r.POST("/echo", noop).Body(echo{}) },
		} {
			t.Run(name, func(t *testing.T) {
				assertRequired(t, requiredOf(t, specSchemas(t, register), "echo"), "id")
			})
		}
	})

	t.Run("a struct nested in a later request body loses response-only requirements", func(t *testing.T) {
		type wrapper struct {
			Profile profile `json:"profile"`
		}
		schemas := specSchemas(t, func(r *Router) {
			r.GET("/me", noop).Response(200, signInResponse{}, "ok")
			r.POST("/me", noop).Body(wrapper{})
		})
		assertRequired(t, requiredOf(t, schemas, "profile"))
	})

	t.Run("a schema only a reusable response references is emitted, with response rules", func(t *testing.T) {
		schemas := specSchemas(t, func(r *Router) {
			r.AddResponse("Problem", 400, problem{}, "bad request")
		})
		assertRequired(t, requiredOf(t, schemas, "problem"), "code")
	})
}
