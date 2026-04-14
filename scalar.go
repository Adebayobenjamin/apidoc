package apidoc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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
	// FallbackToFile, when true, causes ServeDocs to fall back to serving the
	// previously generated openapi.json on disk if regeneration fails at
	// startup or at request time. Defaults to true.
	FallbackToFile *bool
}

func defaultScalarConfig() ScalarConfig {
	t := true
	return ScalarConfig{
		Theme:             "purple",
		DarkMode:          true,
		Layout:            "modern",
		DefaultHttpClient: "curl",
		ShowSidebar:       true,
		FallbackToFile:    &t,
	}
}

// ServeDocs generates the OpenAPI spec, writes it to {basePath}/openapi.json
// relative to the working directory, and registers Gin routes to serve both
// the spec and the Scalar API reference UI.
//
// It mounts:
//   - GET {basePath}              → Scalar UI
//   - GET {basePath}/openapi.json → generated OpenAPI JSON spec
//
// The file is written once at startup; the served endpoint always reflects
// the current spec (regenerated on each request).
func (r *Router) ServeDocs(basePath string, configs ...ScalarConfig) error {
	cfg := defaultScalarConfig()
	if len(configs) > 0 {
		cfg = mergeConfig(cfg, configs[0])
	}

	basePath = strings.TrimRight(basePath, "/")

	fallback := true
	if cfg.FallbackToFile != nil {
		fallback = *cfg.FallbackToFile
	}

	// Write the spec to disk.
	filePath := strings.TrimPrefix(basePath, "/") + "/openapi.json"
	saveErr := r.SaveDocs(filePath)
	if saveErr != nil {
		if !fallback {
			return fmt.Errorf("apidoc: ServeDocs failed to save spec: %w", saveErr)
		}
		// Fallback: only acceptable if an existing spec file is on disk.
		if _, statErr := os.Stat(filePath); statErr != nil {
			return fmt.Errorf("apidoc: ServeDocs failed to save spec and no existing file at %s: %w", filePath, saveErr)
		}
	}

	// Serve the OpenAPI spec as JSON. Falls back to the on-disk file if
	// regeneration fails and fallback is enabled.
	r.engine.GET(basePath+"/openapi.json", func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				if fallback {
					if data, err := os.ReadFile(filePath); err == nil {
						c.Data(http.StatusOK, "application/json", data)
						return
					}
				}
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": fmt.Sprintf("apidoc: failed to generate spec: %v", rec),
				})
			}
		}()
		spec := r.generateSpec()
		c.JSON(http.StatusOK, spec)
	})

	// Serve the Scalar UI.
	html := buildScalarHTML(basePath, r.info.Title, cfg)
	r.engine.GET(basePath, func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
	})

	return nil
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
