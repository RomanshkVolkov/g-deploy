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

	"github.com/RomanshkVolkov/g-deploy/internal/detector"
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
	template    string
	vars        kvFlag
}

// kvFlag implements flag.Value to collect repeated --var KEY=VALUE arguments.
type kvFlag map[string]string

func (kv *kvFlag) String() string {
	if kv == nil || len(*kv) == 0 {
		return ""
	}
	parts := make([]string, 0, len(*kv))
	for k, v := range *kv {
		parts = append(parts, k+"="+v)
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func (kv *kvFlag) Set(raw string) error {
	idx := strings.IndexByte(raw, '=')
	if idx <= 0 {
		return fmt.Errorf("expected KEY=VALUE, got %q", raw)
	}
	key := raw[:idx]
	val := raw[idx+1:]
	if *kv == nil {
		*kv = make(map[string]string)
	}
	(*kv)[key] = val
	return nil
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
	fs.StringVar(&cfg.template, "template", "", "Path to a custom deployment template (overrides .deploy/deployment.<proxy>.template.yml)")
	fs.Var(&cfg.vars, "var", "Custom KEY=VALUE template substitution (repeatable). Built-in *_PLACEHOLDER keys take priority.")

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

	// Resolve template path: explicit --template flag → default .deploy/deployment.<proxy>.template.yml
	templatePath := cfg.template
	if templatePath == "" {
		templatePath = fmt.Sprintf(".deploy/deployment.%s.template.yml", cfg.proxy)
	}
	templateBytes, err := os.ReadFile(templatePath)
	if err != nil {
		if cfg.template != "" {
			return fmt.Errorf("template not found: %s", templatePath)
		}
		return fmt.Errorf("template not found: %s\nRun 'g-deploy init' first to scaffold the project", templatePath)
	}

	content := buildContent(string(templateBytes), cfg, serviceEnvs)

	if err := os.WriteFile(cfg.output, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", cfg.output, err)
	}

	logf("[*] Deployment file generated: %s (proxy: %s, port: %d, template: %s)", cfg.output, cfg.proxy, cfg.port, templatePath)
	return nil
}

// buildContent applies all placeholder substitutions and injects DEPLOY_* env vars.
func buildContent(tmpl string, cfg config, serviceEnvs map[string][]string) string {
	stackFull := cfg.stack + "-" + cfg.environment

	// Built-in placeholders — these always win over user-provided --var entries.
	builtins := map[string]string{
		"STACK_PLACEHOLDER": stackFull,
		"IMAGE_PLACEHOLDER": cfg.image,
		"HOST_PLACEHOLDER":  cfg.host,
		"PORT_PLACEHOLDER":  strconv.Itoa(cfg.port),
		"TLS_PLACEHOLDER":   cfg.tls,
	}

	// Apply user --var substitutions first, skipping any key that collides with a built-in.
	content := tmpl
	if len(cfg.vars) > 0 {
		userPairs := make([]string, 0, len(cfg.vars)*2)
		keys := make([]string, 0, len(cfg.vars))
		for k := range cfg.vars {
			if _, clash := builtins[k]; clash {
				logf("[!] Ignoring --var %s: collides with built-in placeholder", k)
				continue
			}
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			userPairs = append(userPairs, k, cfg.vars[k])
		}
		if len(userPairs) > 0 {
			content = strings.NewReplacer(userPairs...).Replace(content)
		}
	}

	r := strings.NewReplacer(
		"STACK_PLACEHOLDER", builtins["STACK_PLACEHOLDER"],
		"IMAGE_PLACEHOLDER", builtins["IMAGE_PLACEHOLDER"],
		"HOST_PLACEHOLDER", builtins["HOST_PLACEHOLDER"],
		"PORT_PLACEHOLDER", builtins["PORT_PLACEHOLDER"],
		"TLS_PLACEHOLDER", builtins["TLS_PLACEHOLDER"],
	)
	content = r.Replace(content)

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
