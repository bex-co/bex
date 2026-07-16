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
import { ParameterOverridesEditor } from "@/features/databases/components/parameter-overrides-editor";

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
              {(insights.sizes.tables?.length ?? 0) > 0 && (
                <table className="w-full text-xs">
                  <thead>
                    <tr className="border-b text-left text-muted-foreground">
                      <th className="pb-1 pr-4 font-medium">
                        {t("databases.insightsColTable")}
                      </th>
                      <th className="pb-1 text-right font-medium">
                        {t("databases.insightsColSize")}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {insights.sizes.tables?.map((tbl) => (
                      <tr
                        key={`${tbl?.schema}.${tbl?.name}`}
                        className="border-b last:border-0"
                      >
                        <td className="py-1 pr-4 font-mono text-muted-foreground">
                          {tbl?.schema}.{tbl?.name}
                        </td>
                        <td className="py-1 text-right tabular-nums">
                          {tbl?.sizePretty}
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
                      {p?.query || "—"}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
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
            <table className="w-full text-xs">
              <thead>
                <tr className="border-b text-left text-muted-foreground">
                  <th className="pb-1 pr-3 font-medium">
                    {t("databases.insightsColQuery")}
                  </th>
                  <th className="pb-1 pr-3 text-right font-medium">
                    {t("databases.insightsColCalls")}
                  </th>
                  <th className="pb-1 pr-3 text-right font-medium">
                    {t("databases.insightsColTotalTime")}
                  </th>
                  <th className="pb-1 text-right font-medium">
                    {t("databases.insightsColMeanTime")}
                  </th>
                </tr>
              </thead>
              <tbody>
                {insights.topQueries.map((q, i) => (
                  <tr key={i} className="border-b last:border-0">
                    <td className="max-w-xs truncate py-1 pr-3 font-mono text-muted-foreground">
                      {q?.query}
                    </td>
                    <td className="py-1 pr-3 text-right tabular-nums">
                      {q?.calls}
                    </td>
                    <td className="py-1 pr-3 text-right tabular-nums">
                      {q?.totalTimeMs?.toFixed(1)}ms
                    </td>
                    <td className="py-1 text-right tabular-nums">
                      {q?.meanTimeMs?.toFixed(2)}ms
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
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
                    key={`${r?.schema}.${r?.name}`}
                    className="border-b last:border-0"
                  >
                    <td className="py-1 pr-3 font-mono text-muted-foreground">
                      {r?.schema}.{r?.name}
                    </td>
                    <td className="py-1 pr-3 text-right tabular-nums">
                      {r?.seqScans ?? 0}
                    </td>
                    <td className="py-1 pr-3 text-right tabular-nums">
                      {r?.indexScans ?? 0}
                    </td>
                    <td className="py-1 text-right tabular-nums">
                      {r?.liveRows ?? 0}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </Section>

        {/* Parameter overrides */}
        <Section title={t("databases.insightsParamsTitle")}>
          {insights.parameterOverridesLoading &&
          insights.parameterOverrides.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              {t("databases.loading")}
            </p>
          ) : insights.parameterOverridesError ? (
            <p className="text-sm text-destructive">
              {t("databases.insightsUnavailable")}
            </p>
          ) : (
            <ParameterOverridesEditor
              key={insights.parameterOverrides
                .map((parameter) =>
                  [parameter.name, parameter.setting, parameter.source].join(
                    ":",
                  ),
                )
                .join("|")}
              overrides={insights.parameterOverrides}
              saving={insights.saving}
              onSave={insights.saveParameters}
            />
          )}
        </Section>
      </CardContent>
    </Card>
  );
}

function Section({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-2">
      <h3 className="text-sm font-medium">{title}</h3>
      {children}
    </div>
  );
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
