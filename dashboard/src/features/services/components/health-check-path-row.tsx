import { useState } from "react";
import { Pencil, Check, X, Loader2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { useTranslations } from "@/common/hooks/use-translations";
import { useHealthCheckPath } from "@/features/services/hooks/use-health-check-path";

export interface HealthCheckPathRowProps {
  serviceId: string;
  /** Current spec.healthCheckPath; null/empty means the platform default "/". */
  healthCheckPath: string | null | undefined;
}

/**
 * Settings row for the ReadinessProbe HTTP path (w1/m23/t001) — the path the
 * platform polls before routing traffic to the service. Uses the same
 * pencil → input → confirm inline-edit pattern as the Root Directory row.
 * Only shown for web_service and private_service; the settings page gates it.
 */
export function HealthCheckPathRow({
  serviceId,
  healthCheckPath,
}: HealthCheckPathRowProps) {
  const { t } = useTranslations();
  const { setHealthCheckPath, busy } = useHealthCheckPath();
  const current = healthCheckPath ?? "";
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState("");

  const canSave = draft !== current;

  function startEdit() {
    setDraft(current);
    setEditing(true);
  }

  async function handleSave() {
    const ok = await setHealthCheckPath(serviceId, draft || "/");
    if (ok) setEditing(false);
  }

  return (
    <div className="flex items-center justify-between gap-4">
      <div>
        <div className="text-sm text-muted-foreground">
          {t("services.settingsHealthCheckPath")}
        </div>
        <div className="mt-1 text-sm text-muted-foreground">
          {t("services.settingsHealthCheckPathHint")}
        </div>
      </div>
      {editing ? (
        <div className="flex items-center gap-2">
          <Input
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            placeholder={t("services.settingsHealthCheckPathPlaceholder")}
            autoFocus
            className="w-48 font-mono text-sm"
            onKeyDown={(e) => {
              if (e.key === "Enter" && canSave) void handleSave();
              if (e.key === "Escape") setEditing(false);
            }}
          />
          <Button
            size="icon"
            variant="ghost"
            aria-label={t("services.buildDeploySave")}
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
            aria-label={t("services.buildDeployCancel")}
            disabled={busy}
            onClick={() => setEditing(false)}
          >
            <X />
          </Button>
        </div>
      ) : (
        <div className="flex items-center gap-2">
          <span className="font-mono text-sm">
            {current || t("services.settingsHealthCheckPathPlaceholder")}
          </span>
          <Button
            size="icon"
            variant="ghost"
            aria-label={t("services.settingsHealthCheckPathEdit")}
            onClick={startEdit}
          >
            <Pencil />
          </Button>
        </div>
      )}
    </div>
  );
}
