import { useState } from "react";
import { Check, Loader2, Pencil, X } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { useTranslations } from "@/common/hooks/use-translations";
import { useMaxShutdownDelay } from "@/features/services/hooks/use-max-shutdown-delay";

export interface MaxShutdownDelayRowProps {
  serviceId: string;
  /** Effective value from bex-api; null on legacy responses defaults to 30. */
  maxShutdownDelaySeconds: number | null | undefined;
  onChanged?: () => void;
}

/** Render-style pencil → numeric input → confirm row for graceful shutdown. */
export function MaxShutdownDelayRow({
  serviceId,
  maxShutdownDelaySeconds,
  onChanged,
}: MaxShutdownDelayRowProps) {
  const { t } = useTranslations();
  const { setMaxShutdownDelay, busy } = useMaxShutdownDelay();
  const current = maxShutdownDelaySeconds ?? 30;
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(String(current));
  const parsed = Number(draft);
  const valid = Number.isInteger(parsed) && parsed >= 1 && parsed <= 300;
  const canSave = valid && parsed !== current;

  function startEdit() {
    setDraft(String(current));
    setEditing(true);
  }

  async function handleSave() {
    if (!canSave) return;
    const ok = await setMaxShutdownDelay(serviceId, parsed);
    if (ok) {
      setEditing(false);
      onChanged?.();
    }
  }

  return (
    <div className="flex items-center justify-between gap-4">
      <div>
        <div className="text-sm text-muted-foreground">
          {t("services.settingsMaxShutdownDelay")}
        </div>
        <div className="mt-1 text-sm text-muted-foreground">
          {t("services.settingsMaxShutdownDelayHint")}
        </div>
      </div>
      {editing ? (
        <div className="flex items-center gap-2">
          <Input
            type="number"
            min={1}
            max={300}
            step={1}
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            aria-label={t("services.settingsMaxShutdownDelay")}
            autoFocus
            className="w-28 font-mono text-sm"
            onKeyDown={(event) => {
              if (event.key === "Enter" && canSave) void handleSave();
              if (event.key === "Escape") setEditing(false);
            }}
          />
          <Button
            size="icon"
            variant="ghost"
            aria-label={t("services.maxShutdownDelaySave")}
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
            aria-label={t("services.maxShutdownDelayCancel")}
            disabled={busy}
            onClick={() => setEditing(false)}
          >
            <X />
          </Button>
        </div>
      ) : (
        <div className="flex items-center gap-2">
          <span className="font-mono text-sm">
            {t("services.maxShutdownDelaySeconds", { seconds: current })}
          </span>
          <Button
            size="icon"
            variant="ghost"
            aria-label={t("services.maxShutdownDelayEdit")}
            onClick={startEdit}
          >
            <Pencil />
          </Button>
        </div>
      )}
    </div>
  );
}
