import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

/**
 * The mutation-error toast invariant (w1/m145).
 *
 * When bex-api refuses a request it says why: "schedule must be a valid
 * 5-field cron expression", "total secret file size limit of 524288 bytes
 * exceeded". Showing that sentence is the only thing that turns a refusal into
 * something the user can fix. w6/037 did that for `useFieldMutation`, but 77+
 * other catch blocks kept `catch { toast.error(t("…")) }`, so the same
 * refusals still read "Please try again", which no retry ever fixes.
 *
 * This guard reads every source file. Inside a `catch` block, each
 * `toast.error(…)` must use the caught error, normally as
 * `mutationErrorMessage(err, t("…"))`, which keeps the generic copy for a
 * transport failure or throttling. A toast with no server refusal to relay (a
 * read, a Kratos call, a dedicated branch message) goes on ALLOWED with the
 * reason. Structural on purpose, like `one-rail-invariant.test.ts`: it catches
 * the shape where it is written, not after a QA pass finds it live.
 */
const SRC_DIR = join(import.meta.dirname, "../../..");

const READ = "a read, not a refusable edit";
const KRATOS = "a Kratos call through /api/sessions, not a bex-api request";
const DEDICATED = "dedicated copy for a refusal this branch already matched";
const STRIPE =
  "the redacted reason is an operator-side Stripe gap the user cannot fix; logged";
const SHARED_MESSAGE =
  "message is mutationErrorMessage(err, …), shared with the inline error";

const ALLOWED: Record<string, string> = {
  'common/components/user-nav.tsx: toast.error(t("auth.logoutFailedSubtitle"))':
    "Kratos logout, not a bex-api request; the cause is logged",
  "common/hooks/use-copy-to-clipboard.ts: toast.error(errorText)":
    "the browser clipboard API failed; there is no server to relay",
  'features/blueprints/hooks/use-disconnect-blueprint.ts: toast.error(t("blueprints.disconnectBusy"))':
    DEDICATED,
  'features/blueprints/hooks/use-sync-blueprint.ts: toast.error(t("blueprints.syncSourceChanged"))':
    DEDICATED,
  'features/blueprints/hooks/use-sync-blueprint.ts: toast.error(t("blueprints.syncBusy"))':
    DEDICATED,
  'features/deploys/hooks/use-deploys.ts: toast.error(t("deploys.listLoadMoreError"))':
    READ,
  'features/databases/hooks/use-create-database.ts: toast.error(t("databases.fieldPhysicalNameReserved",{name}))':
    DEDICATED,
  'features/databases/hooks/use-rename-database.ts: toast.error(t("databases.nameConflict"))':
    DEDICATED,
  'features/databases/hooks/use-rename-database.ts: toast.error(t("databases.nameInvalid"))':
    DEDICATED,
  'features/keyvalue/hooks/use-rename-key-value.ts: toast.error(t("keyvalue.nameConflict"))':
    DEDICATED,
  'features/keyvalue/hooks/use-rename-key-value.ts: toast.error(t("keyvalue.nameInvalid"))':
    DEDICATED,
  'features/services/components/service-environment-editor.tsx: toast.error(t("services.envExportError"))':
    READ,
  'features/services/components/service-environment-editor.tsx: toast.error(t(kind==="env"?"services.envRevealError":"services.secretFileRevealError"))':
    READ,
  'features/services/hooks/use-cron-runs.ts: toast.error(t("services.cronRunsLoadError"))':
    READ,
  "features/services/hooks/use-disks.ts: toast.error(message)": SHARED_MESSAGE,
  "features/webhooks/hooks/use-create-webhook.ts: toast.error(message)":
    SHARED_MESSAGE,
  "features/webhooks/hooks/use-update-webhook.ts: toast.error(message)":
    SHARED_MESSAGE,
  'features/sessions/hooks/use-active-sessions.ts: toast.error(t("activeSessions.revokeError"))':
    KRATOS,
  'features/sessions/hooks/use-active-sessions.ts: toast.error(t("activeSessions.signOutOthersError"))':
    KRATOS,
  'features/team/hooks/use-invite-member.ts: toast.error(t("team.inviteError",{email}))':
    "reached only when refusalReason(err) is empty; a reason is shown inline",
  'features/usage/hooks/use-billing-onboarding.ts: toast.error(t("usage.billingCheckoutError"))':
    STRIPE,
  'features/usage/hooks/use-billing-onboarding.ts: toast.error(t("usage.billingPortalError"))':
    STRIPE,
};

const AGENT_SESSIONS = "agent-session REST errors; follow-up w5/166";
const INTERPOLATED =
  "the raw text is interpolated into translated copy; follow-up w5/166";
const ENV_GROUPS = "toast description under translated copy; follow-up w5/166";

/**
 * Catch blocks that still hand the caught error's raw `.message` to a toast or
 * an inline error, call → why. A transport failure's message is the browser's
 * own English ("Failed to fetch"), untranslated and naming nothing the user can
 * act on, so a dialog relays a server refusal through `mutationErrorMessage` /
 * `refusalReason` and otherwise shows its own copy (w5/164).
 */
const ALLOWED_RAW_MESSAGE: Record<string, string> = {
  "features/agent-sessions/components/session-detail-header.tsx: toast.error(errinstanceofError?err.message:String(err))":
    AGENT_SESSIONS,
  'features/databases/components/sql-console.tsx: setError(causeinstanceofError&&cause.message?cause.message:t("databases.sqlUnknownError"))':
    "Postgres's own error for the user's SQL; transport fallback is w5/166",
  'features/databases/hooks/use-access-control.ts: toast.error(t("databases.accessAllowListError",{error:(easError).message}))':
    INTERPOLATED,
  'features/databases/hooks/use-access-control.ts: toast.error(hasGraphQLErrorCode(e,"POSTGRES_IDENTIFIER_RESERVED")?t("databases.fieldPhysicalNameReserved",{name}):t("databases.accessUserError",{error:(easError).message}))':
    INTERPOLATED,
  'features/databases/hooks/use-recovery.ts: toast.error(t("databases.recoveryRestoreError",{error:(easError).message,}))':
    INTERPOLATED,
  'features/env-groups/hooks/use-env-groups.ts: toast.error(t("envGroups.createError",{name:input.name}),{description:errorinstanceofError?error.message:undefined,})':
    ENV_GROUPS,
  'features/env-groups/hooks/use-env-groups.ts: toast.error(t("envGroups.renameError"),{description:errorinstanceofError?error.message:undefined,})':
    ENV_GROUPS,
  'features/env-groups/hooks/use-env-groups.ts: toast.error(t("envGroups.moveError"),{description:errorinstanceofError?error.message:undefined,})':
    ENV_GROUPS,
  'features/env-groups/hooks/use-env-groups.ts: toast.error(t("envGroups.cloneError"),{description:errorinstanceofError?error.message:undefined,})':
    ENV_GROUPS,
  'features/keyvalue/hooks/use-key-value-networking.ts: toast.error(t("keyvalue.networkingError",{error:(easError).message}))':
    INTERPOLATED,
  'features/services/hooks/use-service-networking.ts: toast.error(t("services.networkingError",{error:(easError).message}))':
    INTERPOLATED,
};

/** Every non-test source file under src, as path relative to src → contents. */
function readSources(): Map<string, string> {
  const sources = new Map<string, string>();
  for (const path of readdirSync(SRC_DIR, {
    recursive: true,
    encoding: "utf8",
  })) {
    if (
      /\.tsx?$/.test(path) &&
      !/\.test\.tsx?$/.test(path) &&
      !path.split(/[\\/]/).includes("__tests__")
    ) {
      sources.set(path, readFileSync(join(SRC_DIR, path), "utf8"));
    }
  }
  return sources;
}

/** Index of the bracket closing the one at `open`, skipping strings and comments. */
function closingIndex(src: string, open: number): number {
  const opener = src[open];
  const closer = opener === "(" ? ")" : "}";
  let depth = 0;
  for (let i = open; i < src.length; i++) {
    const c = src[i];
    if (c === "/" && src[i + 1] === "/") {
      i = src.indexOf("\n", i);
      if (i < 0) break;
    } else if (c === "/" && src[i + 1] === "*") {
      i = src.indexOf("*/", i) + 1;
    } else if (c === '"' || c === "'" || c === "`") {
      for (i++; i < src.length && src[i] !== c; i++) if (src[i] === "\\") i++;
    } else if (c === opener) {
      depth++;
    } else if (c === closer && --depth === 0) {
      return i;
    }
  }
  return src.length;
}

const CATCH = /\bcatch\s*(?:\(\s*([A-Za-z_$][\w$]*)[^)]*\))?\s*\{/g;

/** Each `callee(…)` call inside a catch block, with the block's error binding. */
function* catchBlockCalls(src: string, callee: RegExp) {
  for (const m of src.matchAll(CATCH)) {
    const open = m.index + m[0].length - 1;
    const body = src.slice(open, closingIndex(src, open) + 1);
    for (const call of body.matchAll(callee)) {
      const paren = call.index + call[0].length - 1;
      const text = body.slice(call.index, closingIndex(body, paren) + 1);
      yield { binding: m[1], call: text, args: text.slice(call[0].length) };
    }
  }
}

/**
 * True when args read binding as a value. Only the arguments count, and not as
 * a property name: a binding called `error` must not be "read" by
 * `toast.error` itself.
 */
function reads(binding: string | undefined, args: string): boolean {
  return !!binding && new RegExp(`(?<![\\w$.])${binding}(?![\\w$])`).test(args);
}

const compact = (call: string) => call.replace(/\s+/g, "").replace(/,\)/g, ")");

/** Each toast.error(…) in a catch block that never mentions the caught error, whitespace removed. */
function blindErrorToasts(src: string): string[] {
  return [...catchBlockCalls(src, /\btoast\.error\(/g)]
    .filter(({ binding, args }) => !reads(binding, args))
    .map(({ call }) => compact(call));
}

/** Each toast.error(…) or set…(…) call in a catch block that shows the caught error's `.message`, whitespace removed. */
function rawMessageReads(src: string): string[] {
  return [...catchBlockCalls(src, /\b(?:toast\.error|set[A-Z][\w$]*)\(/g)]
    .filter(
      ({ binding, args }) => reads(binding, args) && /\.message\b/.test(args),
    )
    .map(({ call }) => compact(call));
}

describe("mutation-error toast invariant (w1/m145)", () => {
  const sources = readSources();
  const offenders = [...sources].flatMap(([path, src]) =>
    blindErrorToasts(src).map((call) => `${path}: ${call}`),
  );

  it("every toast.error in a catch block relays the caught error, or is allowlisted", () => {
    expect(offenders.filter((offender) => !(offender in ALLOWED))).toEqual([]);
  });

  it("has no stale allowlist entries", () => {
    expect(
      Object.keys(ALLOWED).filter((entry) => !offenders.includes(entry)),
    ).toEqual([]);
  });

  it("flags the bare shape w1/m145 removed and passes its fix", () => {
    expect(
      blindErrorToasts(
        `try { await save(); } catch { toast.error(t("services.deployError")); }`,
      ),
    ).toEqual(['toast.error(t("services.deployError"))']);
    // Reading the error elsewhere in the block does not excuse the toast.
    expect(
      blindErrorToasts(
        `try { await save(); } catch (err) { log(err); toast.error(t("services.createError")); }`,
      ),
    ).toHaveLength(1);
    // A binding named `error` is not read by the call name `toast.error`.
    expect(
      blindErrorToasts(
        `try { await save(); } catch (error) { toast.error(t("services.deployError")); }`,
      ),
    ).toHaveLength(1);
    expect(
      blindErrorToasts(
        `try { await save(); } catch (err) { toast.error(mutationErrorMessage(err, t("services.deployError"))); }`,
      ),
    ).toEqual([]);
  });

  describe("no raw transport text (w5/164)", () => {
    const rawReaders = [...sources].flatMap(([path, src]) =>
      rawMessageReads(src).map((call) => `${path}: ${call}`),
    );

    it("no catch block shows the caught error's raw message, or is allowlisted", () => {
      expect(
        rawReaders.filter((reader) => !(reader in ALLOWED_RAW_MESSAGE)),
      ).toEqual([]);
    });

    it("has no stale allowlist entries", () => {
      expect(
        Object.keys(ALLOWED_RAW_MESSAGE).filter(
          (entry) => !rawReaders.includes(entry),
        ),
      ).toEqual([]);
    });

    it("flags the shape w5/164 removed and passes its fix", () => {
      expect(
        rawMessageReads(
          `try { await save(); } catch (err) { setError(err instanceof Error ? err.message : t("workspaces.renameError")); }`,
        ),
      ).toHaveLength(1);
      expect(
        rawMessageReads(
          `try { await save(); } catch (error) { toast.error(t("x"), { description: error.message }); }`,
        ),
      ).toHaveLength(1);
      expect(
        rawMessageReads(
          `try { await save(); } catch (e) { toast.error(t("x", { error: (e as Error).message })); }`,
        ),
      ).toHaveLength(1);
      expect(
        rawMessageReads(
          `try { await save(); } catch (err) { setError(mutationErrorMessage(err, t("workspaces.renameError"))); }`,
        ),
      ).toEqual([]);
    });
  });

  it("scans a non-trivial number of catch blocks", () => {
    // Keeps the assertions above from passing vacuously if the walk breaks.
    const catches = [...sources.values()].reduce(
      (n, src) => n + [...src.matchAll(CATCH)].length,
      0,
    );
    expect(catches).toBeGreaterThan(100);
  });
});
