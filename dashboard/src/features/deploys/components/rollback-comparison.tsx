import type { ReactNode } from "react";
import { Loader2 } from "lucide-react";
import {
  useTranslations,
  type Translate,
} from "@/common/hooks/use-translations";
import {
  PLACEHOLDER_SENTINEL,
  aroundPlaceholder,
} from "@/common/lib/i18n-placeholder";
import type { RollbackComparisonQuery } from "@/graphql/definitions";

type Comparison = NonNullable<RollbackComparisonQuery["rollbackComparison"]>;
type Snapshot = NonNullable<Comparison["deploy"]["serviceConfigSnapshot"]>;
type ScalarField =
  | "runCommand"
  | "preDeployCommand"
  | "dockerCommand"
  | "healthCheckPath"
  | "instances";

/**
 * The comparison's frame: Render's `min-h-[9.25rem]`, which its loading
 * spinner fills and every other state keeps, so the block does not jump when
 * the answer lands (w1/m646).
 */
export const ROLLBACK_COMPARISON_FRAME_CLASS = "min-h-[9.25rem]";

// Render's masked value: every env var value reads the same.
const MASKED_VALUE = "••••••••••••";

const SCALAR_ROWS: ReadonlyArray<{
  field: ScalarField;
  label:
    | "deploys.rollbackRowStartCommand"
    | "deploys.rollbackRowPreDeployCommand"
    | "deploys.rollbackRowDockerCommand"
    | "deploys.rollbackRowHealthCheckPath"
    | "deploys.rollbackRowInstances";
}> = [
  { field: "runCommand", label: "deploys.rollbackRowStartCommand" },
  { field: "preDeployCommand", label: "deploys.rollbackRowPreDeployCommand" },
  { field: "dockerCommand", label: "deploys.rollbackRowDockerCommand" },
  { field: "healthCheckPath", label: "deploys.rollbackRowHealthCheckPath" },
  { field: "instances", label: "deploys.rollbackRowInstances" },
];

export interface RollbackComparisonBodyProps {
  loading: boolean;
  error: boolean;
  data: Comparison | null | undefined;
}

/**
 * Render's rollback comparison (docs/render-artifacts/rollback-comparison.md):
 * one row per changed setting, in Render's order, each only when it changed.
 * The `-` line is the live value and the `+` line the target's; env var values
 * and secret file contents never appear, since the read carries none.
 */
export function RollbackComparisonBody({
  loading,
  error,
  data,
}: RollbackComparisonBodyProps) {
  const { t } = useTranslations();
  if (loading && !data) {
    return (
      <div
        role="status"
        className={`flex items-center justify-center px-5 py-4 ${ROLLBACK_COMPARISON_FRAME_CLASS}`}
      >
        <Loader2
          aria-hidden
          className="size-5 animate-spin text-muted-foreground motion-reduce:animate-none"
        />
        <span className="sr-only">{t("deploys.rollbackLoading")}</span>
      </div>
    );
  }
  const current = data?.currentLiveDeploy?.serviceConfigSnapshot;
  const old = data?.deploy.serviceConfigSnapshot;
  if (error || !data || !current || !old) {
    return (
      <p className={`px-5 py-4 text-sm ${ROLLBACK_COMPARISON_FRAME_CLASS}`}>
        {t("deploys.rollbackUnavailable")}
      </p>
    );
  }
  const rows: Array<{ key: string; label: string; body: ReactNode }> = [];
  for (const { field, label } of SCALAR_ROWS) {
    const live = current[field] ?? null;
    const target = old[field] ?? null;
    if (live === target) continue;
    rows.push({
      key: field,
      label: t(label),
      body: (
        <DiffLines
          oldValue={live === null ? t("deploys.rollbackNone") : String(live)}
          newValue={
            target === null ? t("deploys.rollbackNone") : String(target)
          }
        />
      ),
    });
  }
  const envVars = envVarRows(old);
  if (envVars.length > 0) {
    rows.push({
      key: "envVars",
      label: t("deploys.rollbackRowEnvVars"),
      body: <EnvVarTable rows={envVars} />,
    });
  }
  const groups = envGroupSentences(current, old, t);
  if (groups.length > 0) {
    rows.push({
      key: "envGroup",
      label: t("deploys.rollbackRowEnvGroup"),
      body: <Sentences items={groups} />,
    });
  }
  const files = secretFileSentences(old, t);
  if (files.length > 0) {
    rows.push({
      key: "secretFiles",
      label: t("deploys.rollbackRowSecretFiles"),
      body: <Sentences items={files} />,
    });
  }
  if (data.currentPlanLower) {
    rows.push({
      key: "plan",
      label: t("deploys.rollbackRowPlan"),
      body: (
        <p className="text-sm text-amber-700 dark:text-amber-400">
          {t("deploys.rollbackPlanWarning")}
        </p>
      ),
    });
  }
  if (rows.length === 0) {
    return (
      <p className={`px-5 py-4 text-sm ${ROLLBACK_COMPARISON_FRAME_CLASS}`}>
        {t("deploys.rollbackNoChanges")}
      </p>
    );
  }
  return (
    <ul className={`divide-y px-5 ${ROLLBACK_COMPARISON_FRAME_CLASS}`}>
      {rows.map((row) => (
        <li
          key={row.key}
          className="grid gap-2 py-4 sm:grid-cols-[minmax(0,1fr)_minmax(0,4fr)] sm:gap-4"
        >
          <h3 className="text-sm font-medium">{row.label}</h3>
          <div className="min-w-0 space-y-3">{row.body}</div>
        </li>
      ))}
    </ul>
  );
}

/** One change: Render's `-` (the live value) over `+` (the target's). */
function DiffLines({
  oldValue,
  newValue,
}: {
  oldValue?: ReactNode;
  newValue?: ReactNode;
}) {
  const { t } = useTranslations();
  return (
    <div
      role="group"
      aria-label={t("deploys.rollbackChange")}
      className="font-mono text-xs"
    >
      {oldValue !== undefined ? (
        <del className="flex gap-1 rounded-sm bg-destructive/10 px-1 py-0.5 text-destructive no-underline">
          <span aria-hidden className="w-4 shrink-0 select-none text-center">
            -
          </span>
          <span className="sr-only">{t("deploys.rollbackRemoved")}</span>
          <span className="min-w-0 break-all">{oldValue}</span>
        </del>
      ) : null}
      {newValue !== undefined ? (
        <ins className="flex gap-1 rounded-sm bg-emerald-500/10 px-1 py-0.5 text-emerald-700 no-underline dark:text-emerald-400">
          <span aria-hidden className="w-4 shrink-0 select-none text-center">
            +
          </span>
          <span className="sr-only">{t("deploys.rollbackAdded")}</span>
          <span className="min-w-0 break-all">{newValue}</span>
        </ins>
      ) : null}
    </div>
  );
}

type EnvVarRow = { key: string; removed: boolean; added: boolean };

/** Render's env var rows: one per key, sorted; a created key on the `+`
 *  line, a deleted one on the `-` line, an updated one on both. */
function envVarRows(old: Snapshot): EnvVarRow[] {
  const { createdVarKeys, deletedVarKeys, updatedVarKeys } = old.envVarChanges;
  return [
    ...createdVarKeys.map((key) => ({ key, removed: false, added: true })),
    ...deletedVarKeys.map((key) => ({ key, removed: true, added: false })),
    ...updatedVarKeys.map((key) => ({ key, removed: true, added: true })),
  ].sort((a, b) => (a.key < b.key ? -1 : a.key > b.key ? 1 : 0));
}

function EnvVarTable({ rows }: { rows: EnvVarRow[] }) {
  const { t } = useTranslations();
  const cell = (key: string) => (
    <span className="grid grid-cols-[minmax(6rem,1fr)_minmax(0,2fr)] gap-3">
      <code className="min-w-0 break-all">{key}</code>
      <span>{MASKED_VALUE}</span>
    </span>
  );
  return (
    <div role="table" className="min-w-0 text-xs">
      <div
        role="row"
        className="mb-1 grid grid-cols-[minmax(6rem,1fr)_minmax(0,2fr)] gap-3 pl-6 font-mono text-[11px] tracking-wide text-muted-foreground uppercase"
      >
        <span role="columnheader">{t("deploys.rollbackEnvKey")}</span>
        <span role="columnheader">{t("deploys.rollbackEnvValue")}</span>
      </div>
      <div role="rowgroup" className="divide-y divide-dashed">
        {rows.map((row) => (
          <div role="row" key={row.key} className="py-1.5">
            <div role="cell">
              <DiffLines
                oldValue={row.removed ? cell(row.key) : undefined}
                newValue={row.added ? cell(row.key) : undefined}
              />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

type Sentence = { key: string; text: ReactNode };

/** A translated sentence with one value styled where the language puts it. */
function styled(
  translated: string,
  value: ReactNode,
  wrap: (value: ReactNode) => ReactNode,
): ReactNode {
  const [before, after] = aroundPlaceholder(translated);
  return (
    <>
      {before}
      {wrap(value)}
      {after}
    </>
  );
}

const bold = (value: ReactNode) => (
  <strong className="font-semibold">{value}</strong>
);
const code = (value: ReactNode) => (
  <code className="rounded bg-muted px-1 py-0.5 font-mono text-xs">
    {value}
  </code>
);

/** Render's Environment Group sentences for a service: groups linked or
 *  unlinked since this deploy, the ones deleted since, and, when this deploy
 *  linked any, that their current values apply. */
function envGroupSentences(
  current: Snapshot,
  old: Snapshot,
  t: Translate,
): Sentence[] {
  const oldIds = new Set(old.envGroups.map((g) => g.id));
  const currentIds = new Set(current.envGroups.map((g) => g.id));
  const out: Sentence[] = [];
  for (const g of current.envGroups.filter((g) => !oldIds.has(g.id))) {
    out.push({
      key: `unlink-${g.id}`,
      text: styled(
        t("deploys.rollbackGroupUnlink", { name: PLACEHOLDER_SENTINEL }),
        g.name,
        bold,
      ),
    });
  }
  for (const g of old.envGroups.filter((g) => !currentIds.has(g.id))) {
    out.push({
      key: `link-${g.id}`,
      text: styled(
        t("deploys.rollbackGroupLink", { name: PLACEHOLDER_SENTINEL }),
        g.name,
        bold,
      ),
    });
  }
  const deleted = old.deletedEnvGroupCount;
  if (deleted > 0) {
    out.push({
      key: "deleted",
      text: styled(
        t("deploys.rollbackGroupDeleted", {
          count: deleted,
          n: PLACEHOLDER_SENTINEL,
        }),
        deleted,
        bold,
      ),
    });
  }
  if (old.envGroups.length > 0) {
    out.push({
      key: "current-values",
      text: t("deploys.rollbackGroupCurrentValues"),
    });
  }
  return out;
}

function secretFileSentences(old: Snapshot, t: Translate): Sentence[] {
  const { createdFileKeys, deletedFileKeys, updatedFileKeys } =
    old.envVarChanges;
  const sentence = (
    kind: "added" | "deleted" | "changed",
    name: string,
  ): Sentence => ({
    key: `${kind}-${name}`,
    text: styled(
      t(
        kind === "added"
          ? "deploys.rollbackSecretFileAdded"
          : kind === "deleted"
            ? "deploys.rollbackSecretFileDeleted"
            : "deploys.rollbackSecretFileChanged",
        { name: PLACEHOLDER_SENTINEL },
      ),
      name,
      code,
    ),
  });
  return [
    ...createdFileKeys.map((name) => sentence("added", name)),
    ...deletedFileKeys.map((name) => sentence("deleted", name)),
    ...updatedFileKeys.map((name) => sentence("changed", name)),
  ];
}

function Sentences({ items }: { items: Sentence[] }) {
  return (
    <ul className="space-y-2 text-sm">
      {items.map((item) => (
        <li key={item.key}>{item.text}</li>
      ))}
    </ul>
  );
}
