import { useState } from "react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import { useRuntime } from "@/features/services/hooks/use-runtime";
import { isDockerBuild } from "@/features/services/lib/service-type";
import { RUNTIME_DEFS, type GitRuntime } from "@/features/services/lib/runtime";

// Render's Settings lists runtimes alphabetically, Docker first.
const SETTINGS_RUNTIMES: GitRuntime[] = [
  "docker",
  "elixir",
  "go",
  "node",
  "python",
  "ruby",
  "rust",
];

const LABEL_KEYS = Object.fromEntries(
  RUNTIME_DEFS.map((def) => [def.id, def.labelKey]),
) as Record<GitRuntime, string>;

/**
 * The saved runtime as a picker value: a legacy App created before runtime
 * existed reads as docker when it builds from a Dockerfile, otherwise as no
 * selection.
 */
function currentRuntime(
  runtime: string | null | undefined,
  builder: string | null | undefined,
): GitRuntime | null {
  const known = SETTINGS_RUNTIMES.find((r) => r === runtime);
  if (known) return known;
  return isDockerBuild({ runtime, builder }) ? "docker" : null;
}

export interface RuntimeSectionProps {
  serviceId: string;
  /** The saved runtime (docker or a native language; empty for legacy). */
  runtime: string | null | undefined;
  /** Legacy build strategy, read when runtime is empty. */
  builder: string | null | undefined;
}

/**
 * Settings section for Render's Runtime: Docker or a native runtime, saved
 * after a confirmation because bex-api's `setRuntime` rebuilds and redeploys
 * the service. Repo-backed, non-static services only; the Settings page gates
 * rendering (an image-backed service or a static site has no runtime to pick).
 */
export function RuntimeSection({
  serviceId,
  runtime,
  builder,
}: RuntimeSectionProps) {
  const { t } = useTranslations();
  const { setRuntime, saving } = useRuntime();
  const saved = currentRuntime(runtime, builder);
  // null follows the saved runtime, so a save (or a refetch) re-syncs it.
  const [draft, setDraft] = useState<GitRuntime | null>(null);
  const selected = draft ?? saved;
  const dirty = selected !== null && selected !== saved;
  const [confirming, setConfirming] = useState(false);
  const selectedLabel = selected ? t(LABEL_KEYS[selected]) : "";

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("services.runtimeTitle")}</CardTitle>
        <CardDescription>{t("services.runtimeDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between sm:gap-4">
          <div className="min-w-0">
            <div className="text-sm text-muted-foreground">
              {t("services.runtimeLabel")}
            </div>
            <div className="mt-1 text-sm text-muted-foreground">
              {t("services.runtimeNote")}
            </div>
          </div>
          <Select
            value={selected ?? ""}
            disabled={saving}
            onValueChange={(v) => setDraft(v as GitRuntime)}
          >
            <SelectTrigger
              aria-label={t("services.runtimeLabel")}
              size="sm"
              className="w-full sm:w-40"
            >
              <SelectValue placeholder={t("services.runtimePlaceholder")} />
            </SelectTrigger>
            <SelectContent>
              {SETTINGS_RUNTIMES.map((r) => (
                <SelectItem key={r} value={r}>
                  {t(LABEL_KEYS[r])}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Button
            size="sm"
            disabled={!dirty || saving}
            onClick={() => setConfirming(true)}
          >
            {saving ? t("services.runtimeSaving") : t("services.runtimeSave")}
          </Button>
        </div>
      </CardContent>

      <ConfirmDialog
        open={confirming}
        onOpenChange={setConfirming}
        title={t("services.runtimeConfirmTitle", { runtime: selectedLabel })}
        description={t("services.runtimeConfirmBody", {
          runtime: selectedLabel,
        })}
        cancelLabel={t("services.confirmCancel")}
        confirmLabel={t("services.runtimeConfirmAction")}
        destructive={false}
        pending={saving}
        onConfirm={() => {
          setConfirming(false);
          if (!selected) return;
          void setRuntime(serviceId, selected).then((ok) => {
            if (ok) setDraft(null);
          });
        }}
      />
    </Card>
  );
}
