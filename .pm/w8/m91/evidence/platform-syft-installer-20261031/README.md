# Platform Syft installer recovery — 2026-10-31

Main run41178940668/job124478896135 passed every test suite and built all five images, then failed fetching the default Syft1.51.1 raw installer. The action exhausted three requests; its empty error does not identify the underlying transport fault. The complete raw log remains private with the recorded SHA; the exact failure excerpt is retained here.

The repair keeps Syft1.51.1 and downloads its official release archive directly with bounded fresh transfer retries and fixed size/SHA256 checks. Root independently matched both architecture pins to official release asset digests and reviewed the install/publication/cleanup paths. All12 hermetic transport/integrity/extraction tests and the repository actions validator pass. Actionlint has no new diagnostics. All five complete SBOM/signature/CVE paths remain unchanged. Actual new CI execution and API adoption remain pending.
