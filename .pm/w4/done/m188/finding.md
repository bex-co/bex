# w4/m188 — Two hosting phrases ignore the selected language

Why: Chinese operators still see English relative times and schedule explanations on otherwise translated hosting screens.

**Source:** production `$qa-find-bugs` loop, 2026-10-09 cycle 28, muse.env, user-directed w4. **Severity:** two minor presentation defects. **Research HEAD:** `1d717c68f1606e181ac0e710bdb503aee22ace97`, clean and equal to origin/main. Filing only; no product code changed.

## 1. Relative ages and countdowns stay English

- **Repro:** create the sole owned Free image cron `qa-20261009-loop-a28-cron` (`busybox:1.37`), annual schedule `0 0 1 1 *`, command `printf 'qa-a28 locale cron begin\n'; sleep 3; printf 'qa-a28 locale cron end\n'`. It became Running and its deploy Live. In Chinese, Trigger Run → confirm; run `crr-pfjidslm5b31gfsgkmjs` succeeded, started 11:33:39Z, finished 11:33:46Z, duration 7s. Both expected output lines reached logs.
- **Actual:** fresh Chinese `/services/srv-db4d1ib93q6c73at017g/events?lang=zh` at 11:34:00 has `上次运行：now` and `下次运行：in 83d`; the corresponding datetime values are 11:33:46Z and 2027-01-01T00:00:00Z. At 11:35:57 fresh desktop, header and history ages are 2m/3m and countdown remains in 83d. At 11:36:16 fresh 390×844, the populated header still shows Chinese labels with in 83d. Its lower history/activity regions were loading; that screenshot is evidence about the header only.
- **Control:** fresh English at 11:35:16 uses `Last run: 1m`, `Next run: in 83d`, Created 2m; this is appropriate English output. English and Chinese API reads have the same created, last-success and next-run instants. The age progressed on later reads, so this is a language-input gap, not proof that w4/m152's clock fix regressed.
- **Target:** format with the active component/request translator. English keeps the existing compact strings and thresholds. Chinese past ages use 刚刚, N分钟, N小时, N天, N个月, N年; future countdowns use 现在 below 60 seconds and N分钟后 / N小时后 / N天后 otherwise. The exercised annual countdown becomes `83天后`, not `in 83d`. Exact instants, fallback nodes, titles, time/span geometry and clock behavior remain intact.
- **Root:** `dashboard/src/features/services/lib/format.ts:25–68` takes only instant and clock and returns literal now, m/h/d/mo/y, and in-prefixed output. `common/components/relative-time.tsx:26–78` calls it at :47 without any language/translator; it correctly guards missing/invalid values, retains `dateTime` and suppresses hydration bucket warnings. Header :227/:236/:546 consumes it; `api/server.graphql:42,122–123` → `lib/status.ts:66–68` preserves the exact API fields. There is no failing API localization step to fix.
- **Expressible fix:** inject the hook/request's `t` into the pure formatters and call translated common.\* messages with numeric amounts; obtain it unconditionally in RelativeTime before the fallback return. Supply the same translator to SessionRow's direct formatter use at :26, so its revoke name and visible age agree. Do not read an ambient i18next singleton from a pure helper; SSR already owns a request-specific instance. The hook is `common/hooks/use-translations.ts`, common locale catalogs are aggregated by `i18n/index.ts:45–65` and `i18n/resources-zh.ts:41–47`. No REST/GraphQL/MCP/operator change.
- **Blast radius:** a complete non-test JSX search finds **27 sites in 18 files** (table below), 26 RelativeAge and one RelativeUntil. There is also **one direct production formatter call**, SessionRow :26. The fix is global to that family, not cron-only. Shared-wrapper coverage plus the direct label and representative existing caller tests must protect current English behavior.
- **Unverified:** all remaining caller states, real screen-reader output, missing/invalid input, future-age clamping, months/year boundary arithmetic, hidden-tab resume, stable-page two-minute clock guarantees and SSR language/bucket races were not live-recertified. They are test/verification work, not additional live defects.

## 2. Cron schedule explanations stay English

- **Repro:** fresh Chinese `/services/new?type=cron_job&lang=zh` at desktop and 390×844. Default `*/5 * * * *` gives `Every 5 minutes · 使用 UTC`; `0 0 * * MON` gives `Every Monday at 00:00 · 使用 UTC`. Fresh Settings of the owned cron → 编辑计划 → type either expression gives the same mixed-language phrase. Reload Settings and repeat at 11:37:36: still Every 5 minutes. These drafts were canceled; the annual persisted schedule was never changed.
- **Control:** fresh English Settings gives `Every 5 minutes · runs in UTC`. Chinese `*/40 * * * *` remains valid but has no uniform-interval claim: Create uses its translated generic crontab hint, Settings shows no preview. `99 99 * * *` is invalid, translated validation appears and Settings Save is disabled. Create had other required fields blank, so its disabled Deploy button alone does not establish schedule-specific submit gating.
- **Target:** both consumers use translated complete phrase templates and localized weekday names from the current translator. `*/5 * * * *` is `每 5 分钟 · 使用 UTC`; `0 0 * * MON` is `每周一 00:00 · 使用 UTC`. English retains the current correct descriptions. Supported hourly/daily/monthly branches follow the same locale; unsupported, non-uniform, invalid and never-firing schedules keep their existing fallback/validation semantics. Do not expand the described schedule set to obtain a translation.
- **Root:** `features/services/lib/cron.ts:188–196,233–286` hardcodes English day names and all description sentences. `routes/services.new.tsx:94–96,341–344` inserts the result into `services.createFieldSchedulePreview`; `components/cron-deploy-section.tsx:88–92` inserts it into `services.deploySchedulePreview`. Both zh catalog entries (:3188 and :1860) translate only `{description} · 使用 UTC`; the interpolated value is already English.
- **Consumer check:** `EditableFieldRow` :149–155 describes the shown draft while editing and persisted value otherwise; validation suppresses the description. Its describe prop accepts a ReactNode/null, so localized text is representable. Source functions `canonicalField`, `stepShape`, `cronScheduleProblem` and their accepted vector set stay intact. `describeCron` currently calls its trimmed local variable `t`; rename that local if injecting a translator with that name.
- **Framework:** installed/lockfile react-i18next **17.0.8**, i18next **26.3.4**. `react-i18next/dist/commonjs/useTranslation.js:53–94` subscribes to the language event (defaults.js:9) and obtains fixed `t` for the current language. `i18next/dist/cjs/i18next.js:2033–2066` forwards the fixed language; its Interpolator :1168–1230 fetches the supplied description and substitutes it verbatim. Interpolation does not translate an English sentence. `i18n/init.ts` configures single-brace interpolation and registers the lazy catalog before switching. The mechanism is the helper's English literals, not a broken Chinese resource loader or a cronstrue dependency (no such dependency is used here).
- **Blast radius:** **two production describeCron call sites**, Create and CronDeploySection; the latter has **one production mount**, `routes/services.$serviceId.settings.tsx:169`, gated by actual cron type. Web, static, private and worker omit the cron form controls; Postgres/Key Value are separate families.
- **Unverified:** saved every-five-minute/weekly execution, all other phrase branches in both live forms, all sources/types and alias entrypoints, and settled in-place language switching. Source-derived branches belong in the implementation's test work.

## Shared caller inventory and aliases

Search: `rg -n 'Relative(Age|Until)|formatRelative(Age|Until)' dashboard/src --glob '*.ts' --glob '*.tsx' --glob '!**/__tests__/**'`, followed by a JSX-only count excluding src/test. Definitions, comments, imports and test utilities are not production render sites.

| file under dashboard/src | JSX lines |
| --- | --- |
| routes/blueprints.tsx | 157 |
| routes/blueprints.$blueprintId.tsx | 437, 445, 539, 542 |
| routes/services.$serviceId.events.tsx | 478 |
| features/services/components/cron-runs-section.tsx | 223 |
| features/services/components/service-detail-header.tsx | 227, 236 (Until), 546 |
| features/registry-credentials/components/registry-credential-row.tsx | 44 |
| features/webhooks/components/webhook-row.tsx | 81 (span) |
| features/webhooks/components/webhook-deliveries-card.tsx | 460 (span) |
| features/events/components/event-timeline.tsx | 93 |
| features/agent-sessions/components/session-list.tsx | 281, 365 |
| features/keyvalue/components/key-value-metadata-card.tsx | 93 |
| features/sessions/components/session-row.tsx | 47; also direct formatter :26 |
| features/api-keys/components/api-key-row.tsx | 40, 46 |
| features/connected-agents/components/connected-agent-row.tsx | 63 |
| features/projects/components/resource-table.tsx | 221, 226 |
| features/ssh-keys/components/ssh-keys-panel.tsx | 196 |
| features/databases/components/recovery-panel.tsx | 257, 380 |
| features/databases/components/database-metadata-card.tsx | 116 |

ResourceTable has three production placements: Overview :287, EnvironmentsPanel :323 and EnvironmentCard :379. ServiceDetailHeader has one production mount in ServiceDetailLayout :135; the layout has two route mounts, services.$serviceId :63 and static.$serviceId :57. The same header therefore covers web/static/cron/worker/private, with cron-only last/next fields. Database/Key Value metadata have their own shared-wrapper sites above. Read-only Chinese probes of existing web, Postgres, Key Value and Blueprint list showed 2mo/1mo for correct exact instants; no resource configuration was touched.

`common/lib/render-alias.ts:12–20,35–43,58–92` maps `/web`, `/worker`, `/pserv`, `/static`, `/cron` to the canonical service family and `/d`, `/r` to database/Key Value; `/cron/new` selects cron type in Create. Static canonicalizes to `/static/<id>`. These aliases share the implementation; their live replay remains verification work. No claim that the path's segment alone identifies a resource type.

## Adjacent states, dedupe and limits

- Keep existing pending-route skeletons, cached data through background polls, draft/cancel behavior and invalid-input fallback. A resolved Chinese render uses its already-loaded request/component translator; never show English as a temporary locale-loading substitute. Clock-derived text retains the current hydration guard. No permission, not-found, unauthenticated, timeout or resource existence policy changes; there are no added requests.
- This is **not a regression** of w6/done/m102 (hydration warning), w4/done/m152 (presentation clock), w6/done/m107 (viewer-local absolute dates) or w9/done/061 (truthful cron steps). Their README/DoD and relevant implementation were reviewed. Current m152 advances ages; m102's wrapper guards remain. Their whole wider DoD was not repeated and none is reopened/closed by these probes.
- w9/done/m91 introduced the English human-readable preview and validation; its DoD did not establish Chinese phrasing. Original git history `d8d72c1d0` already hardcodes these sentences. Relative literals trace to `9a2e700d7` / `bcce3d5ff`; no subsequently landed locale-aware formatter/preview fix found.
- Searched open/blocked/done board items for relative-time localization, formatRelativeAge/Until, describeCron and English schedule previews; scanned all 72 non-done README headings and open scopes, last 40 dashboard/lego commits and targeted -S history. No existing owner or undeployed equivalent. Full current DO_NOT_DO.md reviewed; no anti-goal applies. The earlier `w4/227` label wording is a separate pending copy fix and is not duplicated here.
- Absolute English calendar dates, run-duration units and command-save lifecycle toast are different mechanisms and are outside this filing. Paid services, real authz/error failures, custom domains, timed scheduler ticks and raw Kubernetes artifact cleanup were not exercised.
- An initial handcrafted log query used the wrong argument/envelope; the corrected request below succeeded. Create observer initially expected a single operation, but Apollo submitted a batch and created the fixture once. A dialog role lookup and a post-logout heading lookup timed out in the harness; neither is filed as a product failure. Immediate language-switch output was captured before async settling and is not a pass.

## Durable requests and responses

All requests used the isolated browser session; never copy cookies. The fixture has been removed. Replay on a new owned Free cron. No endpoint URL exists for this cron, so no public-serving assertion is made.

Fresh Chinese post-run DOM at 11:34:00.005Z (the handcrafted API query in that observation had a schema error and is not offered as an API proof; successful exact queries follow):

```json
{
  "observedAt": "2026-10-09T11:34:00.005Z",
  "url": "https://dashboard.bex.co/services/srv-db4d1ib93q6c73at017g/events?lang=zh",
  "lang": "zh",
  "times": [
    {
      "text": "now",
      "dateTime": "2026-10-09T11:33:46Z"
    },
    {
      "text": "in 83d",
      "dateTime": "2027-01-01T00:00:00Z"
    },
    {
      "text": "1m",
      "dateTime": "2026-10-09T11:32:25Z"
    }
  ]
}
```

UI Trigger Run, 11:33:39.759Z, POST https://api.bex.co/graphql, exact request and complete response:

```json
{
  "observedAt": "2026-10-09T11:33:39.759Z",
  "request": [
    {
      "operationName": "RunCronJob",
      "variables": {
        "id": "srv-db4d1ib93q6c73at017g"
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "mutation RunCronJob($id: String!) {\n  runCronJob(id: $id) {\n    id\n    status\n    startedAt\n    finishedAt\n    __typename\n  }\n}"
    }
  ],
  "status": 200,
  "response": [
    {
      "data": {
        "runCronJob": {
          "__typename": "CronRun",
          "finishedAt": "",
          "id": "crr-pfjidslm5b31gfsgkmjs",
          "startedAt": "",
          "status": "pending"
        }
      }
    }
  ]
}
```

Fresh Chinese desktop, 11:35:57.100Z, POST https://api.bex.co/graphql. Exact query, complete response for every selected field and exact DOM time values:

```json
{
  "observedAt": "2026-10-09T11:35:57.100Z",
  "url": "https://dashboard.bex.co/services/srv-db4d1ib93q6c73at017g/events?lang=zh",
  "lang": "zh",
  "request": {
    "query": "query QaA28Cron($id:String!){server(id:$id){id name type phase plan createdAt schedule command lastSuccessfulRunAt nextRunAt} cronJobRuns(serviceId:$id,limit:10){id status startedAt finishedAt} logs(resource:$id,type:\"app\",limit:50){hasMore nextStartTime nextEndTime logs{message timestamp type instance level}}}",
    "variables": {
      "id": "srv-db4d1ib93q6c73at017g"
    }
  },
  "status": 200,
  "response": {
    "data": {
      "cronJobRuns": [
        {
          "finishedAt": "2026-10-09T11:33:46Z",
          "id": "crr-pfjidslm5b31gfsgkmjs",
          "startedAt": "2026-10-09T11:33:39Z",
          "status": "successful"
        }
      ],
      "logs": {
        "hasMore": false,
        "logs": [
          {
            "instance": "srv-db4d1ib93q6c73at017g-u35np444fba5jv5989mm",
            "level": "unknown",
            "message": "qa-a28 locale cron begin",
            "timestamp": "2026-10-09T11:33:41.541564944Z",
            "type": "app"
          },
          {
            "instance": "srv-db4d1ib93q6c73at017g-u35np444fba5jv5989mm",
            "level": "unknown",
            "message": "qa-a28 locale cron end",
            "timestamp": "2026-10-09T11:33:44.543384744Z",
            "type": "app"
          }
        ],
        "nextEndTime": "2026-10-09T11:33:41.541564943Z",
        "nextStartTime": "2026-10-09T10:35:56.75456763Z"
      },
      "server": {
        "command": "printf 'qa-a28 locale cron begin\\n'; sleep 3; printf 'qa-a28 locale cron end\\n'",
        "createdAt": "2026-10-09T11:32:25Z",
        "id": "srv-db4d1ib93q6c73at017g",
        "lastSuccessfulRunAt": "2026-10-09T11:33:46Z",
        "name": "qa-20261009-loop-a28-cron",
        "nextRunAt": "2027-01-01T00:00:00Z",
        "phase": "Running",
        "plan": "free",
        "schedule": "0 0 1 1 *",
        "type": "cron_job"
      }
    }
  },
  "times": [
    {
      "text": "2m",
      "dateTime": "2026-10-09T11:33:46Z"
    },
    {
      "text": "in 83d",
      "dateTime": "2027-01-01T00:00:00Z"
    },
    {
      "text": "3m",
      "dateTime": "2026-10-09T11:32:25Z"
    },
    {
      "text": "2m",
      "dateTime": "2026-10-09T11:33:46Z"
    },
    {
      "text": "2m",
      "dateTime": "2026-10-09T11:33:39Z"
    },
    {
      "text": "3m",
      "dateTime": "2026-10-09T11:32:28Z"
    },
    {
      "text": "3m",
      "dateTime": "2026-10-09T11:32:25Z"
    },
    {
      "text": "2m",
      "dateTime": "2026-10-09T11:33:39Z"
    }
  ]
}
```

English control, fresh page, same exact query/selected response and corresponding time values:

```json
{
  "observedAt": "2026-10-09T11:35:16.065Z",
  "url": "https://dashboard.bex.co/services/srv-db4d1ib93q6c73at017g/events?lang=en",
  "lang": "en",
  "request": {
    "query": "query QaA28Cron($id:String!){server(id:$id){id name type phase plan createdAt schedule command lastSuccessfulRunAt nextRunAt} cronJobRuns(serviceId:$id,limit:10){id status startedAt finishedAt} logs(resource:$id,type:\"app\",limit:50){hasMore nextStartTime nextEndTime logs{message timestamp type instance level}}}",
    "variables": {
      "id": "srv-db4d1ib93q6c73at017g"
    }
  },
  "status": 200,
  "response": {
    "data": {
      "cronJobRuns": [
        {
          "finishedAt": "2026-10-09T11:33:46Z",
          "id": "crr-pfjidslm5b31gfsgkmjs",
          "startedAt": "2026-10-09T11:33:39Z",
          "status": "successful"
        }
      ],
      "logs": {
        "hasMore": false,
        "logs": [
          {
            "instance": "srv-db4d1ib93q6c73at017g-u35np444fba5jv5989mm",
            "level": "unknown",
            "message": "qa-a28 locale cron begin",
            "timestamp": "2026-10-09T11:33:41.541564944Z",
            "type": "app"
          },
          {
            "instance": "srv-db4d1ib93q6c73at017g-u35np444fba5jv5989mm",
            "level": "unknown",
            "message": "qa-a28 locale cron end",
            "timestamp": "2026-10-09T11:33:44.543384744Z",
            "type": "app"
          }
        ],
        "nextEndTime": "2026-10-09T11:33:41.541564943Z",
        "nextStartTime": "2026-10-09T10:35:15.979626805Z"
      },
      "server": {
        "command": "printf 'qa-a28 locale cron begin\\n'; sleep 3; printf 'qa-a28 locale cron end\\n'",
        "createdAt": "2026-10-09T11:32:25Z",
        "id": "srv-db4d1ib93q6c73at017g",
        "lastSuccessfulRunAt": "2026-10-09T11:33:46Z",
        "name": "qa-20261009-loop-a28-cron",
        "nextRunAt": "2027-01-01T00:00:00Z",
        "phase": "Running",
        "plan": "free",
        "schedule": "0 0 1 1 *",
        "type": "cron_job"
      }
    }
  },
  "times": [
    {
      "text": "1m",
      "dateTime": "2026-10-09T11:33:46Z"
    },
    {
      "text": "in 83d",
      "dateTime": "2027-01-01T00:00:00Z"
    },
    {
      "text": "2m",
      "dateTime": "2026-10-09T11:32:25Z"
    },
    {
      "text": "1m",
      "dateTime": "2026-10-09T11:33:46Z"
    },
    {
      "text": "1m",
      "dateTime": "2026-10-09T11:33:39Z"
    },
    {
      "text": "2m",
      "dateTime": "2026-10-09T11:32:28Z"
    },
    {
      "text": "2m",
      "dateTime": "2026-10-09T11:32:25Z"
    },
    {
      "text": "1m",
      "dateTime": "2026-10-09T11:33:39Z"
    }
  ]
}
```

REST and MCP service reads returned the same raw instants. These are **explicit safe projections**, not complete response bodies; unrelated sensitive service fields were excluded:

```json
{
  "rest": {
    "observedAt": "2026-10-09T11:35:57.100Z",
    "method": "GET",
    "url": "https://api.bex.co/v1/services/srv-db4d1ib93q6c73at017g",
    "status": 200,
    "safeProjection": {
      "id": "srv-db4d1ib93q6c73at017g",
      "name": "qa-20261009-loop-a28-cron",
      "type": "cron_job",
      "createdAt": "2026-10-09T11:32:25Z",
      "serviceDetails": {
        "schedule": "0 0 1 1 *",
        "command": "printf 'qa-a28 locale cron begin\\n'; sleep 3; printf 'qa-a28 locale cron end\\n'",
        "lastSuccessfulRunAt": "2026-10-09T11:33:46Z",
        "nextRunAt": "2027-01-01T00:00:00Z"
      }
    }
  },
  "mcp": {
    "observedAt": "2026-10-09T11:36:16.100Z",
    "request": {
      "jsonrpc": "2.0",
      "id": 28,
      "method": "tools/call",
      "params": {
        "name": "get_service",
        "arguments": {
          "serviceId": "srv-db4d1ib93q6c73at017g"
        }
      }
    },
    "status": 200,
    "safeProjection": {
      "id": "srv-db4d1ib93q6c73at017g",
      "name": "qa-20261009-loop-a28-cron",
      "type": "cron_job",
      "createdAt": "2026-10-09T11:32:25Z",
      "serviceDetails": {
        "schedule": "0 0 1 1 *",
        "command": "printf 'qa-a28 locale cron begin\\n'; sleep 3; printf 'qa-a28 locale cron end\\n'",
        "lastSuccessfulRunAt": "2026-10-09T11:33:46Z",
        "nextRunAt": "2027-01-01T00:00:00Z"
      }
    }
  }
}
```

Exact form values and captured preview/validation excerpts, two independent consumer paths (no schedule update submitted):

```json
{
  "settings": {
    "observedAt": "2026-10-09T11:36:51.513Z",
    "url": "https://dashboard.bex.co/services/srv-db4d1ib93q6c73at017g/settings?lang=zh",
    "lang": "zh",
    "cases": [
      {
        "value": "*/5 * * * *",
        "preview": [
          "- textbox \"计划\":",
          "- button \"保存更改\"",
          "- paragraph: Every 5 minutes · 使用 UTC"
        ]
      },
      {
        "value": "0 0 * * MON",
        "preview": [
          "- textbox \"计划\":",
          "- button \"保存更改\"",
          "- paragraph: Every Monday at 00:00 · 使用 UTC"
        ]
      },
      {
        "value": "*/40 * * * *",
        "preview": ["- textbox \"计划\":", "- button \"保存更改\""]
      },
      {
        "value": "99 99 * * *",
        "preview": [
          "- textbox \"计划\" [invalid]:",
          "- button \"保存更改\" [disabled]",
          "- paragraph: 请输入有效的 5 段 cron 表达式，例如 0 * * * *。"
        ]
      }
    ],
    "restored": "0 0 1 1 *"
  },
  "create": {
    "observedAt": "2026-10-09T11:38:06.649Z",
    "url": "https://dashboard.bex.co/services/new?type=cron_job&lang=zh",
    "lang": "zh",
    "cases": [
      {
        "value": "*/5 * * * *",
        "invalid": "false",
        "paragraphs": ["Every 5 minutes · 使用 UTC"]
      },
      {
        "value": "0 0 * * MON",
        "invalid": "false",
        "paragraphs": ["Every Monday at 00:00 · 使用 UTC"]
      },
      {
        "value": "*/40 * * * *",
        "invalid": "false",
        "paragraphs": ["5 字段 crontab 表达式（分 时 日 月 周），使用 UTC。"]
      },
      {
        "value": "99 99 * * *",
        "invalid": "true",
        "paragraphs": ["请输入有效的 5 字段 cron 表达式，例如 0 0 * * *。"]
      }
    ]
  }
}
```

## Render reference, artifacts and cleanup

[Render cron docs](https://render.com/docs/cronjobs), checked 2026-10-09, documents UTC schedule/command semantics. [Service object fields](https://api-docs.render.com/reference/service-fields) identifies lastSuccessfulRunAt as a cron field. This fix preserves those semantics. Authenticated Render Chinese text and its relative-time styles were not observed; no Render Chinese-UI parity claim.

Artifacts verified present and viewed: `.playwright-mcp/qa-a28-relative-time-zh-desktop.png` (header plus populated history), `qa-a28-relative-time-zh-mobile.png` (header, lower regions loading), `qa-a28-cron-preview-zh-settings.png` (only the owned safe cron card), `qa-a28-cron-preview-zh-create.png` and `qa-a28-cron-preview-zh-mobile.png` (scrolled so the actual preview is visible). All filenames are under `.playwright-mcp/`. Ledger: `.playwright-mcp/qa-relative-time-language-a28-ledger.json`. Local ignored screenshots supplement the durable requests/text above.

UI typed only the owned phrase `sudo delete cron job qa-20261009-loop-a28-cron`. DeleteService was accepted at 11:40:17.925Z. At 11:40:23, service and run-history GETs were 404, Overview had zero exact owned-name matches, and all six family ID sets matched baseline (5 services, 4 databases, 2 Key Value, 3 projects, 1 env group, 1 Blueprint). The QA-named pre-existing database and env group belong to other workers and remained untouched.

```json
{
  "deletion": {
    "at": "2026-10-09T11:40:17.925Z",
    "request": [
      {
        "operationName": "DeleteService",
        "variables": {
          "id": "srv-db4d1ib93q6c73at017g"
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "mutation DeleteService($id: String!, $confirm: String) {\n  deleteService(id: $id, confirm: $confirm)\n}"
      }
    ],
    "status": 200,
    "response": [
      {
        "data": {
          "deleteService": true
        }
      }
    ],
    "url": "https://dashboard.bex.co/"
  },
  "verification": {
    "observedAt": "2026-10-09T11:40:23.013Z",
    "service": {
      "method": "GET",
      "url": "https://api.bex.co/v1/services/srv-db4d1ib93q6c73at017g",
      "status": 404,
      "response": {
        "code": "NOT_FOUND",
        "error": "not found",
        "id": "not_found",
        "message": "not found",
        "params": null
      }
    },
    "runs": {
      "method": "GET",
      "url": "https://api.bex.co/v1/cron-jobs/srv-db4d1ib93q6c73at017g/runs",
      "status": 404,
      "response": {
        "code": "NOT_FOUND",
        "error": "not found",
        "id": "not_found",
        "message": "not found",
        "params": null
      }
    },
    "ownedNameCount": 0
  },
  "baselineComparison": {
    "blueprints": {
      "before": ["blp-d9nqg95cavls73fp8m10"],
      "after": ["blp-d9nqg95cavls73fp8m10"],
      "sameIds": true,
      "countBefore": 1,
      "countAfter": 1
    },
    "databases": {
      "before": [
        "dpg-d9nqg95cavls73fp8m20",
        "dpg-d9rrkoc4h4mc73edurp0",
        "dpg-d9rs3ee0ccis738kc7c0",
        "dpg-db34l9bor55s73cqpq9g"
      ],
      "after": [
        "dpg-d9nqg95cavls73fp8m20",
        "dpg-d9rrkoc4h4mc73edurp0",
        "dpg-d9rs3ee0ccis738kc7c0",
        "dpg-db34l9bor55s73cqpq9g"
      ],
      "sameIds": true,
      "countBefore": 4,
      "countAfter": 4
    },
    "envGroups": {
      "before": ["evg-db0s94c48ccs739jikr0"],
      "after": ["evg-db0s94c48ccs739jikr0"],
      "sameIds": true,
      "countBefore": 1,
      "countAfter": 1
    },
    "keyValues": {
      "before": ["red-d9p49kdrtmes73c34ovg", "red-da4086iii7bs73drbqh0"],
      "after": ["red-d9p49kdrtmes73c34ovg", "red-da4086iii7bs73drbqh0"],
      "sameIds": true,
      "countBefore": 2,
      "countAfter": 2
    },
    "projects": {
      "before": [
        "prj-d9dgeo0bd9nc73a0vh1g",
        "prj-d9e5qct5qe4s73b1mjn0",
        "prj-d9sgfnbjghus73cg6hg0"
      ],
      "after": [
        "prj-d9dgeo0bd9nc73a0vh1g",
        "prj-d9e5qct5qe4s73b1mjn0",
        "prj-d9sgfnbjghus73cg6hg0"
      ],
      "sameIds": true,
      "countBefore": 3,
      "countAfter": 3
    },
    "services": {
      "before": [
        "srv-d9bj8s3eg85c7390eb9g",
        "srv-d9bkcspg9s7c73d0n8ug",
        "srv-d9e40ei9086p3l1jri30",
        "srv-d9ndt8hmcglc739fkp50",
        "srv-d9nqg9dcavls73fp8m2g"
      ],
      "after": [
        "srv-d9bj8s3eg85c7390eb9g",
        "srv-d9bkcspg9s7c73d0n8ug",
        "srv-d9e40ei9086p3l1jri30",
        "srv-d9ndt8hmcglc739fkp50",
        "srv-d9nqg9dcavls73fp8m2g"
      ],
      "sameIds": true,
      "countBefore": 5,
      "countAfter": 5
    }
  }
}
```

`qa-login.sh --logout` ended `ok logged-out`; the old browser's whoami returned 401 at 11:41:23. Jar/state files were removed, and final cookie clearing returned zero at 11:42:54. Browser returned to sign-in. No Kube/Pod/Secret garbage-collection proof is claimed. Pre-logout sampled console after Settings and the fresh Create page had zero errors/warnings; intentional authentication teardown afterward is not part of a clean hosting-console claim.
