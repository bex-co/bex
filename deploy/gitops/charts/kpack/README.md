# kpack

Vendored upstream kpack controller plus bex's Paketo `ClusterStore`, Jammy `ClusterStack`, and `ClusterBuilder`.

- Version: `v0.18.0`
- Upstream asset: `https://github.com/buildpacks-community/kpack/releases/download/v0.18.0/release-0.18.0.yaml`
- SHA-256: `cde8b7df8d31d6a5758ec4880eec45009f17811baf3df5a29b76a144fe200e69`

v0.18.0 adds arm64/multi-architecture component images and resolves registry images for the controller's current architecture. It has no CRD migration from v0.17.2. Production Kubernetes is v1.34.9, so the old `KUBERNETES_MIN_VERSION=1.31.0` startup override is intentionally absent.

`platform.yaml` imports the complete image named by `BEX_CNB_BUILDER` into a `ClusterStore`. Because current kpack builds a `ClusterBuilder` from a store and stack rather than referencing a prebuilt builder directly, its order mirrors the top-level order embedded in the default Paketo builder.

The `zot.local:5000` tag is a DNS alias of `zot.bex-registry.svc:5000`. go-containerregistry recognizes `*.local` as an HTTP development registry, so kpack can use the same internal Zot endpoint without a global insecure-registry switch. Production TLS registries can set `BEX_KPACK_REGISTRY` equal to `BEX_REGISTRY` and patch the two config values here.

`scripts/registry-secrets.sh` writes `bex-registry-push-kpack` both to the tenant build namespace and to `bex-system`, where this chart's `bex-kpack-builder` ServiceAccount lives. Secret references are namespace-local; the control-plane copy is what authorizes ClusterBuilder publication, while per-App build credentials remain isolated in `bex-build`.
