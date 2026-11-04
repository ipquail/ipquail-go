# ipquail-go #

Here is a Go version of ipquail.com. The backend uses Go's standard `net/http` package.

Go 1.27.3 or newer is required.

	cd ipquail
	go build
	./ipquail

## Deployment

Build the backend with `go build -o ipquail-go ./ipquail` and run it as a
service bound to port 3000. Deploy the generated `ipquail/public` directory to
the web server's document root. The backend should be reachable only by the
local web server or a private container network: it accepts `X-Real-IP` only
from loopback or private-network peers, so the reverse proxy must overwrite
that header with the actual client address.

Sample host configurations and a production Compose setup are in
[`deploy/`](deploy/). Follow [deploy/README.md](deploy/README.md) to pin a
release, start the backend on loopback, and configure a host-installed web
server. Copy `ipquail/public` to `/var/www/ipquail/public` or update the web
server config's document root, and change the sample hostnames before use.

The public service intentionally exposes the same plaintext and JSON endpoints
over both HTTP and HTTPS. Port 80 is not redirected because longstanding
clients and older devices depend on direct, shell-friendly HTTP responses;
clients that support TLS should use port 443. The examples below keep `/metrics`
available only to local clients. Change that restriction only if you have a
specific, trusted metrics scraper.

### nginx config

The example below assumes nginx and the backend run on the same host. Replace
the document root and certificate paths as needed.

	map $http_user_agent $ipquailindex {
		default /index.html;
		~curl /ip;
		~Python-urllib /ip;
		~OpenBSD /ip;
		~Go-http-client /ip;
	}

	server {
		listen 80;
		listen [::]:80 ipv6only=on;

		root /home/website/ipquail-go/ipquail/public;

		include /etc/nginx/cloudflare;

		charset utf-8;

		server_name ipquail.com 4.ipquail.com 6.ipquail.com www.ipquail.com;

		access_log /var/log/nginx/ipquail_com_access.log;
		error_log /var/log/nginx/ipquail_com_error.log;

		# Special endpoint depending on useragent
		# rewritten index
		location = / { rewrite ^ $ipquailindex; }

		# staticly serve files
		location / {
				try_files $uri $uri/ $uri.html =404;
		}

		# revproxy endpoints
		location /ip {
			proxy_pass              http://127.0.0.1:3000/ip;
			proxy_set_header        X-Real-IP $remote_addr;
			include                 proxy_params;
			proxy_headers_hash_bucket_size  256;
		}
		location /ptr {
			proxy_pass              http://127.0.0.1:3000/ptr;
			proxy_set_header        X-Real-IP $remote_addr;
			include                 proxy_params;
			proxy_headers_hash_bucket_size  256;
		}
		location /asn {
			proxy_pass              http://127.0.0.1:3000/asn;
			proxy_set_header        X-Real-IP $remote_addr;
			include                 proxy_params;
			proxy_headers_hash_bucket_size  256;
		}
		location /api/ {
			proxy_pass              http://127.0.0.1:3000/api/;
			proxy_set_header        X-Real-IP $remote_addr;
			include                 proxy_params;
			proxy_headers_hash_bucket_size  256;
		}
		location = /metrics {
			allow                   127.0.0.1;
			allow                   ::1;
			deny                    all;
			proxy_pass              http://127.0.0.1:3000/metrics;
			include                 proxy_params;
		}
	}

	server {
		listen 443 ssl http2;
		listen [::]:443 ssl http2 ipv6only=on;

		server_name ipquail.com 4.ipquail.com 6.ipquail.com www.ipquail.com;

		root /home/website/ipquail-go/ipquail/public;

		include /etc/nginx/cloudflare;

		charset utf-8;

		access_log /var/log/nginx/ipquail_com_access.log;
		error_log /var/log/nginx/ipquail_com_error.log;

		ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;

		ssl_certificate /etc/letsencrypt/live/ipquail.com/fullchain.pem;
		ssl_certificate_key /etc/letsencrypt/live/ipquail.com/privkey.pem;

		ssl_session_timeout 1d;
		ssl_session_cache shared:MozSSL:10m;  # about 40000 sessions
		ssl_session_tickets off;

		# intermediate configuration
		ssl_protocols TLSv1.2 TLSv1.3;
	    ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-CHACHA20-POLY1305:ECDHE-RSA-CHACHA20-POLY1305;
		ssl_ecdh_curve X25519MLKEM768:X25519:prime256v1:secp384r1;
		ssl_prefer_server_ciphers off;

		# rewritten index
		location = / { rewrite ^ $ipquailindex; }

		# staticly serve files
		location / {
			try_files $uri $uri/ $uri.html =404;
		}

		# revproxy endpoints
		location /ip {
			proxy_pass              http://127.0.0.1:3000/ip;
			proxy_set_header        X-Real-IP $remote_addr;
			include                 proxy_params;
			proxy_headers_hash_bucket_size  256;
		}
		location /ptr {
			proxy_pass              http://127.0.0.1:3000/ptr;
			proxy_set_header        X-Real-IP $remote_addr;
			include                 proxy_params;
			proxy_headers_hash_bucket_size  256;
		}
		location /asn {
			proxy_pass              http://127.0.0.1:3000/asn;
			proxy_set_header        X-Real-IP $remote_addr;
			include                 proxy_params;
			proxy_headers_hash_bucket_size  256;
		}
		location /api/ {
			proxy_pass              http://127.0.0.1:3000/api/;
			proxy_set_header        X-Real-IP $remote_addr;
			include                 proxy_params;
			proxy_headers_hash_bucket_size  256;
		}
		location = /metrics {
			allow                   127.0.0.1;
			allow                   ::1;
			deny                    all;
			proxy_pass              http://127.0.0.1:3000/metrics;
			include                 proxy_params;
		}
	}

### Apache httpd

Enable the `rewrite`, `proxy`, `proxy_http`, and `remoteip` modules. Configure
`mod_remoteip` to trust only the local backend proxy, so `%a` contains the
client address. For example, add this to the server configuration:

```apache
RemoteIPHeader X-Forwarded-For
RemoteIPInternalProxy 127.0.0.1
RemoteIPInternalProxy ::1
```

Place the following directives inside the HTTPS virtual host (and repeat them
in the port 80 virtual host). Set `DocumentRoot` to the deployed `public`
directory and set the hostname and certificate paths for your site. Apache
does not redirect port 80 to HTTPS here.

```apache
DocumentRoot /home/website/ipquail-go/ipquail/public
ServerName ipquail.com
ServerAlias 4.ipquail.com 6.ipquail.com www.ipquail.com

<Directory /home/website/ipquail-go/ipquail/public>
    Require all granted
    Options -Indexes
    AllowOverride None
</Directory>

RewriteEngine On
# Preserve the user-agent-dependent root response.
RewriteCond %{REQUEST_URI} ^/$
RewriteCond %{HTTP_USER_AGENT} "(curl|Python-urllib|OpenBSD|Go-http-client)" [NC]
RewriteRule ^/$ /ip [PT,L]

# Proxy API endpoints to the Go backend; do not expose its health endpoint.
ProxyPreserveHost On
RequestHeader set X-Real-IP "%a"
ProxyPass        /api/ http://127.0.0.1:3000/api/
ProxyPassReverse /api/ http://127.0.0.1:3000/api/
ProxyPass        /ip http://127.0.0.1:3000/ip
ProxyPassReverse /ip http://127.0.0.1:3000/ip
ProxyPass        /ptr http://127.0.0.1:3000/ptr
ProxyPassReverse /ptr http://127.0.0.1:3000/ptr
ProxyPass        /asn http://127.0.0.1:3000/asn
ProxyPassReverse /asn http://127.0.0.1:3000/asn

# Keep metrics local to the web server.
<Location /metrics>
    Require local
    ProxyPass http://127.0.0.1:3000/metrics
    ProxyPassReverse http://127.0.0.1:3000/metrics
</Location>
```

With `mod_remoteip`, `%a` is the resolved client address. For deployments
behind another trusted proxy (such as Cloudflare), configure that proxy's
addresses in `mod_remoteip` and ensure it overwrites `X-Forwarded-For`.

### Caddy

Set `root` to the deployed `ipquail/public` directory and replace the example
hostnames. Caddy obtains and renews certificates automatically. The separate
HTTP site block intentionally serves HTTP without redirecting to HTTPS.

```caddyfile
(ipquail) {
	root * /home/website/ipquail-go/ipquail/public
	@backend path /ip /ptr /asn /api/*
	handle @backend {
		reverse_proxy 127.0.0.1:3000 {
			header_up X-Real-IP {http.request.remote.host}
		}
	}
	@metrics path /metrics
	handle @metrics {
		@local remote_ip 127.0.0.1 ::1
		handle @local {
			reverse_proxy 127.0.0.1:3000
		}
		respond "Forbidden" 403
	}
	@commandline {
		path /
		header_regexp User-Agent User-Agent "(?i)(curl|Python-urllib|OpenBSD|Go-http-client)"
	}
	rewrite @commandline /ip
	file_server
}

http://ipquail.com, http://4.ipquail.com, http://6.ipquail.com, http://www.ipquail.com {
	import ipquail
}

https://ipquail.com, https://4.ipquail.com, https://6.ipquail.com, https://www.ipquail.com {
	import ipquail
}
```

Caddy's `{http.request.remote.host}` value sets `X-Real-IP` to the address of
the connection directly to Caddy. If Caddy sits behind a CDN or load balancer,
configure Caddy's trusted-proxy handling first and use the trusted client IP
placeholder supported by your Caddy version.

## dns zone config ##

The front-end web application uses specific IPv4 and IPv6 only hostnames. The base hostname should be both IPv4 and IPv6 capable.

	$ORIGIN ipquail.com.
		A		<ip4>
		AAAA	<ip6>
	4	A		<ip4>
	6	AAAA	<ip6>

## Backend

The backend listens on port 3000 and is accessed through the nginx reverse proxy shown above. Keep it private or publish it only on the host loopback interface; clients must not be able to supply the trusted `X-Real-IP` header directly. `/metrics` is proxied only for loopback clients in the nginx examples. It has no third-party Go dependencies.
