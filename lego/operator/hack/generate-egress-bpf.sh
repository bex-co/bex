#!/usr/bin/env bash
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

set -euo pipefail

operator_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
repo_root="$(cd "$operator_root/../.." && pwd)"

docker run --rm \
  --platform linux/amd64 \
  -v "$repo_root:/src" \
  -w /src/lego/operator/internal/egressmeter \
  golang:1.25-bookworm \
  bash -euc '
    apt-get update >/dev/null
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends clang llvm libbpf-dev gcc-multilib >/dev/null
    go run github.com/cilium/ebpf/cmd/bpf2go \
      -go-package egressmeter \
      -cc clang \
      -cflags "-O2 -g -Wall -Werror" \
      bpf bpf/meter.bpf.c
  '
