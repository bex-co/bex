/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package s3gw

// knownAnswers are SigV4 signatures computed by botocore (the AWS SDK for Python)
// for fixed requests, so the verifier is checked against an implementation that
// is not this one. The first three are the worked examples of the S3 API
// reference (a header-signed GET with a Range, a header-signed PUT with a Date
// header and a storage class, and a presigned GET), with its documented example
// credentials and dates: their signatures are the documented ones.
type knownAnswer struct {
	name, kind, method, url, region string
	headers                         map[string]string
	signedHeaders, signature        string
}

const (
	exampleAccessKey = "AKIAIOSFODNN7EXAMPLE"
	exampleSecretKey = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
)

var knownAnswers = []knownAnswer{
	{name: "aws-doc-get-range", kind: "header", method: "GET", url: "https://examplebucket.s3.amazonaws.com/test.txt", region: "us-east-1",
		headers:       map[string]string{"Range": "bytes=0-9", "Host": "examplebucket.s3.amazonaws.com", "X-Amz-Content-SHA256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", "X-Amz-Date": "20130524T000000Z"},
		signedHeaders: "host;range;x-amz-content-sha256;x-amz-date", signature: "f0e8bdb87c964420e857bd35b5d6ed310bd44f0170aba48dd91039c6036bdb41"},
	{name: "aws-doc-put", kind: "header", method: "PUT", url: "https://examplebucket.s3.amazonaws.com/test%24file.text", region: "us-east-1",
		headers:       map[string]string{"Host": "examplebucket.s3.amazonaws.com", "x-amz-storage-class": "REDUCED_REDUNDANCY", "Date": "Fri, 24 May 2013 00:00:00 GMT", "X-Amz-Content-SHA256": "44ce7dd67c959e0d3524ffac1771dfbba87d2b6b4b4e99e42034a8b803f8b072", "X-Amz-Date": "20130524T000000Z"},
		signedHeaders: "date;host;x-amz-content-sha256;x-amz-date;x-amz-storage-class", signature: "98ad721746da40c64f1a55b78f14c238d841ea1380cd77a1b5971af0ece108bd"},
	{name: "aws-doc-presigned-get", kind: "presigned", method: "GET", url: "https://examplebucket.s3.amazonaws.com/test.txt?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=AKIAIOSFODNN7EXAMPLE%2F20130524%2Fus-east-1%2Fs3%2Faws4_request&X-Amz-Date=20130524T000000Z&X-Amz-Expires=86400&X-Amz-SignedHeaders=host", region: "us-east-1",
		headers:       map[string]string{},
		signedHeaders: "", signature: "aeeed9bbccd4d02ee5c0109b86d86835f995330da4c265957d157751f604d404"},
	{name: "path-style-special-key", kind: "header", method: "GET", url: "https://objects.onbex.co/photos/a%20b%2Bc/%C3%A9.txt", region: "oregon",
		headers:       map[string]string{"Host": "objects.onbex.co", "X-Amz-Content-SHA256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", "X-Amz-Date": "20130524T000000Z"},
		signedHeaders: "host;x-amz-content-sha256;x-amz-date", signature: "6f37689014e6fd9800592c9fb02c1ca6dab492efe1f569d8fe2794ce7af3b3b9"},
	{name: "list-v2", kind: "header", method: "GET", url: "https://objects.onbex.co/photos?list-type=2&prefix=a%2Fb%20c&delimiter=%2F&max-keys=2", region: "oregon",
		headers:       map[string]string{"Host": "objects.onbex.co", "X-Amz-Content-SHA256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", "X-Amz-Date": "20130524T000000Z"},
		signedHeaders: "host;x-amz-content-sha256;x-amz-date", signature: "84df4abe75af9ec0e34c999a96904fa4ab458f0a903191dad9be2943ca558494"},
	{name: "uploads-subresource", kind: "header", method: "POST", url: "https://objects.onbex.co/photos/big.bin?uploads", region: "frankfurt",
		headers:       map[string]string{"Host": "objects.onbex.co", "Content-Type": "application/octet-stream", "x-amz-meta-mtime": "1700000000.5", "X-Amz-Content-SHA256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", "X-Amz-Date": "20130524T000000Z"},
		signedHeaders: "content-type;host;x-amz-content-sha256;x-amz-date;x-amz-meta-mtime", signature: "166106f8f40025b7456fd385f0df772fd06b9e63a9c3ae45818bce3a6605be2f"},
	{name: "unsigned-payload-put", kind: "header", method: "PUT", url: "https://objects.onbex.co/photos/up.bin", region: "oregon",
		headers:       map[string]string{"Host": "objects.onbex.co", "Content-Length": "5", "X-Amz-Content-SHA256": "UNSIGNED-PAYLOAD", "X-Amz-Date": "20130524T000000Z"},
		signedHeaders: "content-length;host;x-amz-content-sha256;x-amz-date", signature: "a2df09756d5d55d401db84d73bb7c916ade2b44830be5ab503a802a63e9ff131"},
	{name: "presigned-put-with-query", kind: "presigned", method: "PUT", url: "https://objects.onbex.co/photos/up.bin?partNumber=2&uploadId=abc%2Bdef&X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=AKIAIOSFODNN7EXAMPLE%2F20130524%2Foregon%2Fs3%2Faws4_request&X-Amz-Date=20130524T000000Z&X-Amz-Expires=3600&X-Amz-SignedHeaders=host", region: "oregon",
		headers:       map[string]string{},
		signedHeaders: "", signature: "4446863f3010b36e3190cf603272914cd30a812c73cdc438180c1384416ede74"},
}
