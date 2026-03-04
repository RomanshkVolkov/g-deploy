# g-deploy

CLI tool para scaffoldear CI/CD con Docker Swarm. Detecta el framework del proyecto, copia el Dockerfile correcto y genera los archivos de deployment para **Caddy** o **Traefik**.

## Instalación

```sh
curl -fsSL https://raw.githubusercontent.com/RomanshkVolkov/g-deploy/main/install.sh | sh
```

El script detecta tu OS y arquitectura automáticamente (Linux, macOS Intel, macOS Apple Silicon) e instala el binario en `~/.local/bin` o `/usr/local/bin` si eres root.

### Actualizar

El mismo comando sirve para actualizar. Si ya tienes la última versión, el script lo detecta y no hace nada:

```sh
curl -fsSL https://raw.githubusercontent.com/RomanshkVolkov/g-deploy/main/install.sh | sh
# [✓] Already up to date: v2.1.0
```

### Verificar instalación

```sh
g-deploy version
```

---

## Uso: `init`

Ejecuta `init` **dentro del directorio raíz de tu proyecto**. Detecta el framework y copia los archivos necesarios.

```sh
cd mi-proyecto/
g-deploy init
```

### Qué copia

| Archivo / carpeta | Descripción |
|---|---|
| `.deploy/deployment.caddy.template.yml` | Template de Docker Stack para Caddy |
| `.deploy/deployment.traefik.template.yml` | Template de Docker Stack para Traefik |
| `.github/workflows/dev.yml` | Workflow de GitHub Actions para deploy a dev |
| `Dockerfile` | Dockerfile del framework detectado |
| `.dockerignore` | Ignorados de Docker |

### Frameworks detectados

| Framework | Detección | Puerto |
|---|---|---|
| Go | `go.mod` | 8080 |
| .NET | `*.csproj` | 80 |
| Deno | `deno.json` | 8000 |
| SvelteKit (node) | `@sveltejs/adapter-node` en package.json | 3000 |
| Next.js | `next` en package.json | 3000 |
| Angular / React | `@angular` o `react` en package.json | 3000 |
| NestJS | `@nestjs` en package.json | 8000 |
| Express | `express` en package.json | 8000 |
| Koa | `koa` en package.json | 8000 |

---

## Uso: `build`

Genera el archivo YAML de Docker Stack a partir del template. Se llama desde el workflow de CI, no manualmente.

```sh
g-deploy build \
  -s <stack>       \  # nombre del stack (ej. mi-proyecto)
  -e <environment> \  # entorno (ej. dev, prod)
  -i <image>       \  # imagen Docker completa con tag
  -h <host>        \  # dominio (ej. mi-app.com)
  -p <proxy>       \  # caddy o traefik (default: caddy)
  -t <tls>         \  # solo Caddy: email o "internal" (default: internal)
  -o <output>         # ruta del archivo generado
```

El port se auto-detecta del framework. Se puede sobreescribir con `--port`.

### Variables de entorno para los servicios

Para inyectar secrets en el deployment usa el formato `DEPLOY_<servicio>_<VARIABLE>`:

```sh
export DEPLOY_app_DATABASE_URL="postgres://..."
export DEPLOY_app_SECRET_KEY="abc123"

g-deploy build -s mi-proyecto -e dev -i ghcr.io/... -h mi-app.com -p traefik -o deployment.yml
```

El resultado en el YAML:
```yaml
environment:
  - PORT=3000
  - DATABASE_URL=postgres://...
  - SECRET_KEY=abc123
```

---

## Workflow de GitHub Actions

Después de `g-deploy init`, tu proyecto tendrá `.github/workflows/dev.yml` listo para usar. Solo necesitas configurar los secrets y variables en tu repositorio de GitHub.

### Secrets requeridos

| Secret | Descripción |
|---|---|
| `HOST_SSH_PRIVATE_KEY` | Clave SSH privada para conectar al servidor |
| `HOST_SSH_NAME` | IP o hostname del servidor |
| `HOST_SSH_PORT` | Puerto SSH (normalmente 22) |
| `HOST_SSH_USERNAME` | Usuario SSH |

### Variables requeridas

| Variable | Descripción | Ejemplo |
|---|---|---|
| `HOST_DNS` | Dominio de la aplicación | `mi-app.com` |
| `PROXY` | Proxy a usar | `caddy` o `traefik` |
| `TLS` | Solo para Caddy: email o método TLS | `admin@mi-app.com` o `internal` |

### Agregar secrets de la aplicación

En el step `Build deployment file` del workflow, agrega tus secrets con el formato `DEPLOY_app_*`:

```yaml
- name: Build deployment file
  env:
    DEPLOY_app_DATABASE_URL: ${{ secrets.DATABASE_URL }}
    DEPLOY_app_JWT_SECRET: ${{ secrets.JWT_SECRET }}
  run: |
    g-deploy build ...
```

### Flujo completo del workflow

```
push a dev
    │
    ├── Checkout
    ├── Login a ghcr.io
    ├── docker build + push (tag = SHA corto del commit)
    ├── g-deploy build → genera deployment.yml
    ├── scp → copia deployment.yml al servidor
    ├── docker pull (en el servidor)
    ├── docker stack deploy (en el servidor)
    └── limpieza de archivos temporales
```

---

## Proxy: Caddy vs Traefik

### Caddy
El servidor debe tener Caddy corriendo en modo proxy con Docker Swarm. El compose generado se conecta a la red `caddy` y usa labels de Caddy.

### Traefik
El servidor debe tener Traefik corriendo con el entrypoint `web` (80), `websecure` (443) y un `certresolver` llamado `myresolver`. El compose generado se conecta a la red `traefik_proxy`.

Los templates en `.deploy/` son editables — puedes personalizar reglas, middlewares o agregar más servicios al stack.
