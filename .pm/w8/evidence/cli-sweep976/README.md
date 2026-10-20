# CLI sweep 976 — Key Value lifecycle; w8/139 still live at the pin (live, 2026-10-20)

One disposable free Key Value instance created and deleted (204; GET 404). No new bug filed.

- Create with the underscore spelling `allkeys_lru` → 201 (the free slot was available), GET → 200.
- The [w8/139](../../139.md) probe re-run on this fixture: PATCH with the hyphenated `allkeys-lru` → 400, exactly the filed behavior. The fix (`9e9c28e82`) is on `origin/main` but not in the production pin `3ede47d3c5a9`, so the finding stays open pending the pin.
