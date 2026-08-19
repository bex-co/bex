import { useState } from "react";
import { Eye, EyeOff, Pencil, Trash2, Check, X, Loader2 } from "lucide-react";
import { TableRow, TableCell } from "@/common/components/ui/table";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/common/components/ui/alert-dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import type { EnvVarKey } from "@/features/services/types";

const MASK = "••••••••••••";

interface EnvVarRowProps {
  entry: EnvVarKey;
  /** Fetch this key's value on demand (Render's "Show secret"). */
  reveal: (key: string) => Promise<string>;
  /** Add/update the value (setEnvVar). */
  onSave: (key: string, value: string) => Promise<boolean>;
  /** Remove the variable (deleteEnvVar). */
  onDelete: (key: string) => Promise<boolean>;
  /** A write is in flight somewhere in the table — disable row actions. */
  busy: boolean;
  /** Context-specific consequence shown in the delete confirmation. */
  deleteConfirmBody?: string;
  /** Whether the caller may reveal secret values (can_view_sensitive); when
   *  false the reveal button is disabled with a reason. Defaults true so a
   *  caller that doesn't thread capabilities keeps prior behavior. */
  canReveal?: boolean;
  /** Reason shown on the disabled reveal button when `canReveal` is false. */
  revealReason?: string;
  /** Whether the caller may add/edit/delete (can_create); when false the
   *  Edit pencil and Delete are disabled with a reason. Defaults true so a
   *  caller that doesn't thread capabilities keeps prior behavior. */
  canCreate?: boolean;
  /** Reason shown on the disabled write controls when `canCreate` is false. */
  createReason?: string;
}

/**
 * One Environment-tab row: the key, a masked value that reveals on demand
 * (`envVar(key)`), and inline edit / delete. Values are never shown until the
 * user asks — bex-api returns them only per-key, matching Render's dashboard.
 */
export function EnvVarRow({
  entry,
  reveal,
  onSave,
  onDelete,
  busy,
  deleteConfirmBody,
  canReveal = true,
  revealReason,
  canCreate = true,
  createReason,
}: EnvVarRowProps) {
  const { t } = useTranslations();
  const [value, setValue] = useState<string | null>(null); // null = not revealed
  const [revealing, setRevealing] = useState(false);
  const [revealError, setRevealError] = useState(false);
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState("");

  // Fetch this key's value, tracking the loading + error state; null on failure.
  async function loadValue(): Promise<string | null> {
    setRevealing(true);
    setRevealError(false);
    try {
      const v = await reveal(entry.key);
      setValue(v);
      return v;
    } catch {
      setRevealError(true);
      return null;
    } finally {
      setRevealing(false);
    }
  }

  async function toggleReveal() {
    if (value !== null) {
      setValue(null); // hide
      return;
    }
    await loadValue();
  }

  async function startEdit() {
    // Prefill the input with the current value (reveal it if not already shown).
    let current = value;
    if (current === null) {
      current = await loadValue();
      if (current === null) return; // reveal failed; stay in read mode
    }
    setDraft(current);
    setEditing(true);
  }

  async function saveEdit() {
    const ok = await onSave(entry.key, draft);
    if (ok) {
      setValue(draft);
      setEditing(false);
    }
  }

  return (
    <TableRow>
      <TableCell className="align-top font-mono text-sm break-all">
        {entry.key}
      </TableCell>
      <TableCell>
        {editing ? (
          <div className="flex items-center gap-2">
            <Input
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              placeholder={t("services.envValuePlaceholder")}
              autoFocus
              className="font-mono text-sm"
              onKeyDown={(e) => {
                if (e.key === "Enter") void saveEdit();
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
          <div className="flex items-center gap-2">
            <span className="font-mono text-sm break-all text-muted-foreground">
              {revealError
                ? t("services.envRevealError")
                : value !== null
                  ? value === ""
                    ? "—"
                    : value
                  : MASK}
            </span>
            <PermissionTooltip
              reason={value === null && !canReveal ? revealReason : undefined}
            >
              <Button
                size="icon"
                variant="ghost"
                aria-label={
                  value !== null
                    ? t("services.envHideSecret")
                    : t("services.envShowSecret")
                }
                // Hiding an already-revealed value never needs permission; only
                // revealing does (can_view_sensitive).
                disabled={revealing || (value === null && !canReveal)}
                onClick={() => void toggleReveal()}
              >
                {revealing ? (
                  <Loader2 className="animate-spin" />
                ) : value !== null ? (
                  <EyeOff />
                ) : (
                  <Eye />
                )}
              </Button>
            </PermissionTooltip>
          </div>
        )}
      </TableCell>
      <TableCell className="text-right whitespace-nowrap">
        {!editing && (
          <>
            <PermissionTooltip reason={createReason}>
              <Button
                size="icon"
                variant="ghost"
                aria-label={t("services.envEdit")}
                disabled={busy || !canCreate}
                onClick={() => void startEdit()}
              >
                <Pencil />
              </Button>
            </PermissionTooltip>
            <AlertDialog>
              <PermissionTooltip reason={createReason}>
                <AlertDialogTrigger asChild>
                  <Button
                    size="icon"
                    variant="ghost"
                    aria-label={t("services.envDelete")}
                    disabled={busy || !canCreate}
                  >
                    <Trash2 className="text-destructive" />
                  </Button>
                </AlertDialogTrigger>
              </PermissionTooltip>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>
                    {t("services.envDeleteConfirmTitle", { key: entry.key })}
                  </AlertDialogTitle>
                  <AlertDialogDescription>
                    {deleteConfirmBody ?? t("services.envDeleteConfirmBody")}
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>
                    {t("services.envCancel")}
                  </AlertDialogCancel>
                  <AlertDialogAction onClick={() => void onDelete(entry.key)}>
                    {t("services.envDelete")}
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </>
        )}
      </TableCell>
    </TableRow>
  );
}
