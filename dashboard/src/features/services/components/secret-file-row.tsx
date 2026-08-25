import { useState } from "react";
import { Eye, EyeOff, Pencil, Trash2, Check, X, Loader2 } from "lucide-react";
import { TableRow, TableCell } from "@/common/components/ui/table";
import { Button } from "@/common/components/ui/button";
import { Textarea } from "@/common/components/ui/textarea";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import type { SecretFileName } from "@/features/services/types";

const MASK = "••••••••••••";

interface SecretFileRowProps {
  entry: SecretFileName;
  /** Fetch this file's content on demand (Render's "Show"). */
  reveal: (name: string) => Promise<string>;
  /** Add/update the content (setSecretFile). */
  onSave: (name: string, content: string) => Promise<boolean>;
  /** Remove the file (deleteSecretFile). */
  onDelete: (name: string) => Promise<boolean>;
  /** A write is in flight somewhere in the table — disable row actions. */
  busy: boolean;
  /** Context-specific consequence shown in the delete confirmation. */
  deleteConfirmBody?: string;
}

/**
 * One Environment-tab secret-file row: the file name, a masked content preview
 * that reveals on demand (`secretFile(name)`), and inline edit / delete. Content
 * is never shown until the user asks — bex-api returns it only per-file, matching
 * the env-vars surface. The body is a free-form file, so edits use a Textarea.
 */
export function SecretFileRow({
  entry,
  reveal,
  onSave,
  onDelete,
  busy,
  deleteConfirmBody,
}: SecretFileRowProps) {
  const { t } = useTranslations();
  const [content, setContent] = useState<string | null>(null); // null = not revealed
  const [revealing, setRevealing] = useState(false);
  const [revealError, setRevealError] = useState(false);
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState("");

  // Fetch this file's content, tracking the loading + error state; null on failure.
  async function loadContent(): Promise<string | null> {
    setRevealing(true);
    setRevealError(false);
    try {
      const v = await reveal(entry.name);
      setContent(v);
      return v;
    } catch {
      setRevealError(true);
      return null;
    } finally {
      setRevealing(false);
    }
  }

  async function toggleReveal() {
    if (content !== null) {
      setContent(null); // hide
      return;
    }
    await loadContent();
  }

  async function startEdit() {
    // Prefill the input with the current content (reveal it if not already shown).
    let current = content;
    if (current === null) {
      current = await loadContent();
      if (current === null) return; // reveal failed; stay in read mode
    }
    setDraft(current);
    setEditing(true);
  }

  async function saveEdit() {
    const ok = await onSave(entry.name, draft);
    if (ok) {
      setContent(draft);
      setEditing(false);
    }
  }

  return (
    <TableRow>
      <TableCell className="align-top font-mono text-sm break-all">
        {entry.name}
      </TableCell>
      <TableCell>
        {editing ? (
          <div className="flex items-start gap-2">
            <Textarea
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              placeholder={t("services.secretFileContentPlaceholder")}
              autoFocus
              className="font-mono text-sm"
              onKeyDown={(e) => {
                if (e.key === "Escape") setEditing(false);
              }}
            />
            <Button
              size="icon"
              variant="ghost"
              aria-label={t("services.envSave")}
              disabled={busy}
              onClick={() => void saveEdit()}
            >
              {busy ? (
                <Loader2 className="animate-spin" />
              ) : (
                <Check className="text-emerald-600" />
              )}
            </Button>
            <Button
              size="icon"
              variant="ghost"
              aria-label={t("services.envCancel")}
              onClick={() => setEditing(false)}
            >
              <X />
            </Button>
          </div>
        ) : (
          <div className="flex items-start gap-2">
            <span className="font-mono text-sm break-all whitespace-pre-wrap text-muted-foreground">
              {revealError
                ? t("services.envRevealError")
                : content !== null
                  ? content === ""
                    ? "—"
                    : content
                  : MASK}
            </span>
            <Button
              size="icon"
              variant="ghost"
              aria-label={
                content !== null
                  ? t("services.envHideSecret")
                  : t("services.envShowSecret")
              }
              disabled={revealing}
              onClick={() => void toggleReveal()}
            >
              {revealing ? (
                <Loader2 className="animate-spin" />
              ) : content !== null ? (
                <EyeOff />
              ) : (
                <Eye />
              )}
            </Button>
          </div>
        )}
      </TableCell>
      <TableCell className="text-right whitespace-nowrap align-top">
        {!editing && (
          <>
            <Button
              size="icon"
              variant="ghost"
              aria-label={t("services.envEdit")}
              disabled={busy}
              onClick={() => void startEdit()}
            >
              <Pencil />
            </Button>
            <ConfirmDialog
              trigger={
                <Button
                  size="icon"
                  variant="ghost"
                  aria-label={t("services.envDelete")}
                  disabled={busy}
                >
                  <Trash2 className="text-destructive" />
                </Button>
              }
              title={t("services.secretFileDeleteConfirmTitle", {
                name: entry.name,
              })}
              description={
                deleteConfirmBody ?? t("services.secretFileDeleteConfirmBody")
              }
              cancelLabel={t("services.envCancel")}
              confirmLabel={t("services.envDelete")}
              onConfirm={() => void onDelete(entry.name)}
            />
          </>
        )}
      </TableCell>
    </TableRow>
  );
}
