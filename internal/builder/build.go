// Package builder generates Docker Stack deployment YAML files from templates.
// It replaces the original build_deployment.sh bash script with a dependency-free
// Go implementation (no yq required).
package builder

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/RomanshkVolkov/guz-deploy/internal/detector"
)

type config struct {
	stack       string
	environment string
	image       string
	host        string
	proxy       string
	port        int
	tls         string
	output      string
}

// Run parses args and generates the deployment YAML file.
// Args mirror the old build_deployment.sh flags plus -p for proxy and --port for port override.
func Run(args []string) error {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)

	var cfg config
	var portStr string

	fs.StringVar(&cfg.stack, "s", "", "Stack name (required)")
	fs.StringVar(&cfg.environment, "e", "", "Environment, e.g. dev or prod (required)")
	fs.StringVar(&cfg.image, "i", "", "Docker image URL (required)")
	fs.StringVar(&cfg.host, "h", "", "Host domain (required)")
	fs.StringVar(&cfg.proxy, "p", "caddy", "Proxy: caddy or traefik (default: caddy)")
	fs.StringVar(&portStr, "port", "", "Container port — auto-detected from framework when omitted")
	fs.StringVar(&cfg.tls, "t", "internal", "TLS value (Caddy only, e.g. email or 'internal')")
	fs.StringVar(&cfg.output, "o", "", "Output file path (required)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Validate required flags
	missing := validateRequired(map[string]string{
		"-s": cfg.stack,
		"-e": cfg.environment,
		"-i": cfg.image,
		"-h": cfg.host,
		"-o": cfg.output,
	})
	if len(missing) > 0 {
		return fmt.Errorf("missing required args: %s", strings.Join(missing, ", "))
	}

	if cfg.proxy != "caddy" && cfg.proxy != "traefik" {
		return fmt.Errorf("-p must be 'caddy' or 'traefik', got: %q", cfg.proxy)
	}

	// Resolve port: explicit flag → auto-detect from framework → default 3000
	cfg.port = resolvePort(portStr)

	// Collect DEPLOY_<SERVICE>_<VAR>=<value> environment variables
	serviceEnvs := collectDeployEnvs()

	// Load template from the project's .deploy/ directory (allows per-project customisation)
	templatePath := fmt.Sprintf(".deploy/deployment.%s.template.yml", cfg.proxy)
	templateBytes, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("template not found: %s\nRun 'guz-deploy init' first to scaffold the project", templatePath)
	}

	content := buildContent(string(templateBytes), cfg, serviceEnvs)

	if err := os.WriteFile(cfg.output, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", cfg.output, err)
	}

	logf("[*] Deployment file generated: %s (proxy: %s, port: %d)", cfg.output, cfg.proxy, cfg.port)
	return nil
}

// buildContent applies all placeholder substitutions and injects DEPLOY_* env vars.
func buildContent(tmpl string, cfg config, serviceEnvs map[string][]string) string {
	stackFull := cfg.stack + "-" + cfg.environment

	r := strings.NewReplacer(
		"STACK_PLACEHOLDER", stackFull,
		"IMAGE_PLACEHOLDER", cfg.image,
		"HOST_PLACEHOLDER", cfg.host,
		"PORT_PLACEHOLDER", strconv.Itoa(cfg.port),
		"TLS_PLACEHOLDER", cfg.tls,
	)
	content := r.Replace(tmpl)

	// Inject env vars per service via ##ENVS:serviceSuffix## markers
	// Marker pattern: ##ENVS:app## → replaced with "      - VAR=val" lines
	// from DEPLOY_app_VAR=val environment variables.
	markerRe := regexp.MustCompile(`[ \t]*##ENVS:([a-zA-Z0-9_]+)##\n?`)
	content = markerRe.ReplaceAllStringFunc(content, func(match string) string {
		sub := markerRe.FindStringSubmatch(match)
		serviceSuffix := strings.ToLower(sub[1])

		vars := serviceEnvs[serviceSuffix]
		if len(vars) == 0 {
			return ""
		}

		sorted := make([]string, len(vars))
		copy(sorted, vars)
		sort.Strings(sorted)

		var lines []string
		for _, v := range sorted {
			lines = append(lines, "      - "+v)
		}
		return strings.Join(lines, "\n") + "\n"
	})

	return content
}

// collectDeployEnvs reads DEPLOY_<SERVICE>_<VAR>=<value> from the process environment.
// Returns a map of lowercased service suffix → []"VAR=value".
var deployEnvRe = regexp.MustCompile(`^DEPLOY_([A-Za-z0-9]+)_([A-Za-z0-9_]+)$`)

func collectDeployEnvs() map[string][]string {
	result := make(map[string][]string)
	for _, e := range os.Environ() {
		idx := strings.IndexByte(e, '=')
		if idx < 0 {
			continue
		}
		key, val := e[:idx], e[idx+1:]
		m := deployEnvRe.FindStringSubmatch(key)
		if m == nil {
			continue
		}
		svc := strings.ToLower(m[1])
		varName := m[2]
		result[svc] = append(result[svc], varName+"="+val)
		logf("[*] Injecting %s into service %s", varName, svc)
	}
	return result
}

// resolvePort returns the port to use: explicit string → framework detection → 3000.
func resolvePort(explicit string) int {
	if explicit != "" {
		if p, err := strconv.Atoi(explicit); err == nil && p > 0 {
			return p
		}
	}
	cwd, err := os.Getwd()
	if err == nil {
		if fw := detector.Detect(cwd); fw != nil {
			logf("[*] Auto-detected port %d from framework: %s", fw.Port, fw.Name)
			return fw.Port
		}
	}
	return 3000
}

func validateRequired(fields map[string]string) []string {
	var missing []string
	for flag, val := range fields {
		if val == "" {
			missing = append(missing, flag)
		}
	}
	sort.Strings(missing)
	return missing
}

func logf(format string, args ...any) {
	fmt.Printf(format+"\n", args...)
}
