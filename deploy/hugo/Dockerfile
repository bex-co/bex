# syntax=docker/dockerfile:1.7
# bex Hugo image: the reviewed Hugo every native static build carries (w8/m106),
# as a digest-pinned image builders already pull from, instead of a GitHub
# release download on every build (w8/m148). On 2026-11-08 GitHub's release
# CDN answered 504 for the Hugo tarballs for over 90 minutes, and every native
# static build failed `digest mismatch` while it did.
#
# The image is the official linux tarball's `hugo` binary and nothing else, at
# /hugo. Each architecture's tarball is checksum-pinned to the sha256 that
# Hugo's release lists (inventory ids `native-hugo-amd64`/`native-hugo-arm64`
# in lego/operator/internal/build/toolchain-freshness.json); it is unpacked on
# the build platform, so publishing needs no emulation. A Hugo bump edits the
# version and both checksums here, then pins the republished digest in
# lego/operator/internal/build/nativehugo.go (`native-hugo-image`).
ARG HUGO_VERSION=0.111.3

FROM scratch AS tarball-amd64
ARG HUGO_VERSION
ADD --checksum=sha256:b382aacb522a470455ab771d0e8296e42488d3ea4e61fe49c11c32ec7fb6ee8b https://github.com/gohugoio/hugo/releases/download/v${HUGO_VERSION}/hugo_extended_${HUGO_VERSION}_linux-amd64.tar.gz /hugo.tar.gz

FROM scratch AS tarball-arm64
ARG HUGO_VERSION
ADD --checksum=sha256:695176416dbe09d8c84abeb093f5f27132e6811221569811c855b577122bd670 https://github.com/gohugoio/hugo/releases/download/v${HUGO_VERSION}/hugo_extended_${HUGO_VERSION}_linux-arm64.tar.gz /hugo.tar.gz

FROM tarball-${TARGETARCH} AS tarball

# The pinned busybox native builds already use (inventory id `native-busybox`)
# unpacks just the binary, without the archive's owner.
FROM --platform=$BUILDPLATFORM busybox:1.37.0@sha256:bdf57e528e45e4433820e045b29b4597825a1c9e38353532d90a01445013f82e AS unpack
RUN --mount=type=bind,from=tarball,target=/in mkdir /out && tar -xzof /in/hugo.tar.gz -C /out hugo

FROM scratch
COPY --from=unpack /out/hugo /hugo
