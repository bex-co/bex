import { useMemo, useState } from "react";
import { Link } from "@tanstack/react-router";
import {
  AlertTriangle,
  ExternalLink,
  Link2,
  Loader2,
  Plus,
  Server,
} from "lucide-react";
import { useApolloClient } from "@apollo/client/react";
import {
  Alert,
  AlertDescription,
} from "@/common/components/ui/alert";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import { Card, CardContent } from "@/common/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { useTranslations } from "@/common/hooks/use-translations";
import { regionLabel } from "@/common/lib/region-label";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useLinkBuildSource } from "@/features/services/hooks/use-link-build-source";
import { useServices } from "@/features/services/hooks/use-services";
import {
  RUNTIME_COMMANDS,
  isNativeRuntime,
} from "@/features/services/lib/runtime";
import {
  SERVICE_TYPE_LABEL,
  deriveServiceType,
  isCronType,
} from "@/features/services/lib/service-type";
import { BuildSourceDocument, BuildSourcesDocument } from "@/graphql/definitions";
import { useBuildSourceDetailContext } from "@/features/build-sources/components/build-source-detail-context";
import { isLinkableServiceType } from "@/features/build-sources/lib/create-build-source-input";

/**
 * Linked Services tab (`/build-sources/$buildSourceId/services`, `w2/m614/t005`):
 * displays all linked services with type/region/auto-deploy info, navigation to
 * change/unlink source, and a `can_create`-gated Link Existing Service dialog.
 */
export function BuildSourceServicesTab() {
  const { t } = useTranslations();
  const client = useApolloClient();
  const { buildSource, refetch } = useBuildSourceDetailContext();
  const capabilities = useCapabilities();
  const { canCreate } = capabilities;
  const createReasonKey = capabilities.reasonKey("can_create");
  const createReason = createReasonKey ? t(createReasonKey) : undefined;
  const { services } = useServices({ poll: false });
  const { linkBuildSource, busy, lastError } = useLinkBuildSource();

  const [linkOpen, setLinkOpen] = useState(false);
  const [selectedServiceId, setSelectedServiceId] = useState("");
  const [startCommand, setStartCommand] = useState("");
  const [showError, setShowError] = useState(false);

  const serviceById = useMemo(
    () => new Map(services.map((s) => [s.id, s])),
    [services],
  );
  const linkedIds = useMemo(
    () => new Set(buildSource.serviceLinks.map((l) => l.id)),
    [buildSource.serviceLinks],
  );
  const eligibleServices = useMemo(
    () =>
      services.filter(
        (svc) => isLinkableServiceType(svc.type) && !linkedIds.has(svc.id),
      ),
    [services, linkedIds],
  );

  const nativeRuntime =
    buildSource.kind === "git" &&
    buildSource.git?.runtime &&
    isNativeRuntime(buildSource.git.runtime)
      ? buildSource.git.runtime
      : null;
  const needsStartCommand = nativeRuntime !== null;
  const selectedService = eligibleServices.find(
    (s) => s.id === selectedServiceId,
  );
  const regionMismatch =
    buildSource.kind === "git" &&
    Boolean(buildSource.git?.region) &&
    Boolean(selectedService?.region) &&
    buildSource.git?.region !== selectedService?.region;

  function openLinkDialog() {
    setSelectedServiceId("");
    setStartCommand(
      nativeRuntime ? RUNTIME_COMMANDS[nativeRuntime].start : "",
    );
    setShowError(false);
    setLinkOpen(true);
  }

  function handleSelectService(id: string) {
    setSelectedServiceId(id);
    setShowError(false);
    if (!needsStartCommand) return;
    const svc = eligibleServices.find((s) => s.id === id);
    const fallback = nativeRuntime ? RUNTIME_COMMANDS[nativeRuntime].start : "";
    setStartCommand(svc?.startCommand || svc?.command || fallback);
  }

  const canSubmitLink =
    !busy &&
    selectedServiceId !== "" &&
    !regionMismatch &&
    (!needsStartCommand || startCommand.trim() !== "");

  async function handleLinkSubmit() {
    if (!canSubmitLink) return;
    setShowError(false);
    const result = await linkBuildSource(
      selectedServiceId,
      buildSource.id,
      needsStartCommand ? startCommand.trim() : undefined,
      false,
    );
    if (!result.ok) {
      setShowError(true);
      return;
    }
    setLinkOpen(false);
    await refetch().catch(() => undefined);
    await client
      .refetchQueries({
        include: [BuildSourceDocument, BuildSourcesDocument],
      })
      .catch(() => undefined);
  }

  return (
    <div data-region="linked-services" className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-base font-semibold">
            {t("buildSources.servicesTabTitle")}
          </h2>
          <p className="text-muted-foreground text-sm">
            {t("buildSources.servicesTabDescription")}
          </p>
        </div>
        <PermissionTooltip reason={createReason}>
          <Button
            size="sm"
            variant="outline"
            disabled={!canCreate}
            onClick={openLinkDialog}
          >
            <Plus className="size-4" aria-hidden="true" />
            {t("buildSources.linkExistingService")}
          </Button>
        </PermissionTooltip>
      </div>

      {buildSource.serviceLinks.length === 0 ? (
        <div className="flex flex-col items-center gap-2 rounded-lg border border-dashed px-4 py-12 text-center">
          <Server className="text-muted-foreground size-8" aria-hidden="true" />
          <p className="text-sm font-medium">
            {t("buildSources.servicesEmptyTitle")}
          </p>
          <p className="text-muted-foreground max-w-md text-xs">
            {t("buildSources.servicesEmptyDescription")}
          </p>
          <PermissionTooltip reason={createReason}>
            <Button
              size="sm"
              variant="outline"
              className="mt-2"
              disabled={!canCreate}
              onClick={openLinkDialog}
            >
              <Link2 className="size-4" aria-hidden="true" />
              {t("buildSources.linkExistingService")}
            </Button>
          </PermissionTooltip>
        </div>
      ) : (
        <div className="space-y-3">
          {buildSource.serviceLinks.map((link) => {
            const svc = serviceById.get(link.id);
            const typeKey = svc ? deriveServiceType(svc.type) : null;
            return (
              <Card key={link.id} className="py-4">
                <CardContent className="flex flex-wrap items-center justify-between gap-3 px-4">
                  <div className="min-w-0 space-y-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <Link
                        to="/services/$serviceId"
                        params={{ serviceId: link.id }}
                        className="truncate font-medium hover:underline"
                      >
                        {svc?.name ?? link.name}
                      </Link>
                      {typeKey ? (
                        <Badge variant="outline">
                          {t(SERVICE_TYPE_LABEL[typeKey])}
                        </Badge>
                      ) : null}
                      {svc?.region ? (
                        <Badge variant="secondary">
                          {regionLabel(svc.region, t)}
                        </Badge>
                      ) : null}
                      {svc ? (
                        <Badge
                          variant={svc.autoDeploy ? "success" : "secondary"}
                        >
                          {t(
                            svc.autoDeploy
                              ? "buildSources.autoDeployOn"
                              : "buildSources.autoDeployOff",
                          )}
                        </Badge>
                      ) : null}
                      {svc?.suspended ? (
                        <Badge variant="destructive">
                          {t("buildSources.skippedReasonSuspended")}
                        </Badge>
                      ) : null}
                    </div>
                    <p className="text-muted-foreground font-mono text-xs">
                      {link.id}
                    </p>
                  </div>

                  <div className="flex shrink-0 items-center gap-2">
                    <Button asChild size="sm" variant="outline">
                      <Link
                        to="/services/$serviceId/settings"
                        params={{ serviceId: link.id }}
                      >
                        {t("buildSources.changeOrUnlinkSource")}
                        <ExternalLink className="size-3.5" aria-hidden="true" />
                      </Link>
                    </Button>
                  </div>
                </CardContent>
              </Card>
            );
          })}
        </div>
      )}

      <Dialog open={linkOpen} onOpenChange={setLinkOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{t("buildSources.linkDialogTitle")}</DialogTitle>
            <DialogDescription>
              {t("buildSources.linkDialogDescription", {
                name: buildSource.name,
              })}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-2">
            {eligibleServices.length === 0 ? (
              <p className="text-muted-foreground text-sm">
                {t("buildSources.linkDialogNoEligible")}
              </p>
            ) : (
              <>
                <div className="space-y-2">
                  <Label htmlFor="bds-link-service-select">
                    {t("buildSources.linkDialogServiceLabel")}
                  </Label>
                  <Select
                    value={selectedServiceId}
                    onValueChange={handleSelectService}
                  >
                    <SelectTrigger id="bds-link-service-select">
                      <SelectValue
                        placeholder={t(
                          "buildSources.linkDialogServicePlaceholder",
                        )}
                      />
                    </SelectTrigger>
                    <SelectContent>
                      {eligibleServices.map((svc) => {
                        const typeKey = deriveServiceType(svc.type);
                        return (
                          <SelectItem key={svc.id} value={svc.id}>
                            {svc.name} ({t(SERVICE_TYPE_LABEL[typeKey])}
                            {svc.region
                              ? ` · ${regionLabel(svc.region, t)}`
                              : ""}
                            )
                          </SelectItem>
                        );
                      })}
                    </SelectContent>
                  </Select>
                </div>

                {regionMismatch && selectedService ? (
                  <Alert variant="destructive" role="alert">
                    <AlertTriangle className="size-4" aria-hidden="true" />
                    <AlertDescription>
                      {t("buildSources.linkDialogRegionMismatch", {
                        sourceRegion: regionLabel(
                          buildSource.git?.region ?? "",
                          t,
                        ),
                        serviceRegion: regionLabel(
                          selectedService.region ?? "",
                          t,
                        ),
                      })}
                    </AlertDescription>
                  </Alert>
                ) : null}

                {needsStartCommand ? (
                  <div className="space-y-2">
                    <Label htmlFor="bds-link-start-command">
                      {t(
                        selectedService && isCronType(selectedService.type)
                          ? "services.deployCommandLabel"
                          : "services.startCommandLabel",
                      )}
                    </Label>
                    <Input
                      id="bds-link-start-command"
                      value={startCommand}
                      onChange={(e) => setStartCommand(e.target.value)}
                      placeholder={
                        nativeRuntime
                          ? RUNTIME_COMMANDS[nativeRuntime].start
                          : ""
                      }
                      className="font-mono"
                      autoComplete="off"
                    />
                    <p className="text-muted-foreground text-xs">
                      {t("buildSources.linkDialogStartCommandHint")}
                    </p>
                  </div>
                ) : null}
              </>
            )}

            {showError && lastError ? (
              <Alert variant="destructive" role="alert">
                <AlertTriangle className="size-4" aria-hidden="true" />
                <AlertDescription>{lastError}</AlertDescription>
              </Alert>
            ) : null}
          </div>

          <DialogFooter className="gap-2 sm:gap-2">
            <Button
              type="button"
              variant="outline"
              disabled={busy}
              onClick={() => setLinkOpen(false)}
            >
              {t("common.cancel")}
            </Button>
            <Button
              type="button"
              disabled={!canSubmitLink}
              onClick={() => void handleLinkSubmit()}
            >
              {busy ? (
                <Loader2 className="size-4 animate-spin" aria-hidden="true" />
              ) : (
                <Link2 className="size-4" aria-hidden="true" />
              )}
              {t("buildSources.linkDialogSubmit")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
