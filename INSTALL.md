# Installation

## Requirements

The Go backend requires Go 1.27.3 or newer and has no third-party Go module
dependencies. The static website is served separately from `ipquail/public`.
To rebuild its vendored Bootstrap and jQuery assets, see [DEVELOPING.md](DEVELOPING.md).

## Build and run

From the repository root:

```sh
go build -o ipquail-go ./ipquail
./ipquail-go
```

The backend listens on port 3000. It provides `/ip`, `/ptr`, `/asn`, `/api/`,
`/health`, and `/metrics`. `/health` is intended for local monitoring and
`/metrics` should be limited to trusted monitoring clients.

## Reverse proxy and static files

For production, serve `ipquail/public` from your web server and proxy `/ip`,
`/ptr`, `/asn`, and `/api/` to the backend. Keep the backend private or bound
to loopback. The proxy must overwrite `X-Real-IP` with the actual client
address: the backend accepts that header only from loopback or private-network
peers. Do not expose `/health` publicly, and restrict `/metrics` to local or
trusted monitoring clients.

The service intentionally serves the same plaintext and JSON endpoints over
HTTP and HTTPS. Port 80 is not redirected because existing clients depend on
direct HTTP responses; clients that support TLS should use HTTPS.

Ready-to-adapt web server configurations and a Docker Compose deployment are
in [`deploy/`](deploy/README.md). Copy the generated `ipquail/public` directory
to the document root, replace the example hostnames and paths, and configure
TLS for your site. For container image builds, health checks, and publishing,
see [DOCKER.md](DOCKER.md).

## DNS

The frontend expects the main hostname to support both address families, with
separate IPv4-only and IPv6-only hostnames:

```dns
$ORIGIN ipquail.com.
        A       <ipv4-address>
        AAAA    <ipv6-address>
4       A       <ipv4-address>
6       AAAA    <ipv6-address>
```

Replace the example domain and addresses with those for your deployment.
