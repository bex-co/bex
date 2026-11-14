/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { Activity, RefreshCw } from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { Badge } from "@/common/components/ui/badge";
import { useTranslations } from "@/common/hooks/use-translations";
import { useDatabaseInsights } from "@/features/databases/hooks/use-database-insights";
import type { DatabaseParameterSpecQuery } from "@/graphql/definitions";

/**
 * `"schema"."table"` with each identifier double-quoted and embedded quotes
 * doubled, as PostgreSQL spells them. Always quoting keeps the label
 * injective — `"qa.a174"."widgets"` and `"qa"."a174.widgets"` stay distinct
 * where a bare `schema.name` join collides — so it doubles as the row key.
 */
function qualifiedTableName(
  schema: string | null,
  name: string | null,
): string {
  return [schema ?? "", name ?? ""]
    .map((part) => `"${part.replace(/"/g, '""')}"`)
    .join(".");
}

type SizeRelation = {
  schema: string | null;
  name: string | null;
  sizeBytes?: number | null;
  sizePretty: string | null;
  /** The indexed table, set only on an index's row. */
  table?: string | null;
};

/**
 * Tables and indexes in one list, largest first, as Render's sizes endpoint
 * ranks them (w4/634). A table's size excludes its indexes, which have rows of
 * their own. Tables and indexes share a schema's relation namespace, so the
 * qualified name stays a unique key across both.
 */
function sizeRows(
  tables: SizeRelation[],
  indexes: SizeRelation[],
): SizeRelation[] {
  return [...tables, ...indexes].sort(
    (a, b) => (b.sizeBytes ?? 0) - (a.sizeBytes ?? 0),
  );
}

/**
 * The database detail's Insights section: processes, top queries, sizes,
 * table scans, and non-default parameter overrides — a live introspection
 * window into the running Postgres cluster. All five datasets load
 * concurrently; each sub-section renders independently so a partial failure
 * (e.g. pg_stat_statements unavailable) only blanks that sub-section.
 */
export function InsightsPanel({ id }: { id: string }) {
  const { t } = useTranslations();
  const insights = useDatabaseInsights(id);

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <div className="space-y-1">
          <CardTitle className="flex items-center gap-2">
            <Activity className="h-4 w-4" />
            {t("databases.insightsTitle")}
          </CardTitle>
          <CardDescription>
            {t("databases.insightsDescription")}
          </CardDescription>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={insights.refetchAll}
          aria-label={t("databases.insightsRefresh")}
        >
          <RefreshCw className="h-3.5 w-3.5" />
        </Button>
      </CardHeader>

      <CardContent className="space-y-6">
        {/* Database size */}
        <Section title={t("databases.insightsSizeTitle")}>
          {insights.sizesLoading && !insights.sizes ? (
            <p className="text-sm text-muted-foreground">
              {t("databases.loading")}
            </p>
          ) : insights.sizesError ? (
            <p className="text-sm text-destructive">
              {t("databases.insightsUnavailable")}
            </p>
          ) : insights.sizes ? (
            <div className="space-y-3">
              <div className="flex items-center justify-between text-sm">
                <span className="font-mono text-muted-foreground">
                  {insights.sizes.database?.name ?? "—"}
                </span>
                <span className="font-semibold tabular-nums">
                  {insights.sizes.database?.sizePretty ?? "—"}
                </span>
              </div>
              {(insights.sizes.tables?.length ?? 0) +
                (insights.sizes.indexes?.length ?? 0) >
                0 && (
                <table className="w-full text-xs">
                  <thead>
                    <tr className="border-b text-left text-muted-foreground">
                      <th className="pb-1 pr-4 font-medium">
                        {t("databases.insightsColRelation")}
                      </th>
                      <th className="pb-1 text-right font-medium">
                        {t("databases.insightsColSize")}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {sizeRows(
                      insights.sizes.tables ?? [],
                      insights.sizes.indexes ?? [],
                    ).map((rel) => (
                      <tr
                        key={qualifiedTableName(rel.schema, rel.name)}
                        className="border-b last:border-0"
                      >
                        <td className="py-1 pr-4 font-mono text-muted-foreground">
                          {qualifiedTableName(rel.schema, rel.name)}
                          {rel.table != null && (
                            <span className="ml-2 font-sans">
                              {t("databases.insightsIndexOn", {
                                table: qualifiedTableName(
                                  rel.schema,
                                  rel.table,
                                ),
                              })}
                            </span>
                          )}
                        </td>
                        <td className="py-1 text-right tabular-nums">
                          {rel.sizePretty}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">
              {t("databases.insightsNoSizes")}
            </p>
          )}
        </Section>

        {/* Active processes */}
        <Section title={t("databases.insightsProcessesTitle")}>
          {insights.processesLoading && insights.processes.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              {t("databases.loading")}
            </p>
          ) : insights.processesError ? (
            <p className="text-sm text-destructive">
              {t("databases.insightsUnavailable")}
            </p>
          ) : insights.processes.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              {t("databases.insightsNoProcesses")}
            </p>
          ) : (
            <table className="w-full text-xs">
              <thead>
                <tr className="border-b text-left text-muted-foreground">
                  <th className="pb-1 pr-3 font-medium">PID</th>
                  <th className="pb-1 pr-3 font-medium">
                    {t("databases.insightsColUser")}
                  </th>
                  <th className="pb-1 pr-3 font-medium">
                    {t("databases.insightsColState")}
                  </th>
                  <th className="pb-1 pr-3 font-medium">
                    {t("databases.insightsColDuration")}
                  </th>
                  <th className="pb-1 font-medium">
                    {t("databases.insightsColQuery")}
                  </th>
                </tr>
              </thead>
              <tbody>
                {insights.processes.map((p) => (
                  <tr key={p?.pid} className="border-b last:border-0">
                    <td className="py-1 pr-3 tabular-nums">{p?.pid}</td>
                    <td className="py-1 pr-3 font-mono">{p?.userName}</td>
                    <td className="py-1 pr-3">
                      <StateBadge state={p?.state ?? ""} />
                    </td>
                    <td className="py-1 pr-3 tabular-nums">
                      {p?.durationSeconds}s
                    </td>
                    <td className="max-w-xs truncate py-1 font-mono text-muted-foreground">
                      <QueryCell
                        row={p ?? {}}
                        hiddenLabel={t("databases.insightsQueryHidden")}
                      />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
          {insights.processes.some((p) => isMasked(p ?? {})) ? (
            <MaskedNote text={t("databases.insightsMaskedNote")} />
          ) : null}
        </Section>

        {/* Top queries */}
        <Section title={t("databases.insightsTopQueriesTitle")}>
          {insights.topQueriesLoading && insights.topQueries.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              {t("databases.loading")}
            </p>
          ) : insights.topQueriesError ? (
            <p className="text-sm text-destructive">
              {t("databases.insightsUnavailable")}
            </p>
          ) : insights.topQueries.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              {t("databases.insightsNoTopQueries")}
            </p>
          ) : (
            // Narrow screens pan this table locally instead of crushing the
            // timing columns into character-wide strips past the card edge.
            <div data-slot="top-queries-scroll" className="overflow-x-auto">
              <table className="w-full min-w-[36rem] text-xs">
                <thead>
                  <tr className="border-b text-left text-muted-foreground">
                    <th className="pb-1 pr-3 font-medium">
                      {t("databases.insightsColQuery")}
                    </th>
                    <th className="pb-1 pr-3 text-right font-medium whitespace-nowrap">
                      {t("databases.insightsColCalls")}
                    </th>
                    <th className="pb-1 pr-3 text-right font-medium whitespace-nowrap">
                      {t("databases.insightsColTotalTime")}
                    </th>
                    <th className="pb-1 text-right font-medium whitespace-nowrap">
                      {t("databases.insightsColMeanTime")}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {insights.topQueries.map((q, i) => (
                    <tr key={i} className="border-b last:border-0">
                      <td className="max-w-xs truncate py-1 pr-3 font-mono text-muted-foreground">
                        <QueryCell
                          row={q ?? {}}
                          hiddenLabel={t("databases.insightsQueryHidden")}
                        />
                      </td>
                      <td className="py-1 pr-3 text-right whitespace-nowrap tabular-nums">
                        {q?.calls}
                      </td>
                      <td className="py-1 pr-3 text-right whitespace-nowrap tabular-nums">
                        {q?.totalTimeMs?.toFixed(1)}ms
                      </td>
                      <td className="py-1 text-right whitespace-nowrap tabular-nums">
                        {q?.meanTimeMs?.toFixed(2)}ms
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          {insights.topQueries.some((q) => isMasked(q ?? {})) ? (
            <MaskedNote text={t("databases.insightsMaskedNote")} />
          ) : null}
        </Section>

        {/* Table scans */}
        <Section title={t("databases.insightsTableScansTitle")}>
          {insights.tableScansLoading && insights.tableScans.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              {t("databases.loading")}
            </p>
          ) : insights.tableScansError ? (
            <p className="text-sm text-destructive">
              {t("databases.insightsUnavailable")}
            </p>
          ) : insights.tableScans.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              {t("databases.insightsNoTableScans")}
            </p>
          ) : (
            <table className="w-full text-xs">
              <thead>
                <tr className="border-b text-left text-muted-foreground">
                  <th className="pb-1 pr-3 font-medium">
                    {t("databases.insightsColTable")}
                  </th>
                  <th className="pb-1 pr-3 text-right font-medium">
                    {t("databases.insightsColSeqScans")}
                  </th>
                  <th className="pb-1 pr-3 text-right font-medium">
                    {t("databases.insightsColIdxScans")}
                  </th>
                  <th className="pb-1 text-right font-medium">
                    {t("databases.insightsColLiveRows")}
                  </th>
                </tr>
              </thead>
              <tbody>
                {insights.tableScans.map((r) => (
                  <tr
                    key={qualifiedTableName(r.schema, r.name)}
                    className="border-b last:border-0"
                  >
                    <td className="py-1 pr-3 font-mono text-muted-foreground">
                      {qualifiedTableName(r.schema, r.name)}
                    </td>
                    <td className="py-1 pr-3 text-right tabular-nums">
                      {r.seqScans ?? 0}
                    </td>
                    <td className="py-1 pr-3 text-right tabular-nums">
                      {r.indexScans ?? 0}
                    </td>
                    <td className="py-1 text-right tabular-nums">
                      {r.liveRows ?? 0}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </Section>

        {/* The DECLARED parameters, read-only and shown only when a database
            still carries some: parameterOverrides was retired to match Render
            (w2/066), but a set declared before then keeps applying. */}
        {insights.parameterSpec.length > 0 && (
          <Section
            title={t("databases.insightsParamsTitle")}
            description={t("databases.insightsParamsDescription")}
          >
            <DeclaredParameters parameters={insights.parameterSpec} />
          </Section>
        )}

        {/* The OBSERVED effective config, read-only. Mostly the platform's —
            a database nobody has configured still lists ~48 rows. */}
        <Section
          title={t("databases.insightsEffectiveParamsTitle")}
          description={t("databases.insightsEffectiveParamsDescription")}
        >
          {insights.parameterOverridesLoading &&
          insights.parameterOverrides.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              {t("databases.loading")}
            </p>
          ) : insights.parameterOverridesError ? (
            <p className="text-sm text-destructive">
              {t("databases.insightsUnavailable")}
            </p>
          ) : insights.parameterOverrides.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              {t("databases.insightsNoParams")}
            </p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-xs">
                <thead>
                  <tr className="border-b text-left text-muted-foreground">
                    <th className="pb-1 pr-3 font-medium">
                      {t("databases.insightsColParam")}
                    </th>
                    <th className="pb-1 pr-3 font-medium">
                      {t("databases.insightsColSetting")}
                    </th>
                    <th className="pb-1 font-medium">
                      {t("databases.insightsColSource")}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {insights.parameterOverrides.map((parameter) => (
                    <tr key={parameter.name} className="border-b last:border-0">
                      <td className="py-1 pr-3 font-mono text-muted-foreground">
                        {parameter.name}
                      </td>
                      <td className="py-1 pr-3 font-mono">
                        {parameter.setting}
                        {parameter.unit ? ` ${parameter.unit}` : ""}
                      </td>
                      <td className="py-1 text-muted-foreground">
                        {parameter.source}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Section>
      </CardContent>
    </Card>
  );
}

type DeclaredParameter = NonNullable<
  NonNullable<DatabaseParameterSpecQuery["databaseParameterSpec"]>[number]
>;

/** Read-only view of Database.spec.parameters beside what pg_settings reports
 * for each name (w4/m132). */
function DeclaredParameters({
  parameters,
}: {
  parameters: DeclaredParameter[];
}) {
  const { t } = useTranslations();
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-xs">
        <thead>
          <tr className="border-b text-left text-muted-foreground">
            <th className="pb-1 pr-3 font-medium">
              {t("databases.insightsColParam")}
            </th>
            <th className="pb-1 pr-3 font-medium">
              {t("databases.insightsParamsDeclaredValue")}
            </th>
            <th className="pb-1 font-medium">
              {t("databases.insightsParamsObservedValue")}
            </th>
          </tr>
        </thead>
        <tbody>
          {parameters.map((parameter, index) => (
            <tr
              key={parameter.name ?? index}
              className="border-b last:border-0"
            >
              <td className="py-1 pr-3 font-mono text-muted-foreground">
                {parameter.name}
              </td>
              <td className="py-1 pr-3 font-mono">{parameter.value}</td>
              <td className="py-1">
                {parameter.observationStatus === "observed" ? (
                  <div>
                    <code>
                      {parameter.observedSetting}
                      {parameter.observedUnit
                        ? ` ${parameter.observedUnit}`
                        : ""}
                    </code>
                    <p className="text-muted-foreground">
                      {t("databases.insightsParamsObservedHint")}
                    </p>
                  </div>
                ) : (
                  <p className="text-amber-700 dark:text-amber-400">
                    {t(
                      parameter.observationStatus === "not_observed"
                        ? "databases.insightsParamsNotObserved"
                        : "databases.insightsParamsObservationUnavailable",
                    )}
                  </p>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function Section({
  title,
  description,
  children,
}: {
  title: string;
  description?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-2">
      <div className="space-y-0.5">
        <h3 className="text-sm font-medium">{title}</h3>
        {description ? (
          <p className="text-xs text-muted-foreground">{description}</p>
        ) : null}
      </div>
      {children}
    </div>
  );
}

/**
 * PostgreSQL's own placeholder for a query text the reading role may not see.
 * The API blanks it and sets `masked` instead (w4/m115), but an older bex-api
 * still sends the literal — matching it here keeps it out of the DOM either
 * way. Stable across PostgreSQL 9.6–18 (pgstatfuncs.c).
 */
const PG_INSUFFICIENT_PRIVILEGE = "<insufficient privilege>";

/** A row is hidden when the API says so, or when it still carries the literal. */
function isMasked(row: {
  masked?: boolean | null;
  query?: string | null;
}): boolean {
  return Boolean(row.masked) || row.query === PG_INSUFFICIENT_PRIVILEGE;
}

/**
 * The Query cell for a process or top-query row: real SQL, an honest "hidden"
 * marker for a masked row, or an em dash when there is simply no query.
 */
function QueryCell({
  row,
  hiddenLabel,
}: {
  row: { masked?: boolean | null; query?: string | null };
  hiddenLabel: string;
}) {
  if (isMasked(row)) {
    return <span className="italic">{hiddenLabel}</span>;
  }
  return <>{row.query || "—"}</>;
}

/** Explains, once per sub-section, why some rows show no SQL. */
function MaskedNote({ text }: { text: string }) {
  return <p className="text-xs text-muted-foreground">{text}</p>;
}

function StateBadge({ state }: { state: string }) {
  const variant =
    state === "active" ? "default" : state === "idle" ? "secondary" : "outline";
  return (
    <Badge variant={variant} className="text-xs">
      {state || "—"}
    </Badge>
  );
}
