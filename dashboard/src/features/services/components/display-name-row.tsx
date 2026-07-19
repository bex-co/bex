import { useState } from "react";
import { Check, Loader2, Pencil, X } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { useTranslations } from "@/common/hooks/use-translations";
import { useDisplayName } from "@/features/services/hooks/use-display-name";

export interface DisplayNameRowProps {
  serviceId: string;
  /** Raw spec.displayName; empty means the immutable service name is displayed. */
  displayName: string | null | undefined;
  /** The immutable App name — the fallback value when no displayName is set
   *  (Render prefills its Name field with the service name, never the id;
   *  w5/m48/t004). Undefined while the service is still loading. */
  name?: string | null;
  onChanged?: () => void;
}

/**
 * Settings row for a service's mutable human label. The immutable service name
 * is shown as the fallback (the id only when even the name is unknown) and
 * never edited, so a rename cannot change routing or any Kubernetes identity
 * derived from the id.
 */
export function DisplayNameRow({
  serviceId,
  displayName,
  name,
  onChanged,
}: DisplayNameRowProps) {
  const { t } = useTranslations();
  const { setDisplayName, busy } = useDisplayName();
  const current = displayName?.trim() ?? "";
  const visible = current || name?.trim() || serviceId;
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(visible);
  const normalizedDraft = draft.trim();
  const canSave =
    normalizedDraft !== visible && !(normalizedDraft === "" && current === "");

  function startEdit() {
    setDraft(visible);
    setEditing(true);
  }

  async function handleSave() {
    const ok = await setDisplayName(serviceId, normalizedDraft);
    if (ok) {
      setEditing(false);
      onChanged?.();
    }
  }

  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between sm:gap-4">
      <div className="min-w-0">
        <div className="text-sm text-muted-foreground">
          {t("services.displayNameLabel")}
        </div>
        <div className="mt-1 text-sm text-muted-foreground">
          {t("services.displayNameHint", { id: serviceId })}
        </div>
      </div>
      {editing ? (
        <div className="flex w-full items-center gap-2 sm:w-auto">
          <Input
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            aria-label={t("services.displayNameLabel")}
            autoFocus
            autoComplete="off"
            className="min-w-0 flex-1 sm:w-56 sm:flex-none"
            onKeyDown={(event) => {
              if (event.key === "Enter" && canSave) void handleSave();
              if (event.key === "Escape") setEditing(false);
            }}
          />
          <Button
            size="icon"
            variant="ghost"
            aria-label={t("services.displayNameSave")}
            disabled={busy || !canSave}
            onClick={() => void handleSave()}
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
            aria-label={t("services.displayNameCancel")}
            disabled={busy}
            onClick={() => setEditing(false)}
          >
            <X />
          </Button>
        </div>
      ) : (
        <div className="flex min-w-0 items-center gap-2 self-start sm:self-auto">
          <span className="truncate text-sm font-medium">{visible}</span>
          <Button
            size="icon"
            variant="ghost"
            aria-label={t("services.displayNameEdit")}
            onClick={startEdit}
          >
            <Pencil />
          </Button>
        </div>
      )}
    </div>
  );
}
