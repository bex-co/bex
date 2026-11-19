#!/usr/bin/env python3
# Copyright 2026.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
"""boto3's half of scripts/s3-contract.sh (ADR112 D11): the client library as a
customer's program uses it, with its own defaults. The one setting is path-style
addressing, which bex requires and its docs tell boto3 users to set. Reads the
endpoint and key from the environment the script exports; talks TLS to a
self-signed endpoint through AWS_CA_BUNDLE."""

import hashlib
import os
import ssl
import sys
import tempfile
import urllib.request

import boto3
from botocore.config import Config
from botocore.exceptions import ClientError

MIB = 1024 * 1024


def sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for block in iter(lambda: f.read(MIB), b""):
            h.update(block)
    return h.hexdigest()


def probe():
    """boto3's defaults, asked which addressing style it uses for this endpoint. The answer
    is in the gateway's report; a refusal here is expected and ignored."""
    s3 = boto3.client(
        "s3",
        endpoint_url=os.environ["S3_ENDPOINT"],
        region_name=os.environ["S3_REGION"],
        aws_access_key_id=os.environ["S3_ACCESS_KEY_ID"],
        aws_secret_access_key=os.environ["S3_SECRET_ACCESS_KEY"],
    )
    try:
        s3.head_bucket(Bucket="contract-probe")
    except Exception:  # noqa: BLE001 - only the request matters
        pass


def main():
    endpoint = os.environ["S3_ENDPOINT"]
    s3 = boto3.client(
        "s3",
        endpoint_url=endpoint,
        region_name=os.environ["S3_REGION"],
        aws_access_key_id=os.environ["S3_ACCESS_KEY_ID"],
        aws_secret_access_key=os.environ["S3_SECRET_ACCESS_KEY"],
        config=Config(s3={"addressing_style": "path"}),
    )
    bucket = "contract-boto3"
    work = tempfile.mkdtemp()

    s3.create_bucket(Bucket=bucket, CreateBucketConfiguration={"LocationConstraint": "frankfurt"})
    assert s3.get_bucket_location(Bucket=bucket)["LocationConstraint"] == "frankfurt"
    assert bucket in [b["Name"] for b in s3.list_buckets()["Buckets"]]

    # upload_file of 20 MiB: s3transfer's multipart path (8 MiB parts, ten at a time).
    big = os.path.join(work, "big")
    with open(big, "wb") as f:
        f.write(os.urandom(20 * MIB))
    s3.upload_file(big, bucket, "dir/big", ExtraArgs={"ContentType": "application/x-contract", "Metadata": {"mtime": "42", "MixedCase": "x"}})
    head = s3.head_object(Bucket=bucket, Key="dir/big")
    assert head["ContentLength"] == 20 * MIB and head["ContentType"] == "application/x-contract", head
    # S3 lowercases user metadata names, and botocore returns the name as it arrived, so a boto3
    # client reads {"mtime": "42", "mixedcase": "x"}. Traefik canonicalises every header name it
    # relays, so the edge's plugin writes them in lowercase again (w21/113); the script runs
    # the gateway behind that edge, and this is exact.
    assert head["Metadata"] == {"mtime": "42", "mixedcase": "x"}, head["Metadata"]
    s3.copy_object(Bucket=bucket, Key="dir/replaced", CopySource={"Bucket": bucket, "Key": "dir/big"},
                   Metadata={"K": "v"}, MetadataDirective="REPLACE")
    assert s3.head_object(Bucket=bucket, Key="dir/replaced")["Metadata"] == {"k": "v"}
    s3.delete_object(Bucket=bucket, Key="dir/replaced")
    down = os.path.join(work, "down")
    s3.download_file(bucket, "dir/big", down)
    assert sha256(down) == sha256(big), "the 20 MiB object came back different"

    # put_object and get_object of small bodies, 0 bytes included, and a ranged read.
    for name, body in (("zero", b""), ("one", b"x"), ("text", b"hello bex")):
        s3.put_object(Bucket=bucket, Key="small/" + name, Body=body)
        assert s3.get_object(Bucket=bucket, Key="small/" + name)["Body"].read() == body, name
    part = s3.get_object(Bucket=bucket, Key="small/text", Range="bytes=6-8")["Body"].read()
    assert part == b"bex", part

    # Listing: pages and a delimiter.
    pages = list(s3.get_paginator("list_objects_v2").paginate(Bucket=bucket, PaginationConfig={"PageSize": 2}))
    keys = [o["Key"] for p in pages for o in p.get("Contents", [])]
    assert keys == ["dir/big", "small/one", "small/text", "small/zero"], keys
    folded = s3.list_objects_v2(Bucket=bucket, Delimiter="/")
    assert [p["Prefix"] for p in folded["CommonPrefixes"]] == ["dir/", "small/"], folded

    # copy (server-side, multipart for the 20 MiB object) and delete_objects.
    s3.copy({"Bucket": bucket, "Key": "dir/big"}, bucket, "copy/big")
    copied = os.path.join(work, "copied")
    s3.download_file(bucket, "copy/big", copied)
    assert sha256(copied) == sha256(big), "the server-side copy differs"

    # A presigned URL upload and download, as a browser or another service would use one.
    context = ssl.create_default_context(cafile=os.environ["AWS_CA_BUNDLE"])
    put_url = s3.generate_presigned_url("put_object", Params={"Bucket": bucket, "Key": "presigned/up"}, ExpiresIn=300)
    request = urllib.request.Request(put_url, data=b"through a presigned url", method="PUT")
    with urllib.request.urlopen(request, context=context) as response:
        assert response.status == 200, response.status
    get_url = s3.generate_presigned_url("get_object", Params={"Bucket": bucket, "Key": "presigned/up"}, ExpiresIn=300)
    with urllib.request.urlopen(get_url, context=context) as response:
        assert response.read() == b"through a presigned url"

    # Bucket configuration round trips.
    cors = {"CORSRules": [{"AllowedOrigins": ["https://example.com"], "AllowedMethods": ["GET", "PUT"], "AllowedHeaders": ["*"], "MaxAgeSeconds": 600}]}
    s3.put_bucket_cors(Bucket=bucket, CORSConfiguration=cors)
    assert s3.get_bucket_cors(Bucket=bucket)["CORSRules"][0]["AllowedOrigins"] == ["https://example.com"]
    s3.delete_bucket_cors(Bucket=bucket)
    lifecycle = {"Rules": [{"ID": "expire-logs", "Status": "Enabled", "Filter": {"Prefix": "logs/"}, "Expiration": {"Days": 7}}]}
    s3.put_bucket_lifecycle_configuration(Bucket=bucket, LifecycleConfiguration=lifecycle)
    assert s3.get_bucket_lifecycle_configuration(Bucket=bucket)["Rules"][0]["ID"] == "expire-logs"
    s3.delete_bucket_lifecycle(Bucket=bucket)
    try:
        s3.get_bucket_lifecycle_configuration(Bucket=bucket)
        raise AssertionError("a deleted lifecycle configuration still reads back")
    except ClientError as error:
        assert error.response["Error"]["Code"] == "NoSuchLifecycleConfiguration", error.response

    # Delete everything, then the bucket.
    listed = s3.list_objects_v2(Bucket=bucket)["Contents"]
    result = s3.delete_objects(Bucket=bucket, Delete={"Objects": [{"Key": o["Key"]} for o in listed]})
    assert not result.get("Errors"), result
    for page in s3.get_paginator("list_objects_v2").paginate(Bucket=bucket):
        assert not page.get("Contents"), page
    s3.delete_bucket(Bucket=bucket)
    print("boto3 contract: ok")


if __name__ == "__main__":
    try:
        probe() if sys.argv[1:] == ["probe"] else main()
    except Exception as error:  # a failed assertion is a failed contract
        print("boto3 contract: FAILED:", repr(error), file=sys.stderr)
        sys.exit(1)
