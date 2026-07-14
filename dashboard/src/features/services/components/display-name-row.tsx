import { useState } from "react";
import { Check, Loader2, Pencil, X } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { useTranslations } from "@/common/hooks/use-translations";
import { useDisplayName } from "@/features/services/hooks/use-display-name";

export interface DisplayNameRowProps {
  serviceId: string;
  /** Raw spec.displayName; empty means the immutable serviceId is displayed. */
  displayName: string | null | undefined;
  onChanged?: () => void;
}

/**
 * Settings row for a service's mutable human label. The immutable service id is
 * shown as the fallback and never edited, so a rename cannot change routing or
 * any Kubernetes identity derived from the id.
 */
export function DisplayNameRow({
  serviceId,
  displayName,
  onChanged,
}: DisplayNameRowProps) {
  const { t } = useTranslations();
  const { setDisplayName, busy } = useDisplayName();
  const current = displayName?.trim() ?? "";
  const visible = current || serviceId;
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
    <div className="flex items-center justify-between gap-4">
      <div>
        <div className="text-sm text-muted-foreground">
          {t("services.displayNameLabel")}
        </div>
        <div className="mt-1 text-sm text-muted-foreground">
          {t("services.displayNameHint", { id: serviceId })}
        </div>
      </div>
      {editing ? (
        <div className="flex items-center gap-2">
          <Input
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            aria-label={t("services.displayNameLabel")}
            autoFocus
            autoComplete="off"
            className="w-56"
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
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium">{visible}</span>
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
