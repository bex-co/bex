# w8 · m95 — Supply libvips in native builds and runtime

**Worker:** worker8 **Goal:** ordinary native image-processing apps can load the supplied libvips dependency and transform images without tenant installation workarounds. **Status:** blocked — needs the user's choice of native tool-supply mechanism ([decision](../m91/design.md)); no task started

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply reviewed native libvips libraries, headers and tools](t001.md) | 90m | — |
| t002 | [Audit shared bases, phases, companions and exclusions](t002.md) | 35m | t001 |
| t003 | [Verify native image-library Render parity and shared inputs](t003.md) | 20m | t002 |
| t004 | [Simplify the native libvips supplier](t004.md) | 15m | t003 |
| t005 | [Test generated-image binding, header and CLI transformations](t005.md) | 55m | t003 |
| t006 | [Accept original native defaults and close the milestone](t006.md) | 15m | t004, t005 |

**Total:**6 tasks/230m. [Finding](finding.md) · [complete evidence](evidence/cli-sweep414/README.md).

## Definition of done

A reviewed immutable multiarch libvips supplier, complete dependency/data closure and promised headers/CLI tools are usable through intended native build/pre-deploy/runtime/static phases. Meaningful real generated-image import, header compile/link and image transformations pass, preserving all audited bases, companions, env/PATH/root/cache behavior and excluded build paths. After publication both unchanged native clients create original first Free Python services without an install line, become Live and pass exact source/program/input/build/runtime/pixel/PNG/native-log/shared-readback proofs and exact cleanup/baseline/current-quota checks. Documented limits are explicit. Apt install controls and source changes alone do not close the milestone.

## Source + Goal linkage

- **Source:** Authorized functional CLI sweep414,2026-10-17: both unchanged native POST201/exit0 first builds fail missing libvips.so.42 after successful PyVips installation; both one-line library controls Live with six real image transformations and independently decoded pixels.
- **Goal:** [ADR008](../../../../docs/ADR008-vision.md), [Render parity ADR018](../../../../docs/ADR018-render-parity.md) and [native assembly ADR060](../../../../docs/ADR060-build-worker-reliability-and-performance.md): ordinary hosting image-processing dependencies should work as documented.
- **Outcome:** Plain image bindings and supplied native libvips tools work without modifying tenant dependencies or CLI inputs.
- **Why now:** Both independent native clients, exact selected-base controls and real image outputs isolate the absent library. There is no supplier or newer fix; adjacent utility work does not provide it.
- **Render parity:** Included because the image contents affect tenant builds/runtime across REST/GraphQL/MCP/Blueprint/dashboard; no new API field is needed.
