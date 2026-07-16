// Copyright 2026.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

#include <linux/bpf.h>
#include <linux/ip.h>
#include <linux/ipv6.h>
#include <bpf/bpf_endian.h>
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_helpers.h>

#define LABEL_BYTES 64
#define NF_ACCEPT 1

enum diagnostic_reason {
    DIAG_SEEN,
    DIAG_PACKET_READ_ERROR,
    DIAG_UNKNOWN_VERSION,
    DIAG_V4_UNATTRIBUTED,
    DIAG_V4_EXCLUDED,
    DIAG_V4_COUNTED,
    DIAG_V6_UNATTRIBUTED,
    DIAG_V6_EXCLUDED,
    DIAG_V6_COUNTED,
    DIAG_MAX,
};

/* BTF context supplied to BPF_PROG_TYPE_NETFILTER programs. */
struct sk_buff {
    unsigned char *head;
    __u16 network_header;
} __attribute__((preserve_access_index));
struct nf_hook_state;
struct bpf_nf_ctx {
    struct nf_hook_state *state;
    struct sk_buff *skb;
} __attribute__((preserve_access_index));

struct resource_key {
    char app_id[LABEL_BYTES];
    char namespace[LABEL_BYTES];
};

struct lpm_v4_key {
    __u32 prefixlen;
    __u32 addr;
};

struct lpm_v6_key {
    __u32 prefixlen;
    __u8 addr[16];
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 65536);
    __type(key, __u32);
    __type(value, struct resource_key);
    __uint(pinning, LIBBPF_PIN_BY_NAME);
} pod_v4_resources SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 65536);
    __type(key, __u8[16]);
    __type(value, struct resource_key);
    __uint(pinning, LIBBPF_PIN_BY_NAME);
} pod_v6_resources SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 16384);
    __type(key, struct resource_key);
    __type(value, __u64);
    __uint(pinning, LIBBPF_PIN_BY_NAME);
} resource_bytes SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_LPM_TRIE);
    __uint(max_entries, 4096);
    __uint(map_flags, BPF_F_NO_PREALLOC);
    __type(key, struct lpm_v4_key);
    __type(value, __u8);
    __uint(pinning, LIBBPF_PIN_BY_NAME);
} excluded_v4 SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_LPM_TRIE);
    __uint(max_entries, 4096);
    __uint(map_flags, BPF_F_NO_PREALLOC);
    __type(key, struct lpm_v6_key);
    __type(value, __u8);
    __uint(pinning, LIBBPF_PIN_BY_NAME);
} excluded_v6 SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, DIAG_MAX);
    __type(key, __u32);
    __type(value, __u64);
    __uint(pinning, LIBBPF_PIN_BY_NAME);
} diagnostics SEC(".maps");

static __always_inline void diagnose(__u32 reason)
{
    __u64 *value = bpf_map_lookup_elem(&diagnostics, &reason);
    if (value)
        __sync_fetch_and_add(value, 1);
}

static __always_inline void add_bytes(struct resource_key *resource, __u32 len)
{
    __u64 *bytes = bpf_map_lookup_elem(&resource_bytes, resource);
    if (bytes)
        __sync_fetch_and_add(bytes, len);
}

SEC("netfilter")
int bex_count_public_egress(struct bpf_nf_ctx *ctx)
{
    unsigned char *head;
    __u16 network_header;
    __u8 first;

    diagnose(DIAG_SEEN);
    if (!ctx->skb) {
        diagnose(DIAG_PACKET_READ_ERROR);
        return NF_ACCEPT;
    }
    head = BPF_CORE_READ(ctx->skb, head);
    network_header = BPF_CORE_READ(ctx->skb, network_header);
    if (!head || bpf_probe_read_kernel(&first, sizeof(first), head + network_header)) {
        diagnose(DIAG_PACKET_READ_ERROR);
        return NF_ACCEPT;
    }

    if ((first >> 4) == 4) {
        struct iphdr buffer = {};
        const struct iphdr *ip = &buffer;
        struct lpm_v4_key dst = {.prefixlen = 32};
        struct resource_key *resource;
        __u32 src;
        __u32 packet_len;

        if (bpf_probe_read_kernel(&buffer, sizeof(buffer), head + network_header)) {
            diagnose(DIAG_PACKET_READ_ERROR);
            return NF_ACCEPT;
        }
        packet_len = bpf_ntohs(ip->tot_len);
        src = ip->saddr;
        resource = bpf_map_lookup_elem(&pod_v4_resources, &src);
        if (!resource) {
            diagnose(DIAG_V4_UNATTRIBUTED);
            return NF_ACCEPT;
        }
        dst.addr = ip->daddr;
        if (bpf_map_lookup_elem(&excluded_v4, &dst)) {
            diagnose(DIAG_V4_EXCLUDED);
            return NF_ACCEPT;
        }
        add_bytes(resource, packet_len);
        diagnose(DIAG_V4_COUNTED);
    } else if ((first >> 4) == 6) {
        struct ipv6hdr buffer = {};
        const struct ipv6hdr *ip6 = &buffer;
        struct lpm_v6_key dst = {.prefixlen = 128};
        struct resource_key *resource;
        __u8 src[16];
        __u32 packet_len;

        if (bpf_probe_read_kernel(&buffer, sizeof(buffer), head + network_header)) {
            diagnose(DIAG_PACKET_READ_ERROR);
            return NF_ACCEPT;
        }
        packet_len = sizeof(buffer) + bpf_ntohs(ip6->payload_len);
        __builtin_memcpy(src, &ip6->saddr, sizeof(src));
        resource = bpf_map_lookup_elem(&pod_v6_resources, &src);
        if (!resource) {
            diagnose(DIAG_V6_UNATTRIBUTED);
            return NF_ACCEPT;
        }
        __builtin_memcpy(dst.addr, &ip6->daddr, sizeof(dst.addr));
        if (bpf_map_lookup_elem(&excluded_v6, &dst)) {
            diagnose(DIAG_V6_EXCLUDED);
            return NF_ACCEPT;
        }
        add_bytes(resource, packet_len);
        diagnose(DIAG_V6_COUNTED);
    } else {
        diagnose(DIAG_UNKNOWN_VERSION);
    }

    return NF_ACCEPT;
}

char LICENSE[] SEC("license") = "GPL";
