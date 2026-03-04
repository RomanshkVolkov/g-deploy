package main

import (
	"fmt"
	"os"

	"github.com/RomanshkVolkov/g-deploy/internal/builder"
	"github.com/RomanshkVolkov/g-deploy/internal/initializer"
)

// version is set at build time via -ldflags "-X main.version=vX.Y.Z"
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "init":
		err = initializer.Run()
	case "build":
		err = builder.Run(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println("g-deploy v" + version)
		return
	default:
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "[x] %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "g-deploy - Docker Swarm deployment scaffold tool")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Usage: g-deploy <command> [options]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  init    Detect framework and copy deployment files to current project")
	fmt.Fprintln(os.Stderr, "  build   Generate Docker Stack deployment YAML from template")
	fmt.Fprintln(os.Stderr, "  version Print version")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Run 'g-deploy <command> -help' for command usage.")
}
