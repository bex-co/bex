# kpack

Vendored upstream kpack controller plus bex's Paketo `ClusterStore`, Jammy `ClusterStack`, and `ClusterBuilder`.

- Version: `v0.17.2`
- Upstream asset: `https://github.com/buildpacks-community/kpack/releases/download/v0.17.2/release-0.17.2.yaml`
- SHA-256: `6e3ed15d7a4fa4dec8fc95c7ece023c9951724b840a9c5cb536cf662c0bbdf8d`

`platform.yaml` imports the complete image named by `BEX_CNB_BUILDER` into a `ClusterStore`. Because current kpack builds a `ClusterBuilder` from a store and stack rather than referencing a prebuilt builder directly, its order mirrors the top-level order embedded in the default Paketo builder.

The `zot.local:5000` tag is a DNS alias of `zot.bex-registry.svc:5000`. go-containerregistry recognizes `*.local` as an HTTP development registry, so kpack can use the same internal Zot endpoint without a global insecure-registry switch. Production TLS registries can set `BEX_KPACK_REGISTRY` equal to `BEX_REGISTRY` and patch the two config values here.
