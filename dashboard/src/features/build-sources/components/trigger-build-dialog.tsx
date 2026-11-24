import { useMemo, useState } from "react";
import { Link } from "@tanstack/react-router";
import {
  AlertTriangle,
  CheckCircle2,
  ExternalLink,
  Loader2,
  Play,
} from "lucide-react";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import { Checkbox } from "@/common/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { Label } from "@/common/components/ui/label";
import { useTranslations } from "@/common/hooks/use-translations";
import { useTriggerBuild } from "@/features/build-sources/hooks/use-trigger-build";
import { skippedReasonLabel } from "@/features/build-sources/lib/build-status";
import type {
  BuildSourceBuildView,
  BuildSourceView,
} from "@/features/build-sources/types";

export type TriggerBuildServiceScope = "autodeploy_only" | "none" | "custom";

export interface TriggerBuildDialogProps {
  buildSource: BuildSourceView;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/**
 * Render round-9 scoped Trigger Build dialog (`w2/m614/t002`):
 * - `deploy.serviceScope`: `autodeploy_only` (default), `none`, or `custom`
 *   with linked-service checkboxes (requiring >= 1 selected; `serviceIds` is
 *   omitted unless `serviceScope === "custom"`).
 * - `clearCache`: `do_not_clear` vs `clear`.
 * - Confirmation view surfacing the new `bld-…`, clickable links to queued
 *   `deploys`, and explicit notices for any `skippedServices`.
 */
export function TriggerBuildDialog({
  buildSource,
  open,
  onOpenChange,
}: TriggerBuildDialogProps) {
  const { t } = useTranslations();
  const { trigger, busy } = useTriggerBuild();

  const [scope, setScope] = useState<TriggerBuildServiceScope>("autodeploy_only");
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [clearCache, setClearCache] = useState(false);
  const [result, setResult] = useState<BuildSourceBuildView | null>(null);

  const serviceNameById = useMemo(
    () => new Map(buildSource.serviceLinks.map((s) => [s.id, s.name])),
    [buildSource.serviceLinks],
  );

  function resetState() {
    setScope("autodeploy_only");
    setSelectedIds([]);
    setClearCache(false);
    setResult(null);
  }

  function handleOpenChange(next: boolean) {
    if (!next) resetState();
    onOpenChange(next);
  }

  function toggleService(id: string) {
    setSelectedIds((prev) =>
      prev.includes(id) ? prev.filter((item) => item !== id) : [...prev, id],
    );
  }

  const customRequiresSelection =
    scope === "custom" && selectedIds.length === 0;
  const canSubmit = !busy && !customRequiresSelection;

  async function handleSubmit() {
    if (!canSubmit) return;
    const created = await trigger({
      buildSourceId: buildSource.id,
      clearCache: clearCache ? "clear" : "do_not_clear",
      deploy:
        scope === "custom"
          ? { serviceScope: "custom", serviceIds: selectedIds }
          : { serviceScope: scope },
    });
    if (created) {
      setResult(created);
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-lg">
        {result ? (
          <>
            <DialogHeader>
              <DialogTitle className="flex items-center gap-2">
                <CheckCircle2
                  className="text-primary size-5 shrink-0"
                  aria-hidden="true"
                />
                {t("buildSources.triggerResultTitle")}
              </DialogTitle>
              <DialogDescription>
                {t("buildSources.triggerResultDescription", {
                  id: result.id,
                })}
              </DialogDescription>
            </DialogHeader>

            <div className="space-y-4 py-2">
              {result.deploys.length > 0 ? (
                <div className="space-y-2" data-testid="trigger-result-deploys">
                  <p className="text-sm font-medium">
                    {t("buildSources.triggerResultDeploysTitle", {
                      count: result.deploys.length,
                    })}
                  </p>
                  <ul className="divide-y rounded-md border">
                    {result.deploys.map((dep) => (
                      <li
                        key={`${dep.serviceId}:${dep.deployId}`}
                        className="flex items-center justify-between gap-2 px-3 py-2 text-sm"
                      >
                        <span className="truncate font-medium">
                          {serviceNameById.get(dep.serviceId) ?? dep.serviceId}
                        </span>
                        <Link
                          to="/services/$serviceId/deploys/$deployId"
                          params={{
                            serviceId: dep.serviceId,
                            deployId: dep.deployId,
                          }}
                          onClick={() => handleOpenChange(false)}
                          className="text-primary inline-flex shrink-0 items-center gap-1 font-mono text-xs hover:underline"
                        >
                          {dep.deployId}
                          <ExternalLink className="size-3" aria-hidden="true" />
                        </Link>
                      </li>
                    ))}
                  </ul>
                </div>
              ) : (
                <p className="text-muted-foreground text-sm">
                  {t("buildSources.triggerResultNoDeploys")}
                </p>
              )}

              {result.skippedServices.length > 0 ? (
                <Alert
                  variant="destructive"
                  data-testid="trigger-result-skipped"
                >
                  <AlertTriangle className="size-4" aria-hidden="true" />
                  <AlertTitle>
                    {t("buildSources.triggerResultSkippedTitle", {
                      count: result.skippedServices.length,
                    })}
                  </AlertTitle>
                  <AlertDescription>
                    <ul className="mt-1 space-y-1 text-xs">
                      {result.skippedServices.map((item) => (
                        <li
                          key={item.serviceId}
                          className="flex flex-wrap items-center justify-between gap-2"
                        >
                          <span className="font-medium">
                            {serviceNameById.get(item.serviceId) ??
                              item.serviceId}
                          </span>
                          <Badge variant="outline" className="text-xs">
                            {skippedReasonLabel(item.reason, t)}
                          </Badge>
                        </li>
                      ))}
                    </ul>
                  </AlertDescription>
                </Alert>
              ) : null}
            </div>

            <DialogFooter className="gap-2 sm:gap-2">
              <Button
                type="button"
                variant="outline"
                onClick={() => handleOpenChange(false)}
              >
                {t("common.close")}
              </Button>
              <Button asChild>
                <Link
                  to="/build-sources/$buildSourceId/builds/$buildId"
                  params={{
                    buildSourceId: buildSource.id,
                    buildId: result.id,
                  }}
                  onClick={() => handleOpenChange(false)}
                >
                  {t("buildSources.triggerResultViewLogs")}
                </Link>
              </Button>
            </DialogFooter>
          </>
        ) : (
          <>
            <DialogHeader>
              <DialogTitle>{t("buildSources.triggerDialogTitle")}</DialogTitle>
              <DialogDescription>
                {t("buildSources.triggerDialogDescription", {
                  name: buildSource.name,
                })}
              </DialogDescription>
            </DialogHeader>

            <div className="space-y-5 py-2">
              <fieldset className="space-y-3">
                <legend className="text-sm font-medium">
                  {t("buildSources.triggerScopeLegend")}
                </legend>
                <div
                  role="radiogroup"
                  aria-label={t("buildSources.triggerScopeLegend")}
                  className="space-y-2.5"
                >
                  <div className="flex items-start gap-2.5 rounded-md border p-3">
                    <input
                      id="trigger-scope-autodeploy"
                      type="radio"
                      name="trigger-build-scope"
                      value="autodeploy_only"
                      checked={scope === "autodeploy_only"}
                      onChange={() => setScope("autodeploy_only")}
                      className="accent-primary mt-0.5 size-4 shrink-0 cursor-pointer"
                    />
                    <Label
                      htmlFor="trigger-scope-autodeploy"
                      className="flex flex-1 cursor-pointer flex-col items-start gap-0.5 font-normal"
                    >
                      <span className="font-medium">
                        {t("buildSources.triggerScopeAutoDeploy")}
                      </span>
                      <span className="text-muted-foreground text-xs">
                        {t("buildSources.triggerScopeAutoDeployHint")}
                      </span>
                    </Label>
                  </div>

                  <div className="flex items-start gap-2.5 rounded-md border p-3">
                    <input
                      id="trigger-scope-none"
                      type="radio"
                      name="trigger-build-scope"
                      value="none"
                      checked={scope === "none"}
                      onChange={() => setScope("none")}
                      className="accent-primary mt-0.5 size-4 shrink-0 cursor-pointer"
                    />
                    <Label
                      htmlFor="trigger-scope-none"
                      className="flex flex-1 cursor-pointer flex-col items-start gap-0.5 font-normal"
                    >
                      <span className="font-medium">
                        {t("buildSources.triggerScopeNone")}
                      </span>
                      <span className="text-muted-foreground text-xs">
                        {t("buildSources.triggerScopeNoneHint")}
                      </span>
                    </Label>
                  </div>

                  <div className="space-y-2.5 rounded-md border p-3">
                    <div className="flex items-start gap-2.5">
                      <input
                        id="trigger-scope-custom"
                        type="radio"
                        name="trigger-build-scope"
                        value="custom"
                        checked={scope === "custom"}
                        onChange={() => setScope("custom")}
                        className="accent-primary mt-0.5 size-4 shrink-0 cursor-pointer"
                      />
                      <Label
                        htmlFor="trigger-scope-custom"
                        className="flex flex-1 cursor-pointer flex-col items-start gap-0.5 font-normal"
                      >
                        <span className="font-medium">
                          {t("buildSources.triggerScopeCustom")}
                        </span>
                        <span className="text-muted-foreground text-xs">
                          {t("buildSources.triggerScopeCustomHint")}
                        </span>
                      </Label>
                    </div>

                    {scope === "custom" ? (
                      <div className="ml-6 space-y-2 pt-1">
                        {buildSource.serviceLinks.length === 0 ? (
                          <p className="text-muted-foreground text-xs">
                            {t("buildSources.triggerScopeCustomNoServices")}
                          </p>
                        ) : (
                          <>
                            <div className="max-h-44 space-y-1.5 overflow-y-auto rounded-md border p-2.5">
                              {buildSource.serviceLinks.map((service) => {
                                const checkboxId = `trigger-service-${service.id}`;
                                const checked = selectedIds.includes(
                                  service.id,
                                );
                                return (
                                  <div
                                    key={service.id}
                                    className="flex items-center gap-2"
                                  >
                                    <Checkbox
                                      id={checkboxId}
                                      checked={checked}
                                      onCheckedChange={() =>
                                        toggleService(service.id)
                                      }
                                    />
                                    <Label
                                      htmlFor={checkboxId}
                                      className="min-w-0 flex-1 cursor-pointer truncate text-sm font-normal"
                                    >
                                      {service.name}
                                    </Label>
                                  </div>
                                );
                              })}
                            </div>
                            {selectedIds.length === 0 ? (
                              <p className="text-destructive text-xs" role="status">
                                {t("buildSources.triggerScopeCustomSelectOne")}
                              </p>
                            ) : null}
                          </>
                        )}
                      </div>
                    ) : null}
                  </div>
                </div>
              </fieldset>

              <div className="flex items-start gap-2.5 rounded-md border p-3">
                <Checkbox
                  id="trigger-clear-cache"
                  checked={clearCache}
                  onCheckedChange={(checked) => setClearCache(checked === true)}
                  className="mt-0.5"
                />
                <Label
                  htmlFor="trigger-clear-cache"
                  className="flex flex-1 cursor-pointer flex-col items-start gap-0.5 font-normal"
                >
                  <span className="font-medium">
                    {t("buildSources.triggerClearCache")}
                  </span>
                  <span className="text-muted-foreground text-xs">
                    {t("buildSources.triggerClearCacheHint")}
                  </span>
                </Label>
              </div>
            </div>

            <DialogFooter className="gap-2 sm:gap-2">
              <Button
                type="button"
                variant="outline"
                disabled={busy}
                onClick={() => handleOpenChange(false)}
              >
                {t("common.cancel")}
              </Button>
              <Button
                type="button"
                disabled={!canSubmit}
                onClick={() => void handleSubmit()}
              >
                {busy ? (
                  <Loader2 className="size-4 animate-spin" aria-hidden="true" />
                ) : (
                  <Play className="size-4" aria-hidden="true" />
                )}
                {busy
                  ? t("buildSources.triggerSubmitting")
                  : t("buildSources.triggerBuildButton")}
              </Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
