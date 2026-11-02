# Actual Docker Hub quota failures — 2026-11-01

Current-source run41363104200 fails both utility architectures on upstream Docker Hub metadata/pull429 responses. The AMD64 first Node compute case passed before later metadata refusals; all actual failures and cleanup/upload results remain preserved. The separate local Node20 attempt failed before authored build/runtime checks, so its compatibility and executed version remain unproved.

A verified immutable-metadata reuse correction reduces duplicate reads while retaining full first-read validation. Existing source-bound successful cohorts and local cases remain historical records; the current complete baseline, remaining families and actual pre-deploy Job are still required. No failed result is relabeled or accepted.
