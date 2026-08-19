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

package sniproxy

import (
	"github.com/bex-co/bex/lego/types/proxyproto"
)

// ParseTrustedProxyCIDRs, RemoteIP, and ReadProxySource are direct re-exports
// of the shared PROXY protocol v1/v2 parser in lego/types/proxyproto (w1/m77)
// — one implementation for the pg/kv SNI proxies and the backend SSH gateway,
// so a parser fix can never land in one copy and miss the other. They are
// assignments, not wrapper funcs, so the delegation guard test can prove by
// function identity that no local fork has crept back in. Non-PROXY bytes
// stay buffered for the PostgreSQL preamble or TLS ClientHello reader.
var (
	ParseTrustedProxyCIDRs = proxyproto.ParseTrustedProxyCIDRs
	RemoteIP               = proxyproto.RemoteIP
	ReadProxySource        = proxyproto.ReadProxySource
)
