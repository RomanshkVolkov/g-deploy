// Package assets embeds all scaffold files (Dockerfiles, CI templates, deployment
// templates) that are copied into target projects by the init command.
package assets

import "embed"

//go:embed all:.deploy all:.github .dockerignore
//go:embed Dockerfile.golang Dockerfile.svelte-node-adapter Dockerfile.nextjs
//go:embed Dockerfile.deno Dockerfile.api Dockerfile.api.only-js
//go:embed Dockerfile.NET.sdk-7 Dockerfile.angular
var FS embed.FS
