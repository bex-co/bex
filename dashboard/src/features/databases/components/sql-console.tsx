import { useMemo, useState } from "react";
import { Clock3, DatabaseZap, Play, Trash2 } from "lucide-react";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { Alert, AlertDescription } from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
import { Textarea } from "@/common/components/ui/textarea";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  useExecuteDatabaseQuery,
  type QueryResult,
} from "@/features/databases/hooks/use-execute-database-query";
import { isReadOnlySQL } from "@/features/databases/lib/sql";

const RESULT_PAGE_SIZE = 50;
const HISTORY_LIMIT = 20;
export function SQLConsole({ id }: { id: string }) {
  const { t } = useTranslations();
  const { execute, loading } = useExecuteDatabaseQuery(id);
  const [sql, setSQL] = useState("SELECT version();");
  const [result, setResult] = useState<QueryResult | null>(null);
  const [error, setError] = useState("");
  const [history, setHistory] = useState<string[]>(() => readHistory(id));
  const [page, setPage] = useState(0);
  const [pendingWrite, setPendingWrite] = useState<string | null>(null);

  const pageCount = Math.max(
    1,
    Math.ceil((result?.rows.length ?? 0) / RESULT_PAGE_SIZE),
  );
  const visibleRows = useMemo(
    () =>
      result?.rows.slice(
        page * RESULT_PAGE_SIZE,
        (page + 1) * RESULT_PAGE_SIZE,
      ) ?? [],
    [page, result],
  );

  function remember(statement: string) {
    setHistory((current) => {
      const next = [
        statement,
        ...current.filter((item) => item !== statement),
      ].slice(0, HISTORY_LIMIT);
      writeHistory(id, next);
      return next;
    });
  }

  async function run(statement: string, allowWrites: boolean) {
    const trimmed = statement.trim();
    if (!trimmed) return;
    remember(trimmed);
    setError("");
    try {
      setResult(await execute(trimmed, allowWrites));
      setPage(0);
    } catch (cause) {
      setResult(null);
      setError(
        cause instanceof Error && cause.message
          ? cause.message
          : t("databases.sqlUnknownError"),
      );
    }
  }

  function requestRun() {
    if (loading) return;
    const statement = sql.trim();
    if (!statement) return;
    if (isReadOnlySQL(statement)) void run(statement, false);
    else setPendingWrite(statement);
  }

  function clearHistory() {
    setHistory([]);
    writeHistory(id, []);
  }

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <DatabaseZap className="size-4" aria-hidden="true" />
            {t("databases.sqlTitle")}
          </CardTitle>
          <CardDescription>{t("databases.sqlDescription")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <Textarea
            aria-label={t("databases.sqlEditorLabel")}
            className="min-h-36 resize-y font-mono text-sm"
            value={sql}
            onChange={(event) => setSQL(event.target.value)}
            onKeyDown={(event) => {
              if ((event.metaKey || event.ctrlKey) && event.key === "Enter") {
                event.preventDefault();
                requestRun();
              }
            }}
            spellCheck={false}
          />
          <div className="flex flex-wrap items-center justify-between gap-2">
            <p className="text-xs text-muted-foreground">
              {t("databases.sqlShortcut")}
            </p>
            <Button onClick={requestRun} disabled={!sql.trim() || loading}>
              <Play className="size-4" aria-hidden="true" />
              {loading ? t("databases.sqlRunning") : t("databases.sqlRun")}
            </Button>
          </div>

          {error ? (
            <Alert variant="destructive" role="alert">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          ) : null}

          {result ? (
            <div className="space-y-3" aria-live="polite">
              <div className="flex flex-wrap items-center justify-between gap-2 text-sm">
                <p className="text-muted-foreground">
                  {result.columns.length === 0
                    ? t("databases.sqlAffectedRows", { count: result.rowCount })
                    : t("databases.sqlReturnedRows", {
                        count: result.rowCount,
                      })}
                </p>
                {result.truncated ? (
                  <p className="font-medium text-amber-700 dark:text-amber-400">
                    {t("databases.sqlTruncated", { limit: result.rowCount })}
                  </p>
                ) : null}
              </div>

              {result.columns.length > 0 ? (
                <div className="rounded-md border">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        {result.columns.map((column, index) => (
                          <TableHead
                            key={`${column}-${index}`}
                            className="whitespace-nowrap font-mono"
                          >
                            {column}
                          </TableHead>
                        ))}
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {visibleRows.length ? (
                        visibleRows.map((row, rowIndex) => (
                          <TableRow key={page * RESULT_PAGE_SIZE + rowIndex}>
                            {result.columns.map((_, cellIndex) => (
                              <TableCell
                                key={cellIndex}
                                className="max-w-80 font-mono whitespace-pre-wrap"
                              >
                                {row[cellIndex] === null ||
                                row[cellIndex] === undefined
                                  ? t("databases.sqlNull")
                                  : row[cellIndex]}
                              </TableCell>
                            ))}
                          </TableRow>
                        ))
                      ) : (
                        <TableRow>
                          <TableCell
                            colSpan={Math.max(1, result.columns.length)}
                            className="text-center text-muted-foreground"
                          >
                            {t("databases.sqlNoRows")}
                          </TableCell>
                        </TableRow>
                      )}
                    </TableBody>
                  </Table>
                </div>
              ) : null}

              {pageCount > 1 ? (
                <div className="flex items-center justify-end gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setPage((value) => Math.max(0, value - 1))}
                    disabled={page === 0}
                  >
                    {t("databases.sqlPrevious")}
                  </Button>
                  <span className="text-sm tabular-nums text-muted-foreground">
                    {t("databases.sqlPage", {
                      page: page + 1,
                      pages: pageCount,
                    })}
                  </span>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() =>
                      setPage((value) => Math.min(pageCount - 1, value + 1))
                    }
                    disabled={page >= pageCount - 1}
                  >
                    {t("databases.sqlNext")}
                  </Button>
                </div>
              ) : null}
            </div>
          ) : null}

          <div className="space-y-2 border-t pt-4">
            <div className="flex items-center justify-between gap-2">
              <p className="flex items-center gap-2 text-sm font-medium">
                <Clock3 className="size-4" aria-hidden="true" />
                {t("databases.sqlHistory")}
              </p>
              {history.length ? (
                <Button variant="ghost" size="sm" onClick={clearHistory}>
                  <Trash2 className="size-4" aria-hidden="true" />
                  {t("databases.sqlClearHistory")}
                </Button>
              ) : null}
            </div>
            {history.length ? (
              <div className="grid gap-1">
                {history.map((statement) => (
                  <button
                    key={statement}
                    type="button"
                    className="truncate rounded-md px-2 py-1.5 text-left font-mono text-xs text-muted-foreground hover:bg-muted hover:text-foreground"
                    title={statement}
                    onClick={() => setSQL(statement)}
                  >
                    {statement.replace(/\s+/g, " ")}
                  </button>
                ))}
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">
                {t("databases.sqlNoHistory")}
              </p>
            )}
          </div>
        </CardContent>
      </Card>

      <ConfirmDialog
        open={pendingWrite !== null}
        onOpenChange={(open) => !open && setPendingWrite(null)}
        title={t("databases.sqlConfirmTitle")}
        description={t("databases.sqlConfirmDescription")}
        cancelLabel={t("databases.sqlConfirmCancel")}
        confirmLabel={t("databases.sqlConfirmRun")}
        onConfirm={() => {
          const statement = pendingWrite;
          setPendingWrite(null);
          if (statement) void run(statement, true);
        }}
      />
    </>
  );
}

function historyKey(id: string) {
  return `bex.sql-history.${id}`;
}

function readHistory(id: string): string[] {
  if (typeof window === "undefined") return [];
  try {
    const parsed: unknown = JSON.parse(
      window.sessionStorage.getItem(historyKey(id)) ?? "[]",
    );
    return Array.isArray(parsed)
      ? parsed
          .filter((item): item is string => typeof item === "string")
          .slice(0, HISTORY_LIMIT)
      : [];
  } catch {
    return [];
  }
}

function writeHistory(id: string, history: string[]) {
  if (typeof window === "undefined") return;
  try {
    window.sessionStorage.setItem(historyKey(id), JSON.stringify(history));
  } catch {
    // The console remains usable when storage is unavailable or full.
  }
}
