package detector

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Framework holds the detected framework metadata.
type Framework struct {
	Name       string
	Dockerfile string
	Port       int
}

// Detect inspects dir and returns the matching framework, or nil if unrecognised.
func Detect(dir string) *Framework {
	if fileExists(filepath.Join(dir, "go.mod")) {
		return &Framework{"golang", "Dockerfile.golang", 8080}
	}

	if globExists(dir, "*.csproj") {
		return &Framework{"dotnet", "Dockerfile.NET.sdk-7", 80}
	}

	if fileExists(filepath.Join(dir, "deno.json")) || globExists(dir, "deno.*") {
		return &Framework{"deno", "Dockerfile.deno", 8000}
	}

	deps := readPackageDeps(filepath.Join(dir, "package.json"))
	if deps != nil {
		switch {
		case hasPrefix(deps, "@sveltejs/adapter-node"):
			return &Framework{"svelte-node", "Dockerfile.svelte-node-adapter", 3000}
		case hasPrefix(deps, "next"):
			return &Framework{"nextjs", "Dockerfile.nextjs", 3000}
		case hasPrefix(deps, "@angular"):
			return &Framework{"angular", "Dockerfile.angular", 80}
		case hasPrefix(deps, "react"):
			return &Framework{"react", "Dockerfile.angular", 80}
		case hasPrefix(deps, "@nestjs"):
			return &Framework{"nestjs", "Dockerfile.api", 8000}
		case hasPrefix(deps, "express"):
			return &Framework{"express", "Dockerfile.api", 8000}
		case hasPrefix(deps, "koa"):
			return &Framework{"koa", "Dockerfile.api.only-js", 8000}
		}
	}

	return nil
}

func readPackageDeps(pkgPath string) map[string]string {
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return nil
	}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}

	merged := make(map[string]string, len(pkg.Dependencies)+len(pkg.DevDependencies))
	for k, v := range pkg.Dependencies {
		merged[k] = v
	}
	for k, v := range pkg.DevDependencies {
		merged[k] = v
	}
	return merged
}

func hasPrefix(deps map[string]string, prefix string) bool {
	for k := range deps {
		if strings.HasPrefix(k, prefix) {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func globExists(dir, pattern string) bool {
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	return err == nil && len(matches) > 0
}
