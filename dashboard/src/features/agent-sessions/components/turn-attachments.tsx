import { useState } from "react";
import { useQuery } from "@apollo/client/react";
import { Download, FileText, Loader2 } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { formatBytes } from "@/common/lib/format";
import { cn } from "@/common/lib/utils/utils";
import { useFreshDownload } from "@/features/agent-sessions/hooks/use-fresh-download";
import { useTranslations } from "@/common/hooks/use-translations";
import { AgentAttachmentDocument } from "@/graphql/definitions";
import {
  isInlineImage,
  type AttachmentView,
} from "@/features/agent-sessions/lib/attachments";

/**
 * A user turn's attachments (ADR095 D10). Raster images show as thumbnails
 * that open a lightbox; every other type — SVG, HTML and PDF included — is a
 * download card, so no uploaded markup ever renders in the dashboard. Bytes
 * come from a presigned URL on the storage origin (D7).
 */
export function TurnAttachments({
  items,
  align = "end",
}: {
  items: AttachmentView[];
  /** "end" under the user's bubble; "start" for files the agent shared. */
  align?: "start" | "end";
}) {
  if (items.length === 0) return null;
  return (
    <ul
      className={cn(
        "mt-1.5 flex flex-wrap gap-1.5",
        align === "end" ? "justify-end" : "justify-start",
      )}
      data-testid="turn-attachments"
    >
      {items.map((item) => (
        <li key={item.id} className="min-w-0">
          {isInlineImage(item.mediaType) ? (
            <ImageThumbnail item={item} />
          ) : (
            <FileCard item={item} />
          )}
        </li>
      ))}
    </ul>
  );
}

function ImageThumbnail({ item }: { item: AttachmentView }) {
  const { t } = useTranslations();
  const [open, setOpen] = useState(false);
  // A presigned URL lives 15 minutes. A load failure refetches once for a
  // renewed link; a second failure on that same link means the file is gone.
  // `retriedUrl` names the link already renewed from, so every later
  // expiry of a fresh link gets its own renewal.
  const [retriedUrl, setRetriedUrl] = useState<string | null>(null);
  const [failed, setFailed] = useState(false);
  const { data, loading, refetch } = useQuery(AgentAttachmentDocument, {
    variables: { id: item.id },
    fetchPolicy: "cache-first",
    errorPolicy: "all",
  });
  const url = data?.agentAttachment?.downloadUrl;

  function onError() {
    if (!url || retriedUrl === url) {
      setFailed(true);
      return;
    }
    setRetriedUrl(url);
    void refetch();
  }

  // A query that settled without a link (an error, a deleted file) shows the
  // card rather than spinning forever.
  if (failed || (!loading && !url)) return <FileCard item={item} unavailable />;
  return (
    <>
      <button
        type="button"
        className="bg-muted block size-20 overflow-hidden rounded-md border"
        aria-label={t("agentSessions.attachmentOpen", { name: item.name })}
        title={item.name}
        onClick={() => setOpen(true)}
        disabled={!url}
      >
        {url ? (
          <img
            src={url}
            alt={item.name}
            loading="lazy"
            className="size-full object-cover"
            onError={onError}
          />
        ) : (
          <Loader2 className="text-muted-foreground m-auto size-4 animate-spin" />
        )}
      </button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-w-[min(90vw,64rem)] p-2 sm:max-w-[min(90vw,64rem)]">
          <DialogTitle className="truncate px-2 pt-1 text-sm">
            {item.name}
          </DialogTitle>
          <DialogDescription className="sr-only">
            {formatBytes(item.size)}
          </DialogDescription>
          {url ? (
            <img
              src={url}
              alt={item.name}
              className="max-h-[80vh] w-full rounded object-contain"
              onError={onError}
            />
          ) : null}
        </DialogContent>
      </Dialog>
    </>
  );
}

function FileCard({
  item,
  unavailable = false,
}: {
  item: AttachmentView;
  unavailable?: boolean;
}) {
  const { t } = useTranslations();
  const { download, busy, failed } = useFreshDownload(item.id, unavailable);

  return (
    <button
      type="button"
      onClick={() => void download()}
      disabled={busy}
      aria-label={t("agentSessions.attachmentDownload", { name: item.name })}
      className="bg-background hover:bg-muted flex max-w-56 min-w-0 items-center gap-2 rounded-md border px-2 py-1.5 text-left text-xs"
    >
      <FileText className="text-muted-foreground size-4 shrink-0" />
      <span className="min-w-0">
        <span className="block truncate" title={item.name}>
          {item.name}
        </span>
        <span className="text-muted-foreground block truncate">
          {failed
            ? t("agentSessions.attachmentUnavailable")
            : formatBytes(item.size)}
        </span>
      </span>
      {busy ? (
        <Loader2 className="text-muted-foreground size-3.5 shrink-0 animate-spin" />
      ) : (
        <Download className="text-muted-foreground size-3.5 shrink-0" />
      )}
    </button>
  );
}
