# syntax=docker/dockerfile:1

FROM golang:1.27.3-alpine AS build
WORKDIR /src

# ipquail-go currently has no third-party Go dependencies and no go.sum.
COPY go.mod ./
COPY ipquail/*.go ./ipquail/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ipquail-go ./ipquail

FROM alpine:3.24
RUN apk add --no-cache ca-certificates \
    && addgroup -g 65532 -S ipquail \
    && adduser -u 65532 -S -G ipquail -h /nonexistent -s /sbin/nologin ipquail

COPY --from=build /out/ipquail-go /usr/local/bin/ipquail-go

USER 65532:65532

# The process listens on port 3000 on the container's network interfaces.
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:3000/health || exit 1
ENTRYPOINT ["/usr/local/bin/ipquail-go"]
