FROM node:22-bookworm-slim AS assets
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts --no-audit --no-fund
COPY tailwind.config.cjs ./
COPY scripts/build-assets.cjs scripts/build-assets.cjs
COPY internal/web/ui internal/web/ui
COPY assets/js assets/js
COPY assets/css assets/css
RUN npm run build

# The application includes HTTP and validation; generation requires the build runtime.
FROM golang:1.27.1-bookworm AS build
RUN apt-get update && apt-get install -y --no-install-recommends curl unzip ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY scripts/prepare-build-runtime.sh /build/scripts/prepare-build-runtime.sh
COPY deployments/build-runtime/go.mod deployments/build-runtime/go.sum /opt/reference/
RUN GOSTARTER_TOOLS_DIR=/opt/gostarter-tools sh scripts/prepare-build-runtime.sh /opt/reference
COPY . .
COPY --from=assets /src/assets/css/tailwind.css assets/css/tailwind.css
COPY --from=assets /src/assets/vendor assets/vendor
RUN go tool templ generate
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/gostarter ./cmd/server

FROM golang:1.27.1-bookworm AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl && rm -rf /var/lib/apt/lists/* && groupadd -g 10001 appgroup && useradd -u 10001 -g appgroup -m appuser
COPY --from=build /opt/gostarter-tools /opt/gostarter-tools
COPY --from=build /go/pkg/mod /go/pkg/mod
COPY --from=build /out/ /app/
ENV PATH="/opt/gostarter-tools/bin:${PATH}" GOMODCACHE=/go/pkg/mod GOCACHE=/tmp/go-build GOPROXY=off APP_PORT=8089 SNAPSHOT_DIR=/app/snapshots VALIDATION_WORKSPACE_DIR=/tmp/gostarter-validation
RUN mkdir -p /app/snapshots /tmp/gostarter-validation /tmp/go-build && chown -R appuser:appgroup /app /go /tmp/gostarter-validation /tmp/go-build
WORKDIR /app
USER appuser:appgroup
EXPOSE 8089
# Bootstrap may be alive before a snapshot exists; /readyz is the release gate.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD curl -fsS "http://127.0.0.1:${APP_PORT}/healthz" || exit 1
ENTRYPOINT ["/app/gostarter"]
