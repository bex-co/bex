# Existing-volume probe-routing regression — 2026-10-31

The real full `LinuxMountService.Publish` path reached a known-existing, healthy owned LUKS/ext4 mapping. The test simulated only the inner `blkid` no-identifiers response (exit2, empty output). Production code requested `mkfs.ext4`; the command guard refused it. The actual regression test exited1 with `ORIGINAL CRITERION REGRESSION`. The wrapper exited0 only because the intended failure and cleanup evidence were complete.

The dedicated executable guard recorded its exact mount flags, PATH resolution, real permitted status preflight and the actual Publish status call. No formatter, resize or mount requested by Publish executed. Independent UUID, geometry and sentinel checks preserved both healthy filesystems. The exact owned test container was stopped/removed with cleanup0 and loop/mapping absence proved. This demonstrates response routing, not actual damaged-filesystem detection, kernel EIO or filesystem data loss.

[Root review](root-review.json) independently verifies all33 retained payload hashes and the frozen receipt inside [the compact proof](proof.tar.gz). The incomplete earlier guard-log attempt remains retained privately with its separate failed criterion qualification; its result was not rewritten. No original hosted, billing, restore or provider-cleanup acceptance is closed by this local test.
