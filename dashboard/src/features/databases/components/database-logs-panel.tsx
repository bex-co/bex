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

import { ScrollText, RefreshCw } from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { useDatabaseLogs } from "@/features/databases/hooks/use-database-logs";

/**
 * Shows the most recent 20 lines from the live CNPG pod logs for a managed
 * Postgres database. CNPG pods are not shipped to Loki, so this is a direct
 * pod-log read — no durable history; results reflect only running pods.
 */
export function DatabaseLogsPanel({ id }: { id: string }) {
  const { t } = useTranslations();
  const { entries, loading, error, refetch } = useDatabaseLogs({ id });

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <div className="space-y-1">
          <CardTitle className="flex items-center gap-2">
            <ScrollText className="h-4 w-4" />
            {t("databases.logsTitle")}
          </CardTitle>
          <CardDescription>{t("databases.logsDescription")}</CardDescription>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={refetch}
          disabled={loading}
          aria-label={t("databases.logsRefresh")}
        >
          <RefreshCw className="h-3.5 w-3.5" />
        </Button>
      </CardHeader>

      <CardContent>
        {loading && entries.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t("databases.loading")}</p>
        ) : error ? (
          <p className="text-sm text-destructive">
            {t("databases.logsUnavailable")}
          </p>
        ) : entries.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t("databases.logsEmpty")}</p>
        ) : (
          <div className="max-h-96 overflow-y-auto rounded-md border bg-muted/50 p-3">
            <pre className="whitespace-pre-wrap break-all font-mono text-xs leading-relaxed">
              {entries
                .map((e) => {
                  const ts = e.timestamp ? e.timestamp.slice(0, 23).replace("T", " ") : "";
                  const msg = e.message ?? "";
                  return ts ? `${ts}  ${msg}` : msg;
                })
                .join("\n")}
            </pre>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
