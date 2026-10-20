# ipquail-go #

Here is a Go version of ipquail.com. The backend uses Go's standard `net/http` package.

Go 1.27.3 or newer is required.

	cd ipquail
	go build
	./ipquail

## nginx config ##

The backend listens on port 3000 and accepts its nginx-supplied `X-Real-IP` header only from loopback or private-network peers, including a reverse proxy on a private container network.

The public service intentionally exposes the same plaintext and JSON endpoints over both HTTP and HTTPS. Port 80 is not redirected because longstanding clients and older devices depend on direct, shell-friendly HTTP responses; clients that support TLS should use port 443.

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
	}

## dns zone config ##

The front-end web application uses specific IPv4 and IPv6 only hostnames. The base hostname should be both IPv4 and IPv6 capable.

	$ORIGIN ipquail.com.
		A		<ip4>
		AAAA	<ip6>
	4	A		<ip4>
	6	AAAA	<ip6>

## Backend

The backend listens on port 3000 and is accessed through the nginx reverse proxy shown above. Keep it private or publish it only on the host loopback interface; clients must not be able to supply the trusted `X-Real-IP` header directly. It has no third-party Go dependencies.
