import { useRef } from "react";
import { FileText, ImageIcon, Loader2, Paperclip, X } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { cn } from "@/common/lib/utils/utils";
import { formatBytes } from "@/common/lib/format";
import { useTranslations } from "@/common/hooks/use-translations";
import type {
  AttachmentChip,
  UseAttachmentUploadsResult,
} from "@/features/agent-sessions/hooks/use-attachment-uploads";
import type { AttachmentLimitsView } from "@/features/agent-sessions/types";

/** The paperclip: a button over a hidden multi-file input. */
export function AttachButton({
  onFiles,
  disabled,
}: {
  onFiles: (files: File[]) => void;
  disabled?: boolean;
}) {
  const { t } = useTranslations();
  const input = useRef<HTMLInputElement | null>(null);
  return (
    <>
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="h-8 w-8 shrink-0 p-0"
        aria-label={t("agentSessions.attachFiles")}
        title={t("agentSessions.attachFiles")}
        disabled={disabled}
        onClick={() => input.current?.click()}
      >
        <Paperclip className="size-4" />
      </Button>
      <input
        ref={input}
        type="file"
        multiple
        hidden
        data-testid="attachment-input"
        onChange={(e) => {
          onFiles(Array.from(e.target.files ?? []));
          // Picking the same file again must fire change again.
          e.target.value = "";
        }}
      />
    </>
  );
}

/** Chips above the input: name, size, progress, the server's error, remove. */
export function AttachmentChips({
  uploads,
  limits,
}: {
  uploads: UseAttachmentUploadsResult;
  limits: AttachmentLimitsView | null;
}) {
  if (uploads.chips.length === 0) return null;
  return (
    <ul className="flex flex-wrap gap-1.5 px-2 pt-2" aria-live="polite">
      {uploads.chips.map((chip) => (
        <Chip
          key={chip.key}
          chip={chip}
          limits={limits}
          onRemove={() => uploads.remove(chip.key)}
        />
      ))}
    </ul>
  );
}

function Chip({
  chip,
  limits,
  onRemove,
}: {
  chip: AttachmentChip;
  limits: AttachmentLimitsView | null;
  onRemove: () => void;
}) {
  const { t } = useTranslations();
  const Icon = chip.mediaType.startsWith("image/") ? ImageIcon : FileText;
  const error = chipErrorText(chip, limits, t);
  const status =
    error ??
    (chip.status === "uploading"
      ? t("agentSessions.attachmentUploading", { percent: chip.progress })
      : t("agentSessions.attachmentReady"));
  return (
    <li
      data-testid="attachment-chip"
      data-status={chip.status}
      className={cn(
        "bg-muted/60 relative flex max-w-64 min-w-0 items-center gap-1.5 overflow-hidden rounded-md border py-1 pr-1 pl-2 text-xs",
        chip.status === "error" && "border-destructive/60 bg-destructive/5",
      )}
    >
      {chip.status === "uploading" ? (
        <Loader2 className="text-muted-foreground size-3.5 shrink-0 animate-spin" />
      ) : (
        <Icon className="text-muted-foreground size-3.5 shrink-0" />
      )}
      <span className="min-w-0">
        <span className="block truncate" title={chip.name}>
          {chip.name}
        </span>
        <span
          className={cn(
            "block truncate",
            error ? "text-destructive" : "text-muted-foreground",
          )}
          title={error ?? undefined}
        >
          {error ?? formatBytes(chip.size)}
        </span>
        {/* Announced by the list's aria-live region: upload progress and
            the outcome, which the visual bar and icon alone don't convey. */}
        <span className="sr-only">{status}</span>
      </span>
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="h-6 w-6 shrink-0 p-0"
        aria-label={t("agentSessions.attachmentRemove", { name: chip.name })}
        onClick={onRemove}
      >
        <X className="size-3.5" />
      </Button>
      {chip.status === "uploading" ? (
        <span
          aria-hidden
          className="bg-primary absolute bottom-0 left-0 h-0.5 transition-[width]"
          style={{ width: `${chip.progress}%` }}
        />
      ) : null}
    </li>
  );
}

/** The chip's error in the user's language; the server's words verbatim. */
function chipErrorText(
  chip: AttachmentChip,
  limits: AttachmentLimitsView | null,
  t: ReturnType<typeof useTranslations>["t"],
): string | null {
  const error = chip.error;
  if (!error) return null;
  switch (error.kind) {
    case "server":
      return error.message;
    case "http":
      return t("agentSessions.attachmentHttpError", { status: error.status });
    case "network":
      return t("agentSessions.attachmentNetworkError");
    case "tooLarge":
      return t("agentSessions.attachmentTooLarge", {
        max: formatBytes(limits?.maxBytes ?? 0),
      });
    case "tooMany":
      return t("agentSessions.attachmentTooMany", {
        max: limits?.maxPerTurn ?? 0,
      });
  }
}

/** Covers the composer while files are dragged over it. */
export function DropOverlay({ visible }: { visible: boolean }) {
  const { t } = useTranslations();
  if (!visible) return null;
  return (
    <div className="bg-background/85 text-muted-foreground pointer-events-none absolute inset-0 z-10 flex items-center justify-center rounded-xl text-sm">
      {t("agentSessions.attachmentDropHint")}
    </div>
  );
}
