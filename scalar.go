package apidoc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ScalarConfig controls the Scalar API reference UI appearance.
type ScalarConfig struct {
	Theme             string // Scalar theme name (default: "purple")
	DarkMode          bool   // Force dark mode (default: true)
	Layout            string // "modern" or "classic" (default: "modern")
	DefaultHttpClient string // Default code sample language (default: "curl")
	HideModels        bool   // Hide schema models section
	ShowSidebar       bool   // Show navigation sidebar (default: true)
	CustomCSS         string // Additional CSS to inject
}

func defaultScalarConfig() ScalarConfig {
	return ScalarConfig{
		Theme:             "purple",
		DarkMode:          true,
		Layout:            "modern",
		DefaultHttpClient: "curl",
		ShowSidebar:       true,
	}
}

// ServeDocs registers routes to serve the Scalar API reference UI and the OpenAPI spec.
// It mounts:
//   - GET {basePath}           → Scalar UI
//   - GET {basePath}/openapi.json → generated OpenAPI JSON spec
func (r *Router) ServeDocs(basePath string, configs ...ScalarConfig) {
	cfg := defaultScalarConfig()
	if len(configs) > 0 {
		cfg = mergeConfig(cfg, configs[0])
	}

	basePath = strings.TrimRight(basePath, "/")

	// Serve the OpenAPI spec as JSON.
	r.engine.GET(basePath+"/openapi.json", func(c *gin.Context) {
		spec := r.generateSpec()
		c.JSON(http.StatusOK, spec)
	})

	// Serve the Scalar UI.
	html := buildScalarHTML(basePath, r.info.Title, cfg)
	r.engine.GET(basePath, func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
	})
}

func mergeConfig(base, override ScalarConfig) ScalarConfig {
	if override.Theme != "" {
		base.Theme = override.Theme
	}
	base.DarkMode = override.DarkMode
	if override.Layout != "" {
		base.Layout = override.Layout
	}
	if override.DefaultHttpClient != "" {
		base.DefaultHttpClient = override.DefaultHttpClient
	}
	base.HideModels = override.HideModels
	base.ShowSidebar = override.ShowSidebar
	if override.CustomCSS != "" {
		base.CustomCSS = override.CustomCSS
	}
	return base
}

func buildScalarHTML(basePath, title string, cfg ScalarConfig) string {
	darkModeState := "dark"
	if !cfg.DarkMode {
		darkModeState = "light"
	}

	configMap := map[string]any{
		"theme":  cfg.Theme,
		"layout": cfg.Layout,
		"defaultHttpClient": map[string]any{
			"targetKey": "shell",
			"clientKey": cfg.DefaultHttpClient,
		},
		"hideModels":         cfg.HideModels,
		"showSidebar":        cfg.ShowSidebar,
		"darkMode":           cfg.DarkMode,
		"forceDarkModeState": darkModeState,
	}

	configJSON, _ := json.Marshal(configMap)

	customStyle := ""
	if cfg.CustomCSS != "" {
		customStyle = fmt.Sprintf("<style>%s</style>", cfg.CustomCSS)
	}

	return fmt.Sprintf(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>%s</title>
    <style>
      :root {
        --scalar-font: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto,
          Oxygen, Ubuntu, Cantarell, "Open Sans", "Helvetica Neue", sans-serif;
      }
    </style>
    %s
  </head>
  <body>
    <script
      id="api-reference"
      data-url="%s/openapi.json"
      data-configuration='%s'
    ></script>
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
  </body>
</html>`, title, customStyle, basePath, string(configJSON))
}
