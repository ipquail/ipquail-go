# Developing

## Requirements

- Go 1.27.3 or newer
- Node.js 24.x and npm, if rebuilding the static website assets
- `nvm` by default for the Makefile's frontend targets (`NVM_DIR` can be overridden)

## Build and run

The Makefile builds frontend dependencies/assets before compiling the backend:

```sh
make build
make run
```

To build only the Go backend, without rebuilding frontend assets:

```sh
go build -o bin/ipquail ./ipquail
go run ./ipquail
```

The frontend dependencies are pinned in `package-lock.json`. Install and build
them directly with:

```sh
npm ci
npm run build
```

## Checks

```sh
make test
make fmt
make vet
make verify
```

`make verify` checks Go formatting and module tidiness. Run `make clean` to
remove generated binary and frontend dependency assets.

## Project notes

- The HTTP backend is in `ipquail/`; static website files are in `ipquail/public/`.
- The backend uses Go's standard library and has no third-party Go dependencies.
- `deploy/` contains sample host web server configs and a production Compose setup.
- `DOCKER.md` documents the container build and release image behavior.
