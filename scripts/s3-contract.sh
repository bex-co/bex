#!/usr/bin/env bash
# s3-contract.sh — run the stock S3 clients against bex's own S3 gateway
# (ADR112 D11, layer 3, w21/m499 t004).
#
# bex serves the S3 API at objects.<base domain> for any client that speaks it,
# and `bex s3` runs the AWS CLI against it. The clients are other people's
# software and change their defaults in releases (checksum algorithms, payload
# framing, addressing), so this installs the pinned releases below into a
# throwaway directory and drives them over TLS against the real gateway
# (lego/backend/internal/s3gw, TestS3ContractServer) on real Postgres and a
# fake Wasabi:
#
#   aws-cli v2   s3 mb|cp|mv|rm|sync|ls|presign|rb, and s3api for each bucket
#                configuration call, a multipart upload and a batch delete, with
#                the config `bex s3` gives it (path-style addressing)
#   rclone       mkdir, copy, copyto, sync, lsf, check, cat, moveto, delete, purge
#   boto3        create_bucket, upload_file and download_file of 20 MiB, copy,
#                listing, a presigned-URL upload, CORS and lifecycle
#
# They reach the gateway the way a customer's do, through the edge: Traefik, the
# release the cluster runs, with the plugin of the S3 route (bexS3MetadataCase,
# deploy/gitops/charts/traefik-plugins/s3metadatacase), forwards to the gateway.
# S3 names user metadata in lowercase and botocore, so boto3 and aws-cli, returns the
# name as it came: the clients read it back exactly as S3 would, which only the edge
# can arrange (w21/113).
#
# Each runs with its own defaults and no tuning of checksums, payload signing or
# part sizes. The gateway logs what every client sent and answered; the report
# at the end is the defaults each client actually used (addressing style,
# checksum framing, payload mode) and every request bex answered 501 (rclone
# asks for a bucket's versioning and goes on without it), and the run fails on
# any 5xx and on any command that fails. Terraform's AWS provider is not tested.
#
# Needs bash, curl, go, python3 (3.10 or later, or uv to fetch one), unzip, and
# Docker for the throwaway Postgres unless BEX_TEST_DB_URI names one. Nothing
# here touches a real bex deployment, Wasabi or an AWS account.
#
# After changing a pin, run it, record the defaults it prints in ADR112 §
# Not verified, and commit both.
set -euo pipefail

AWSCLI_VERSION=2.39.5
RCLONE_VERSION=1.75.2
# botocore is pinned to boto3's own release; the libraries they pull in float.
BOTO3_VERSION=1.43.125
# The edge's Traefik: the tag deploy/gitops/base/values/traefik.values.yaml pins, checked below.
TRAEFIK_VERSION=3.7.17

root=$(cd "$(dirname "$0")/.." && pwd)

die() {
  echo "s3-contract: $*" >&2
  exit 2
}

for tool in curl go unzip; do
  command -v "$tool" >/dev/null || die "$tool is not installed"
done

aws_base="https://awscli.amazonaws.com"
rclone_base="https://downloads.rclone.org/v$RCLONE_VERSION"
traefik_base="https://github.com/traefik/traefik/releases/download/v$TRAEFIK_VERSION"
case "$(uname -s)_$(uname -m)" in
  Linux_x86_64)
    aws_file="awscli-exe-linux-x86_64-$AWSCLI_VERSION.zip"
    aws_sha=fe0553306b3d001d162bc193d3b040027aab3466b9f910b50e702c122c0f3ff9
    rclone_file="rclone-v$RCLONE_VERSION-linux-amd64.zip"
    rclone_sha=349ac8fba6ff65d6247043f1750cdcb518ec5d500ef91463a10d37c0ccdf3702
    traefik_file="traefik_v${TRAEFIK_VERSION}_linux_amd64.tar.gz"
    traefik_sha=a564abd5fc8ee97d838270f188bb94cb3172cc113f4b51aa5e7dd6117eedb393
    ;;
  Linux_aarch64 | Linux_arm64)
    aws_file="awscli-exe-linux-aarch64-$AWSCLI_VERSION.zip"
    aws_sha=ce7b03ebceed1dc6a0a8f27134a41a9a3b5671f09c57ab2e7fc6d43710dbaef3
    rclone_file="rclone-v$RCLONE_VERSION-linux-arm64.zip"
    rclone_sha=7e1e8d69654941b7b7df84ee74c5f7fb09cce7d5496947fb7bf16b55ff427d10
    traefik_file="traefik_v${TRAEFIK_VERSION}_linux_arm64.tar.gz"
    traefik_sha=b06dad2f66ac3481c5695591b7cd3a3eed5e6dbec739889bb01e69cf8e45a0fc
    ;;
  Darwin_arm64)
    aws_file="AWSCLIV2-$AWSCLI_VERSION.pkg"
    aws_sha=f89c9288dbd9bd0579914ea3a72acc14bca1ae3d3112e92bcb29ff5a48b88131
    rclone_file="rclone-v$RCLONE_VERSION-osx-arm64.zip"
    rclone_sha=610ab7d48615135dfc60b763858b11a426a0d3d1160c1bb76fb0ae6011b02edd
    traefik_file="traefik_v${TRAEFIK_VERSION}_darwin_arm64.tar.gz"
    traefik_sha=fd489802a4c1684c5726bc263a44c585c6ff0591809b064831da91c7bb33ccad
    ;;
  Darwin_x86_64)
    aws_file="AWSCLIV2-$AWSCLI_VERSION.pkg"
    aws_sha=f89c9288dbd9bd0579914ea3a72acc14bca1ae3d3112e92bcb29ff5a48b88131
    rclone_file="rclone-v$RCLONE_VERSION-osx-amd64.zip"
    rclone_sha=574e1f31cf1c464e5fb0325cb66fcbdeaf1efa1802bd9b17f13c256fd74b6561
    traefik_file="traefik_v${TRAEFIK_VERSION}_darwin_amd64.tar.gz"
    traefik_sha=c562c73dc5132dc79a4668db2a5ed2f69f630e2493e6e8fe987233a6d025b3ec
    ;;
  *) die "no pinned client builds for $(uname -sm)" ;;
esac

work=$(mktemp -d)
pg_name=""
server_pid=""
edge_pid=""
cleanup() {
  [ -z "$edge_pid" ] || kill "$edge_pid" 2>/dev/null || true
  [ -z "$server_pid" ] || kill "$server_pid" 2>/dev/null || true
  [ -z "$pg_name" ] || docker rm -f -v "$pg_name" >/dev/null 2>&1 || true
  rm -rf "$work"
}
trap cleanup EXIT

sha256_of() {
  if command -v sha256sum >/dev/null; then sha256sum "$1" | cut -c1-64; else shasum -a 256 "$1" | cut -c1-64; fi
}

fetch() { # <url> <sha256> <file>
  bash "$root/scripts/ci-fetch-verified.sh" "$1" "$2" "$3" 4 >&2 || die "could not fetch $1"
}

# --- the clients ---------------------------------------------------------------
echo "s3-contract: installing aws-cli $AWSCLI_VERSION, rclone v$RCLONE_VERSION, boto3 $BOTO3_VERSION and Traefik v$TRAEFIK_VERSION"
grep -qE "^\s+tag: v$TRAEFIK_VERSION@sha256:" "$root/deploy/gitops/base/values/traefik.values.yaml" ||
  die "Traefik v$TRAEFIK_VERSION is not the release deploy/gitops/base/values/traefik.values.yaml pins: move the pins together"
mkdir -p "$work/dl" "$work/bin"
fetch "$aws_base/$aws_file" "$aws_sha" "$work/dl/$aws_file"
case "$aws_file" in
  *.zip)
    unzip -q "$work/dl/$aws_file" -d "$work/dl/aws-src"
    "$work/dl/aws-src/aws/install" -i "$work/aws-cli" -b "$work/bin" >/dev/null
    ;;
  *.pkg)
    pkgutil --expand-full "$work/dl/$aws_file" "$work/dl/aws-pkg"
    ln -s "$work/dl/aws-pkg/aws-cli.pkg/Payload/aws-cli/aws" "$work/bin/aws"
    ;;
esac
fetch "$rclone_base/$rclone_file" "$rclone_sha" "$work/dl/$rclone_file"
unzip -q -j "$work/dl/$rclone_file" '*/rclone' -d "$work/bin"
chmod +x "$work/bin/rclone"
fetch "$traefik_base/$traefik_file" "$traefik_sha" "$work/dl/$traefik_file"
tar -xzf "$work/dl/$traefik_file" -C "$work/bin" traefik

python=""
for candidate in python3.13 python3.12 python3.11 python3.10 python3; do
  if command -v "$candidate" >/dev/null && "$candidate" -c 'import sys; sys.exit(sys.version_info < (3, 10))' 2>/dev/null; then
    python=$candidate
    break
  fi
done
if [ -n "$python" ]; then
  "$python" -m venv "$work/venv"
elif command -v uv >/dev/null; then
  uv venv --quiet --python 3.12 "$work/venv"
else
  die "boto3 needs Python 3.10 or later: install it, or uv"
fi
if [ -x "$work/venv/bin/pip" ]; then
  "$work/venv/bin/pip" install --quiet "boto3==$BOTO3_VERSION" "botocore==$BOTO3_VERSION"
else
  uv pip install --quiet --python "$work/venv/bin/python" "boto3==$BOTO3_VERSION" "botocore==$BOTO3_VERSION"
fi
PATH="$work/bin:$PATH"
aws --version
rclone version | head -1
traefik version | sed -n 1p
"$work/venv/bin/python" -c 'import boto3; print("boto3", boto3.__version__)'

# --- the gateway ---------------------------------------------------------------
if [ -z "${BEX_TEST_DB_URI:-}" ]; then
  command -v docker >/dev/null || die "docker is not installed, and BEX_TEST_DB_URI names no Postgres"
  pg_image=$(grep -oE 'postgres:17@sha256:[0-9a-f]{64}' "$root/.github/workflows/backend-test.yml" | head -1)
  [ -n "$pg_image" ] || die "no postgres pin in backend-test.yml"
  pg_name="bex-s3-contract-$$"
  docker run -d --rm --name "$pg_name" -e POSTGRES_PASSWORD=pw -p 127.0.0.1::5432 "$pg_image" >/dev/null
  for _ in $(seq 1 60); do
    docker exec "$pg_name" pg_isready -U postgres -h 127.0.0.1 >/dev/null 2>&1 && break
    sleep 1
  done
  pg_port=$(docker port "$pg_name" 5432/tcp | head -1 | sed 's/.*://')
  export BEX_TEST_DB_URI="postgres://postgres:pw@127.0.0.1:$pg_port/postgres?sslmode=disable"
fi

state="$work/state"
mkdir -p "$state"
echo setup >"$state/phase"
edge_port=$(python3 -c 'import socket; s = socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])')
(cd "$root/lego/backend" && GOWORK=off go test -c -o "$work/s3gw.test" ./internal/s3gw)
(
  cd "$root/lego/backend/internal/s3gw"
  # exec: the background job is the test binary itself, so the cleanup's kill reaches it.
  BEX_S3_CONTRACT_DIR="$state" BEX_S3_CONTRACT_EDGE_PORT="$edge_port" BEX_TEST_REQUIRE_DEPS=1 exec "$work/s3gw.test" -test.run '^TestS3ContractServer$' -test.v -test.timeout 30m
) >"$work/server.log" 2>&1 &
server_pid=$!
for _ in $(seq 1 120); do
  [ -f "$state/server.env" ] && break
  kill -0 "$server_pid" 2>/dev/null || {
    cat "$work/server.log" >&2
    die "the gateway did not start"
  }
  sleep 1
done
[ -f "$state/server.env" ] || die "the gateway did not start in two minutes"
# shellcheck disable=SC1091
. "$state/server.env"

# --- the edge ------------------------------------------------------------------
# What fronts the gateway in production (lego/operator/config/s3gw/ingressroute.yaml):
# Traefik, loading the S3 route's plugin from the directory the cluster mounts, which
# writes user-metadata names in lowercase. It serves the certificate the clients
# already trust and forwards the request unchanged (Host included: SigV4 signs it) to
# the gateway.
edge="$work/edge"
plugin_src="$root/deploy/gitops/charts/traefik-plugins/s3metadatacase"
plugin_dir="$edge/plugins-local/src/github.com/bex-co/bex-s3-metadata-case"
mkdir -p "$plugin_dir"
cp "$plugin_src/.traefik.yml" "$plugin_src/go.mod" "$plugin_src/s3metadatacase.go" "$plugin_dir/"
cat >"$edge/static.yml" <<EOT
entryPoints:
  websecure:
    address: "127.0.0.1:$edge_port"
providers:
  file:
    filename: "$edge/dynamic.yml"
experimental:
  abortOnPluginFailure: true
  localPlugins:
    bexS3MetadataCase:
      moduleName: github.com/bex-co/bex-s3-metadata-case
log:
  level: ERROR
EOT
cat >"$edge/dynamic.yml" <<EOT
tls:
  certificates:
    - certFile: "$state/ca.pem"
      keyFile: "$state/edge.key"
http:
  middlewares:
    s3-metadata-case:
      plugin:
        bexS3MetadataCase: {}
  routers:
    s3:
      rule: "PathPrefix(\`/\`)"
      entryPoints: [websecure]
      middlewares: [s3-metadata-case]
      service: gateway
      tls: {}
  serversTransports:
    gateway:
      insecureSkipVerify: true
  services:
    gateway:
      loadBalancer:
        serversTransport: gateway
        servers:
          - url: "https://127.0.0.1:$S3_GATEWAY_PORT"
EOT
(cd "$edge" && exec traefik --configFile="$edge/static.yml") >"$work/edge.log" 2>&1 &
edge_pid=$!
for _ in $(seq 1 60); do
  [ "$(curl -s --cacert "$state/ca.pem" -o /dev/null -w '%{http_code}' "https://localhost:$edge_port/" 2>/dev/null)" = 403 ] && break
  kill -0 "$edge_pid" 2>/dev/null || {
    cat "$work/edge.log" >&2
    die "Traefik did not start"
  }
  sleep 1
done
[ "$(curl -s --cacert "$state/ca.pem" -o /dev/null -w '%{http_code}' "https://localhost:$edge_port/" 2>/dev/null)" = 403 ] || {
  cat "$work/edge.log" >&2
  die "the gateway did not answer through Traefik in a minute"
}
echo "s3-contract: the edge is at $S3_ENDPOINT, in front of the gateway on port $S3_GATEWAY_PORT (a client that picked virtual-hosted style would send <bucket>.${S3_ENDPOINT#https://})"

phase() { echo "$1" >"$state/phase"; }
step() { echo "s3-contract: $*"; }
expect_same() { # <what> <file> <file>
  [ "$(sha256_of "$2")" = "$(sha256_of "$3")" ] || die "$1: the bytes differ"
}

# --- the data ------------------------------------------------------------------
data="$work/data"
mkdir -p "$data/tree/sub/deep"
: >"$data/zero"
printf 'x' >"$data/one"
head -c $((20 * 1024 * 1024)) /dev/urandom >"$data/big"
head -c $((9 * 1024 * 1024)) /dev/urandom >"$data/tree/sub/nine.bin"
echo "first" >"$data/tree/a.txt"
echo "second" >"$data/tree/sub/b.txt"
head -c 1048576 /dev/urandom >"$data/tree/sub/deep/c.bin"

# --- how each client is run ------------------------------------------------------
# The settings `bex s3 env` prints for rclone; path-style addressing is its default.
mkdir -p "$work/home"
: >"$work/rclone.conf"
rclone_() {
  env HOME="$work/home" RCLONE_CONFIG="$work/rclone.conf" RCLONE_S3_PROVIDER=Other RCLONE_S3_ENDPOINT="$S3_ENDPOINT" \
    RCLONE_S3_ACCESS_KEY_ID="$S3_ACCESS_KEY_ID" RCLONE_S3_SECRET_ACCESS_KEY="$S3_SECRET_ACCESS_KEY" RCLONE_S3_REGION="$S3_REGION" \
    RCLONE_CA_CERT="$state/ca.pem" rclone "$@"
}
boto3_() {
  env HOME="$work/home" S3_ENDPOINT="$S3_ENDPOINT" S3_ACCESS_KEY_ID="$S3_ACCESS_KEY_ID" S3_SECRET_ACCESS_KEY="$S3_SECRET_ACCESS_KEY" \
    S3_REGION="$S3_REGION" AWS_CA_BUNDLE="$state/ca.pem" AWS_EC2_METADATA_DISABLED=true "$work/venv/bin/python" "$root/scripts/s3-contract-boto3.py" "$@"
}
# --- aws-cli -------------------------------------------------------------------
# What `bex s3` gives the AWS CLI: the endpoint last on the command line, a key
# in the environment, and a config of bex's own with path-style addressing.
: >"$work/aws-credentials"
printf '[default]\ns3 =\n    addressing_style = path\n' >"$work/aws-config"
: >"$work/aws-config-empty"
aws_env() {
  env HOME="$work/home" AWS_ACCESS_KEY_ID="$S3_ACCESS_KEY_ID" AWS_SECRET_ACCESS_KEY="$S3_SECRET_ACCESS_KEY" \
    AWS_REGION="$S3_REGION" AWS_DEFAULT_REGION="$S3_REGION" AWS_CA_BUNDLE="$state/ca.pem" AWS_PAGER="" \
    AWS_EC2_METADATA_DISABLED=true AWS_SHARED_CREDENTIALS_FILE="$work/aws-credentials" "$@"
}
aws_() { aws_env AWS_CONFIG_FILE="$work/aws-config" aws "$@" --endpoint-url "$S3_ENDPOINT"; }
# Progress lines only: they change nothing on the wire.
aws_s3() { aws_ s3 "$@" --no-progress; }

step "aws-cli: the defaults, asked what addressing it uses for this endpoint"
phase probe
aws_env AWS_CONFIG_FILE="$work/aws-config-empty" aws s3 ls s3://contract-probe --endpoint-url "$S3_ENDPOINT" >/dev/null 2>&1 || true

step "rclone and boto3: the defaults, asked the same"
rclone_ lsf ":s3:contract-probe" >/dev/null 2>&1 || true
boto3_ probe

step "aws-cli: buckets, objects, multipart, copy, move, sync, presign"
phase aws
bucket=contract-aws
aws_ s3 mb "s3://$bucket" --region frankfurt
aws_ s3api head-bucket --bucket "$bucket"
[ "$(aws_ s3api get-bucket-location --bucket "$bucket" --query LocationConstraint --output text)" = frankfurt ] ||
  die "aws: get-bucket-location did not answer frankfurt"
aws_ s3 ls | grep "$bucket" >/dev/null || die "aws: ls does not list the bucket"
for name in zero one big; do aws_s3 cp "$data/$name" "s3://$bucket/up/$name"; done
aws_s3 cp "$data/tree/sub/nine.bin" "s3://$bucket/up/nine"
for name in zero one big; do
  aws_s3 cp "s3://$bucket/up/$name" "$work/down-$name"
  expect_same "aws cp $name" "$data/$name" "$work/down-$name"
done
aws_ s3 ls "s3://$bucket/up/" --summarize | grep 'Total Objects: 4' >/dev/null || die "aws: ls --summarize does not count four objects"
aws_s3 cp "s3://$bucket/up/big" "s3://$bucket/copy/big"
aws_s3 mv "s3://$bucket/copy/big" "s3://$bucket/moved/big"
aws_s3 cp "s3://$bucket/moved/big" "$work/down-moved"
expect_same "aws cp, mv" "$data/big" "$work/down-moved"
aws_ s3 rm "s3://$bucket/moved/big"
aws_s3 sync "$data/tree" "s3://$bucket/tree"
[ -z "$(aws_s3 sync "$data/tree" "s3://$bucket/tree")" ] || die "aws: a second sync had something to do"
aws_s3 sync "s3://$bucket/tree" "$work/tree-down"
diff -r "$data/tree" "$work/tree-down" || die "aws: the synced tree differs"
aws_s3 sync "s3://$bucket/tree" "s3://$bucket/tree-copy"
rm "$data/tree/a.txt"
aws_s3 sync "$data/tree" "s3://$bucket/tree" --delete | grep 'delete: ' >/dev/null || die "aws: sync --delete deleted nothing"
echo "first" >"$data/tree/a.txt"
url=$(aws_ s3 presign "s3://$bucket/up/one" --expires-in 300)
[ "$(curl -sS --cacert "$state/ca.pem" "$url")" = x ] || die "aws: the presigned URL did not return the object"

step "aws-cli: s3api — configuration, listing, batch delete, multipart"
phase aws-api
printf '{"CORSRules":[{"AllowedOrigins":["https://example.com"],"AllowedMethods":["GET"],"AllowedHeaders":["*"],"MaxAgeSeconds":600}]}' >"$work/cors.json"
aws_ s3api put-bucket-cors --bucket "$bucket" --cors-configuration "file://$work/cors.json"
aws_ s3api get-bucket-cors --bucket "$bucket" --query 'CORSRules[0].AllowedOrigins[0]' --output text | grep example.com >/dev/null || die "aws: CORS did not round trip"
aws_ s3api delete-bucket-cors --bucket "$bucket"
printf '{"Rules":[{"ID":"logs","Status":"Enabled","Filter":{"Prefix":"logs/"},"Expiration":{"Days":7},"AbortIncompleteMultipartUpload":{"DaysAfterInitiation":3}}]}' >"$work/lifecycle.json"
aws_ s3api put-bucket-lifecycle-configuration --bucket "$bucket" --lifecycle-configuration "file://$work/lifecycle.json"
aws_ s3api get-bucket-lifecycle-configuration --bucket "$bucket" --query 'Rules[0].ID' --output text | grep logs >/dev/null || die "aws: lifecycle did not round trip"
aws_ s3api delete-bucket-lifecycle --bucket "$bucket"
[ "$(aws_ s3api list-objects-v2 --bucket "$bucket" --prefix tree-copy/ --page-size 1 --max-items 100 --query 'Contents[].Key' --output text | tr '\t' '\n' | grep -c .)" = 4 ] ||
  die "aws: a listing of tree-copy/ in pages of one did not return its four objects"
aws_ s3api get-object-tagging --bucket "$bucket" --key up/one >/dev/null
upload=$(aws_ s3api create-multipart-upload --bucket "$bucket" --key mp/explicit --query UploadId --output text)
head -c 5242880 /dev/urandom >"$work/part"
aws_ s3api upload-part --bucket "$bucket" --key mp/explicit --upload-id "$upload" --part-number 1 --body "$work/part" >/dev/null
aws_ s3api list-parts --bucket "$bucket" --key mp/explicit --upload-id "$upload" --query 'length(Parts)' --output text | grep 1 >/dev/null || die "aws: list-parts"
aws_ s3api list-multipart-uploads --bucket "$bucket" --query 'length(Uploads)' --output text | grep 1 >/dev/null || die "aws: list-multipart-uploads"
aws_ s3api abort-multipart-upload --bucket "$bucket" --key mp/explicit --upload-id "$upload"
aws_ s3api delete-objects --bucket "$bucket" --delete '{"Objects":[{"Key":"up/zero"},{"Key":"up/one"}]}' >/dev/null

# User metadata comes back as S3 names it: lowercase, whatever case it was sent in. The AWS
# CLI prints botocore's parse, which keeps the case it received, so this fails when the
# edge's plugin does not run, and a curl of the same object shows the head's own bytes
# (net/http's client would canonicalise them) over HTTP/1.1 (w21/113).
aws_ s3api put-object --bucket "$bucket" --key meta/one --body "$data/one" --metadata mtime=aws,MixedCase=Aws >/dev/null
for pair in mtime=aws mixedcase=Aws; do
  [ "$(aws_ s3api head-object --bucket "$bucket" --key meta/one --query "Metadata.${pair%%=*}" --output text)" = "${pair#*=}" ] ||
    die "aws: head-object does not read Metadata.${pair%%=*} as ${pair#*=}: user metadata names are not lowercase (w21/113)"
done
aws_ s3api copy-object --bucket "$bucket" --key meta/copy --copy-source "$bucket/meta/one" --metadata k=v --metadata-directive REPLACE >/dev/null
[ "$(aws_ s3api head-object --bucket "$bucket" --key meta/copy --query 'Metadata.k' --output text)" = v ] ||
  die "aws: a copy with REPLACE does not read back Metadata.k"
wire=$(curl -sS --http1.1 --cacert "$state/ca.pem" -D - -o /dev/null "$(aws_ s3 presign "s3://$bucket/meta/one" --expires-in 300)" | tr -d '\r')
grep -qx 'x-amz-meta-mtime: aws' <<<"$wire" && grep -qx 'x-amz-meta-mixedcase: Aws' <<<"$wire" ||
  die "the head of an HTTP/1.1 response does not name its user metadata in lowercase: $(grep -i '^x-amz-meta' <<<"$wire" | tr '\n' ' ')"

step "aws-cli: what bex refuses, and says so"
phase refused
if aws_ s3api get-bucket-versioning --bucket "$bucket" >/dev/null 2>&1; then die "aws: get-bucket-versioning succeeded; bex answers 501"; fi
aws_ s3 rb "s3://$bucket" --force

# --- rclone --------------------------------------------------------------------
# The settings `bex s3 env` prints for rclone; path-style addressing is its default.
step "rclone: copy, sync, list, check, move, delete, purge"
phase rclone
remote=":s3:contract-rclone"
rclone_ mkdir "$remote"
for name in zero one big; do rclone_ copyto "$data/$name" "$remote/up/$name"; done
rclone_ copy "$data/tree" "$remote/tree"
rclone_ check "$data/tree" "$remote/tree"
rclone_ sync -v "$data/tree" "$remote/tree" 2>"$work/rclone-resync.log"
if grep Copied "$work/rclone-resync.log" >/dev/null; then die "rclone: a second sync copied again (user metadata or modification time did not round trip)"; fi
[ "$(rclone_ lsf -R "$remote/tree" | grep -vc '/$')" = 4 ] || die "rclone: lsf -R does not list the four files of the tree"
[ "$(rclone_ cat "$remote/up/one")" = x ] || die "rclone: cat"
rclone_ copyto "$remote/up/big" "$work/rclone-big"
expect_same "rclone big" "$data/big" "$work/rclone-big"
rclone_ moveto "$remote/up/big" "$remote/moved/big"
rclone_ size --json "$remote" | grep '"count":' >/dev/null || die "rclone: size"
rclone_ delete "$remote/moved"
rclone_ purge "$remote"

# --- boto3 ---------------------------------------------------------------------
step "boto3: upload_file of 20 MiB, copy, listing, presigned URL, configuration"
phase boto3
boto3_

# --- the verdict -----------------------------------------------------------------
phase finished
touch "$state/stop"
rc=0
wait "$server_pid" || rc=$?
server_pid=""
echo
echo "s3-contract: the defaults each client used (clients' requests by phase; aws-cli $AWSCLI_VERSION, rclone v$RCLONE_VERSION, boto3 $BOTO3_VERSION)"
cat "$state/report.txt" 2>/dev/null || true
if [ "$rc" -ne 0 ]; then
  grep -E '^\s+contract_server_test|--- FAIL|was answered' "$work/server.log" >&2 || cat "$work/server.log" >&2
  die "the gateway answered a client with a 5xx, or failed"
fi
echo "s3-contract: ok"
