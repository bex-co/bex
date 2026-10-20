# CLI sweep 833 — deploy-mode discipline on an image-backed service (live, 2026-10-20)

Disposable free image-backed web service created, deployed, deleted (204, then GET 404). No new bug filed.

- `{"deployMode":"deploy_only"}` → 201 (re-pull of the configured image, per the ADR018 w2/m30 contract for image-backed services).
- `{"imageUrl":"docker.io/library/nginx:latest"}` → named 400: the URL must keep the configured host/repository (`gcr.io/google-samples/hello-app`), only tag/digest may change per deploy (the w8/042 origin-safety rule), with the settings path named as the way to switch repositories.
- `{"commitId":"abc833"}` on an image-backed service → named 400 `commitId is not supported for image-backed services` with the imageUrl alternative spelled out.
