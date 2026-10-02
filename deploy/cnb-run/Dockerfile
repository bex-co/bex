# bex CNB run image: Paketo's run-jammy-base plus libatomic1 (w1/m169).
#
# Official Node.js linux binaries link libatomic.so.1 from Node 25 on, and no
# Paketo Jammy run image ships it (neither run-jammy-base nor run-jammy-full,
# checked 2026-10-02). Render resolves an open `engines.node` range such as
# `>=20` to the latest Node release, so on bex's buildpack path the same app
# built green and then crash-looped with
# "error while loading shared libraries: libatomic.so.1". This image is that
# run image plus the one missing library, nothing else.
#
# The base digest is a reviewed pin (docs/ADR060 D7): it is inventory id
# `cnb-stack-run` in lego/operator/internal/build/toolchain-freshness.json.
# Accepting a new run-jammy-base digest means editing it here, then pinning the
# rebuilt image in deploy/gitops/charts/kpack/platform.yaml (`cnb-run-image`).
#
# Labels (io.buildpacks.stack.id = io.buildpacks.stacks.jammy, the stack
# metadata kpack checks against the build image) are inherited from the base.
FROM paketobuildpacks/run-jammy-base@sha256:33c1e0d7158b09c70d3200fdbe9957075cd39dd4e703a3e6ed384475b9793e2b

USER root
# The final ldconfig check is the regression gate: the build fails if the
# loader cannot resolve libatomic.so.1, rather than shipping a run image that
# only fails when a tenant's Node 25+ process starts.
RUN apt-get update \
  && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends libatomic1 \
  && apt-get clean \
  && rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/*.deb /var/log/apt /var/log/dpkg.log \
  && ldconfig -p | grep -q 'libatomic\.so\.1 '

# Restore the Paketo stack's CNB user (stack.toml: uid 1002, gid 1000).
USER 1002:1000
