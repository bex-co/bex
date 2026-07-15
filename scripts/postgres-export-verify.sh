#!/usr/bin/env bash
set -euo pipefail

# Logical-export smoke test: use the operator's pg_dump/tar and S3 upload
# commands, fetch the artifact through an expiring authenticated URL, restore it
# into a second vanilla Postgres, and assert seeded data. Requires Docker;
# creates only short-lived local containers/network/files.

network="bex-export-verify-$$"
source_container="bex-export-src-$$"
target_container="bex-export-dst-$$"
minio_container="bex-export-minio-$$"
work="$(mktemp -d)"
filename="2026-07-14T12_00Z.dir.tar.gz"
object_uri="s3://tenant-backups/postgres/logical-exports/verify-db/exp-c185th5c2rvvnhbfiltg/$filename"
endpoint="http://$minio_container:9000"
access_key="verify-access"
secret_key="verify-secret-key-for-one-shot-test"

cleanup() {
  docker rm -f "$source_container" "$target_container" "$minio_container" >/dev/null 2>&1 || true
  docker network rm "$network" >/dev/null 2>&1 || true
  rm -rf "$work"
}
trap cleanup EXIT

docker network create "$network" >/dev/null
docker run -d \
  --name "$source_container" \
  --network "$network" \
  -e POSTGRES_PASSWORD=verify \
  -e POSTGRES_DB=verify_db \
  postgres:16 >/dev/null
until docker exec "$source_container" pg_isready -U postgres -d verify_db >/dev/null 2>&1; do
  sleep 1
done
docker exec "$source_container" psql -U postgres -d verify_db -v ON_ERROR_STOP=1 \
  -c "CREATE TABLE export_probe (id integer PRIMARY KEY, value text NOT NULL); INSERT INTO export_probe VALUES (1, 'portable');" >/dev/null

docker run --rm \
  --network "$network" \
  -v "$work:/work" \
  -e PGHOST="$source_container" \
  -e PGPORT=5432 \
  -e PGUSER=postgres \
  -e PGPASSWORD=verify \
  -e PGDATABASE=verify_db \
  -e EXPORT_DIRECTORY=2026-07-14T12:00Z \
  -e EXPORT_FILENAME="$filename" \
  postgres:16 /bin/sh -ec \
  'mkdir -p "/work/${EXPORT_DIRECTORY}/${PGDATABASE}"
pg_dump --format=directory --jobs=2 --no-owner --no-privileges --file="/work/${EXPORT_DIRECTORY}/${PGDATABASE}"
tar -C /work -czf "/work/${EXPORT_FILENAME}" "${EXPORT_DIRECTORY}"'

docker run -d \
  --name "$minio_container" \
  --network "$network" \
  -p 127.0.0.1::9000 \
  -e MINIO_ROOT_USER="$access_key" \
  -e MINIO_ROOT_PASSWORD="$secret_key" \
  minio/minio:RELEASE.2025-04-22T22-12-26Z server /data >/dev/null
until docker run --rm --network "$network" curlimages/curl:8.12.1 \
  --fail --silent "$endpoint/minio/health/live" >/dev/null 2>&1; do
  sleep 1
done

aws() {
  docker run --rm \
    --network "$network" \
    -v "$work:/work" \
    -e AWS_ACCESS_KEY_ID="$access_key" \
    -e AWS_SECRET_ACCESS_KEY="$secret_key" \
    -e AWS_DEFAULT_REGION=us-east-1 \
    -e AWS_EC2_METADATA_DISABLED=true \
    amazon/aws-cli:2.22.35 --endpoint-url "$endpoint" "$@"
}
aws s3 mb s3://tenant-backups >/dev/null
aws s3 cp "/work/$filename" "$object_uri" >/dev/null

host_endpoint="http://$(docker port "$minio_container" 9000/tcp)"
(
  cd lego/backend
  BEX_TEST_EXPORT_S3_ENDPOINT="$host_endpoint" \
    BEX_TEST_EXPORT_S3_OBJECT="$object_uri" \
    BEX_TEST_EXPORT_ACCESS_KEY="$access_key" \
    BEX_TEST_EXPORT_SECRET_KEY="$secret_key" \
    go test ./internal/postgres -run TestS3ExportSignerDownloadsFromS3CompatibleStore -count=1 \
    >/dev/null
)

object_url="$endpoint/tenant-backups/postgres/logical-exports/verify-db/exp-c185th5c2rvvnhbfiltg/$filename"
anonymous_status="$(docker run --rm --network "$network" curlimages/curl:8.12.1 \
  --silent --output /dev/null --write-out '%{http_code}' "$object_url")"
test "$anonymous_status" = "403"

signed_url="$(aws s3 presign "$object_uri" --expires-in 900)"
mkdir -p "$work/download" "$work/restore"
docker run --rm \
  --network "$network" \
  -v "$work/download:/download" \
  curlimages/curl:8.12.1 --fail --silent --show-error \
  --output "/download/$filename" "$signed_url"
tar -xzf "$work/download/$filename" -C "$work/restore"

docker run -d \
  --name "$target_container" \
  --network "$network" \
  -e POSTGRES_PASSWORD=verify \
  -e POSTGRES_DB=verify_db \
  postgres:16 >/dev/null
until docker exec "$target_container" pg_isready -U postgres -d verify_db >/dev/null 2>&1; do
  sleep 1
done
docker run --rm \
  --network "$network" \
  -v "$work/restore:/work:ro" \
  -e PGPASSWORD=verify \
  postgres:16 pg_restore \
  --dbname="postgresql://postgres@$target_container:5432/verify_db" \
  --verbose \
  --clean \
  --if-exists \
  --no-owner \
  --no-privileges \
  --format=directory \
  /work/2026-07-14T12:00Z/verify_db >/dev/null

result="$(docker exec "$target_container" psql -U postgres -d verify_db -Atc "SELECT id || ':' || value FROM export_probe")"
test "$result" = "1:portable"
size="$(wc -c <"$work/download/$filename" | tr -d ' ')"
printf 'authenticated object-store download and portable restore verified: %s (%s bytes)\n' "$result" "$size"
