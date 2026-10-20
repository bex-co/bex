# CLI sweep 1162 — static sites refuse prebuilt images (live, 2026-10-20)

Refusal-side only; nothing was created. No new bug filed.

- A `static_site` create carrying an `image` → 400 `a static_site builds from a Git repo; a prebuilt image is not supported`. The type's build source is enforced at create with an explanatory refusal, consistent with ADR029's repo-built model (the header/route surface itself was live-verified in the w8/137 closure, sweep 1033).
