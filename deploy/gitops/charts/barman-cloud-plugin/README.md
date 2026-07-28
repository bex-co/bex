# Barman Cloud Plugin

This directory vendors the exact upstream `v0.13.0` installation manifest used by bex. It installs the namespaced CNPG-I controller beside CloudNativePG in `cnpg-system`, the `ObjectStore` CRD, RBAC, service, and cert-manager certificates.

- Source: `https://github.com/cloudnative-pg/plugin-barman-cloud/releases/download/v0.13.0/manifest.yaml`
- SHA-256: `d2e71e7b06822448f1a421f05781846cfdb9cc621e7ef32eef5e20c5133213b0`
- Requirements: CloudNativePG 1.26+ (production is 1.30.0) and cert-manager.

The Kustomize patch places only the plugin controller on the platform node pool. CNPG injects the data-plane sidecar into each Postgres instance pod, so it follows that database's existing placement.
