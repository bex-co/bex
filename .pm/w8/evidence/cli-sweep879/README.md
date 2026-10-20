# CLI sweep 879 — `previews` field refuses by name on create and PATCH (live, 2026-10-20)

One refused create, one disposable service created for the PATCH probe and deleted (204, then GET 404). No new bug filed.

- `serviceDetails.previews {generation: "automatic"}` on create → named 400 `services previews is not supported by this platform`; the same block on PATCH → the identical refusal. The rejected anti-goal (`.pm/DO_NOT_DO.md`, sweep 817's missing endpoint) is also enforced inside otherwise-valid bodies — a client cannot smuggle previews in as an ignored field.
