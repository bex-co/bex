import { useState } from "react";
import { useQuery } from "@apollo/client/react";
import { Download, Files, Loader2 } from "lucide-react";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/common/components/ui/sheet";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
import { useTranslations } from "@/common/hooks/use-translations";
import { formatBytes } from "@/common/lib/format";
import { useFreshDownload } from "@/features/agent-sessions/hooks/use-fresh-download";
import { AgentSessionAttachmentsDocument } from "@/graphql/definitions";

/** One page of the Files sheet; the API caps a page at 100. */
export const FILES_PAGE_SIZE = 25;
/**
 * Rows the pending state previews. Not a page's worth: enough to fill the
 * sheet's first screen, with the same header and row height as the table.
 */
const SKELETON_ROWS = 6;

/**
 * The session's Files sheet (ADR095 D10): every file the user attached and
 * every file the agent shared, oldest first, each downloadable through a fresh
 * presigned URL. Opened from the session header — the detail page is one
 * full-page conversation, so the list lives in a sheet rather than a tab.
 */
export function SessionFilesButton({ sessionId }: { sessionId: string }) {
  const { t } = useTranslations();
  const [open, setOpen] = useState(false);
  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger asChild>
        <Button
          variant="outline"
          size="sm"
          className="shrink-0 gap-1.5"
          aria-label={t("agentSessions.filesButton")}
        >
          <Files className="size-4" />
          <span className="hidden sm:inline">
            {t("agentSessions.filesButton")}
          </span>
        </Button>
      </SheetTrigger>
      <SheetContent side="right" className="w-full gap-0 sm:max-w-xl">
        <SheetHeader>
          <SheetTitle>{t("agentSessions.filesTitle")}</SheetTitle>
          <SheetDescription>
            {t("agentSessions.filesDescription")}
          </SheetDescription>
        </SheetHeader>
        <div className="min-h-0 flex-1 overflow-y-auto px-4 pb-4">
          {open ? <SessionFilesTable sessionId={sessionId} /> : null}
        </div>
      </SheetContent>
    </Sheet>
  );
}

export function SessionFilesTable({ sessionId }: { sessionId: string }) {
  const { t } = useTranslations();
  const { data, loading, error, fetchMore } = useQuery(
    AgentSessionAttachmentsDocument,
    {
      variables: { id: sessionId, limit: FILES_PAGE_SIZE },
      fetchPolicy: "cache-and-network",
      notifyOnNetworkStatusChange: true,
    },
  );
  const [more, setMore] = useState<"idle" | "loading" | "failed" | "done">(
    "idle",
  );
  const rows = data?.agentSessionAttachments ?? [];

  if (!data && loading) return <FilesTableShell skeleton />;
  if (!data && error) {
    return (
      <p className="text-destructive py-6 text-sm" role="alert">
        {t("agentSessions.filesError")}
      </p>
    );
  }
  if (rows.length === 0) {
    return (
      <p className="text-muted-foreground py-6 text-sm">
        {t("agentSessions.filesEmpty")}
      </p>
    );
  }

  // A short page is the last one; otherwise another may follow.
  const mayHaveMore = more !== "done" && rows.length % FILES_PAGE_SIZE === 0;
  async function loadMore() {
    setMore("loading");
    let next: typeof more = "failed";
    try {
      const result = await fetchMore({
        variables: { cursor: rows[rows.length - 1]?.id },
        updateQuery: (previous, { fetchMoreResult }) => ({
          agentSessionAttachments: [
            ...previous.agentSessionAttachments,
            ...fetchMoreResult.agentSessionAttachments,
          ],
        }),
      });
      const page = result.data?.agentSessionAttachments ?? [];
      next = page.length < FILES_PAGE_SIZE ? "done" : "idle";
    } finally {
      setMore(next);
    }
  }

  return (
    <>
      <FilesTableShell>
        {rows.map((row) => (
          <TableRow key={row.id} className="h-11">
            <TableCell className="w-full max-w-0">
              <DownloadName id={row.id} name={row.name} />
            </TableCell>
            <TableCell>
              <Badge
                variant={row.direction === "output" ? "default" : "secondary"}
              >
                {row.direction === "output"
                  ? t("agentSessions.filesFromOutput")
                  : t("agentSessions.filesFromInput")}
              </Badge>
            </TableCell>
            <TableCell className="text-muted-foreground tabular-nums">
              {row.turn ?? "—"}
            </TableCell>
            <TableCell className="text-muted-foreground text-right whitespace-nowrap tabular-nums">
              {formatBytes(row.size)}
            </TableCell>
          </TableRow>
        ))}
      </FilesTableShell>
      {mayHaveMore ? (
        <div className="mt-3 flex flex-col items-center gap-1">
          <Button
            variant="outline"
            size="sm"
            disabled={more === "loading"}
            onClick={() => void loadMore().catch(() => undefined)}
          >
            {more === "loading" ? (
              <Loader2 className="size-4 animate-spin" />
            ) : null}
            {t("agentSessions.filesLoadMore")}
          </Button>
          {more === "failed" ? (
            <p className="text-destructive text-xs" role="alert">
              {t("agentSessions.filesError")}
            </p>
          ) : null}
        </div>
      ) : null}
    </>
  );
}

/**
 * The table frame both states share — the header row and fixed-height body
 * rows — so the pending skeleton has exactly the ready table's geometry at
 * every width.
 */
function FilesTableShell({
  skeleton = false,
  children,
}: {
  skeleton?: boolean;
  children?: React.ReactNode;
}) {
  const { t } = useTranslations();
  return (
    <Table
      data-testid={skeleton ? "session-files-skeleton" : "session-files-table"}
    >
      <TableHeader>
        <TableRow>
          <TableHead className="w-full">
            {t("agentSessions.filesColumnName")}
          </TableHead>
          <TableHead>{t("agentSessions.filesColumnFrom")}</TableHead>
          <TableHead>{t("agentSessions.filesColumnTurn")}</TableHead>
          <TableHead className="text-right">
            {t("agentSessions.filesColumnSize")}
          </TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {skeleton
          ? Array.from({ length: SKELETON_ROWS }, (_, i) => (
              <TableRow key={i} className="h-11">
                <TableCell className="w-full max-w-0">
                  <Skeleton className="h-4 w-3/4" />
                </TableCell>
                <TableCell>
                  <Skeleton className="h-5 w-12 rounded-full" />
                </TableCell>
                <TableCell>
                  <Skeleton className="h-4 w-4" />
                </TableCell>
                <TableCell>
                  <Skeleton className="ml-auto h-4 w-12" />
                </TableCell>
              </TableRow>
            ))
          : children}
      </TableBody>
    </Table>
  );
}

/** A file name that downloads through a freshly signed URL on click. */
function DownloadName({ id, name }: { id: string; name: string }) {
  const { t } = useTranslations();
  const { download, busy, failed } = useFreshDownload(id);
  return (
    <button
      type="button"
      onClick={() => void download()}
      disabled={busy}
      className="hover:text-foreground flex w-full min-w-0 items-center gap-1.5 text-left"
      title={name}
    >
      {busy ? (
        <Loader2 className="text-muted-foreground size-3.5 shrink-0 animate-spin" />
      ) : (
        <Download className="text-muted-foreground size-3.5 shrink-0" />
      )}
      <span className="truncate">{name}</span>
      {failed ? (
        <span className="text-destructive shrink-0 text-xs">
          {t("agentSessions.attachmentUnavailable")}
        </span>
      ) : null}
    </button>
  );
}
