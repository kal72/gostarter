#!/bin/sh
# Application-pinned tools only; never run upstream scripts.
set -eu
source_dir=${1:-deployments/build-runtime}
tools_dir=${GOSTARTER_TOOLS_DIR:-/tmp/gostarter-tools}
mkdir -p "$tools_dir/bin"
case "$(uname -s)/$(uname -m)" in
  Linux/x86_64) platform=linux-x86_64 ;;
  Linux/aarch64|Linux/arm64) platform=linux-aarch_64 ;;
  *) echo 'Provisioning supports Linux amd64/arm64 only' >&2; exit 1 ;;
esac
stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT HUP INT TERM
curl --fail --silent --show-error --location --proto '=https' "https://github.com/protocolbuffers/protobuf/releases/download/v34.0/protoc-34.0-${platform}.zip" -o "$stage/protoc.zip"
unzip -q "$stage/protoc.zip" -d "$tools_dir"
test "$("$tools_dir/bin/protoc" --version)" = 'libprotoc 34.0'
export GOBIN="$tools_dir/bin"
go install github.com/google/wire/cmd/wire@v0.7.0
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v2.29.0
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@v2.29.0
# Copy manifests so dependency provisioning never changes the source checkout.
cp "$source_dir/go.mod" "$source_dir/go.sum" "$stage/"
(cd "$stage" && GOTOOLCHAIN=go1.27.1 go mod download all)
