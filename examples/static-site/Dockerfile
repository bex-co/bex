# bex builds static sites through its Dockerfile build plane (docs/ADR029-static-sites.md,
# "Dockerfile-only today"): the build must produce an image whose publishPath
# directory (here ".", the WORKDIR) holds the site. bex's publish Job then copies
# that directory out of this image to object storage and serves it via the shared
# static-server — this image is never run to serve traffic, only to stage files.
#
# There is no build step for a plain HTML site; we just copy the files in. A
# "publish a directory with no Dockerfile" flow (Render parity) is a tracked
# follow-up (.pm/w9/010) — until then, static sites need this tiny Dockerfile.
FROM busybox:stable
WORKDIR /site
COPY index.html .
