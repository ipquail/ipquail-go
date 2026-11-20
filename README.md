# ipquail-go

A Go implementation of [ipquail.com](https://ipquail.com). It provides
plain-text and JSON endpoints for looking up the client IP address, reverse DNS,
and ASN information. The HTTP API uses Go's standard library and has no
third-party Go dependencies.

## Quick start

Requires Go 1.27.3 or newer.

```sh
go build -o ipquail-go ./ipquail
./ipquail-go
```

The server listens on port 3000. For a local check, visit `http://localhost:3000/ip`.

## Documentation

- [Installation and deployment](INSTALL.md), including Docker and reverse proxy setup.
- [Development](DEVELOPING.md), including build, frontend, and maintenance commands.
- [Deployment samples](deploy/README.md) for nginx, Apache, and Caddy.
- [Docker details](DOCKER.md).

## License

See [LICENSE.md](LICENSE.md).
