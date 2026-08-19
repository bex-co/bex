import { useMemo, useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { Loader2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { Switch } from "@/common/components/ui/switch";
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
import { WebhookEventPickerField } from "@/features/webhooks/components/webhook-event-picker-field";
import { useDeleteWebhook } from "@/features/webhooks/hooks/use-delete-webhook";
import { useSetWebhookEnabled } from "@/features/webhooks/hooks/use-set-webhook-enabled";
import { useUpdateWebhook } from "@/features/webhooks/hooks/use-update-webhook";
import { useWebhookEventTypes } from "@/features/webhooks/hooks/use-webhook-event-types";
import { useWebhookFormFeedback } from "@/features/webhooks/hooks/use-webhook-form-feedback";
import { useWebhooks } from "@/features/webhooks/hooks/use-webhooks";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import type { WebhookEndpointView } from "@/features/webhooks/types";

function sameSet(a: Set<string>, b: string[]): boolean {
  return a.size === b.length && b.every((item) => a.has(item));
}

export function WebhookSettingsCard({
  endpoint,
}: {
  endpoint: WebhookEndpointView;
}) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const {
    eventTypes,
    loading: typesLoading,
    error: typesError,
    retry: retryTypes,
  } = useWebhookEventTypes();
  const {
    update,
    busy: saving,
    error: mutationError,
    clearError,
  } = useUpdateWebhook();
  const { setEnabled, toggling } = useSetWebhookEnabled();
  const { remove, deleting } = useDeleteWebhook();
  const { endpoints } = useWebhooks({ poll: false });
  const { canManage, loaded: capabilitiesLoaded } = useCapabilities();

  const [name, setName] = useState(endpoint.name);
  const [url, setUrl] = useState(endpoint.url);
  const [checked, setChecked] = useState<Set<string>>(
    () => new Set(endpoint.eventTypes),
  );
  const [allEvents, setAllEvents] = useState(
    () => endpoint.eventTypes.length === 0,
  );
  const { errors, nameRef, urlRef, eventsRef, validate, clearField } =
    useWebhookFormFeedback({
      mutationError,
      fallbackMessage: t("webhooks.updateError"),
      clearMutationError: clearError,
    });
  const pickerValue = useMemo(
    () => (allEvents ? new Set(eventTypes) : checked),
    [allEvents, checked, eventTypes],
  );
  const targetEventTypes = allEvents ? [] : [...checked];

  const dirty =
    name !== endpoint.name ||
    url !== endpoint.url ||
    !sameSet(new Set(targetEventTypes), endpoint.eventTypes);
  const catalogReady = !typesLoading && !typesError && eventTypes.length > 0;
  const canSave = dirty && catalogReady && canManage && !saving;

  async function handleSave() {
    const valid = validate({
      name,
      url,
      selectedEventCount: allEvents ? eventTypes.length : checked.size,
      existingNames: endpoints
        .filter((candidate) => candidate.id !== endpoint.id)
        .map((candidate) => candidate.name),
      currentName: endpoint.name,
    });
    if (!valid) return;
    if (!canSave) return;
    // Send only what changed — the backend's Update keeps omitted fields
    // (service.Update's merge), same semantics as REST PATCH.
    await update(endpoint.id, {
      ...(name !== endpoint.name ? { name: name.trim() } : {}),
      ...(url !== endpoint.url ? { url: url.trim() } : {}),
      ...(!sameSet(new Set(targetEventTypes), endpoint.eventTypes)
        ? { eventTypes: targetEventTypes }
        : {}),
    });
  }

  async function handleDelete() {
    const ok = await remove(endpoint.id, endpoint.name);
    if (ok) await navigate({ to: "/webhooks", replace: true });
  }

  return (
    <div className="space-y-6">
      {capabilitiesLoaded && !canManage ? (
        <p className="text-muted-foreground text-sm" role="status">
          {t("webhooks.manageRequired")}
        </p>
      ) : null}
      {Object.keys(errors).length > 0 ? (
        <p className="text-destructive text-sm" role="alert">
          {t("webhooks.formErrorsSummary")}
        </p>
      ) : null}
      <Card>
        <CardHeader>
          <CardTitle>{t("webhooks.settingsGeneral")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="flex items-start justify-between gap-4">
            <div>
              <Label htmlFor="webhook-status">
                {t("webhooks.settingsStatus")}
              </Label>
              <p className="text-muted-foreground text-sm">
                {t("webhooks.settingsStatusHelp")}
              </p>
              {!endpoint.enabled && endpoint.disabledReason ? (
                <p className="text-destructive text-sm">
                  {endpoint.disabledReason}
                </p>
              ) : null}
            </div>
            <Switch
              id="webhook-status"
              checked={endpoint.enabled}
              disabled={!canManage || toggling === endpoint.id}
              onCheckedChange={(v) =>
                void setEnabled(endpoint.id, endpoint.name, v)
              }
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="webhook-settings-name">
              {t("webhooks.fieldName")}
            </Label>
            <p className="text-muted-foreground text-sm">
              {t("webhooks.fieldNameHelp")}
            </p>
            <Input
              id="webhook-settings-name"
              ref={nameRef}
              value={name}
              onChange={(e) => {
                setName(e.target.value);
                clearField("name");
              }}
              autoComplete="off"
              disabled={!canManage}
              aria-invalid={!!errors.name}
              aria-describedby={
                errors.name ? "webhook-settings-name-error" : undefined
              }
            />
            {errors.name ? (
              <p
                id="webhook-settings-name-error"
                className="text-destructive text-sm"
              >
                {errors.name}
              </p>
            ) : null}
          </div>
          <div className="space-y-2">
            <Label htmlFor="webhook-settings-url">
              {t("webhooks.fieldUrl")}
            </Label>
            <p className="text-muted-foreground text-sm">
              {t("webhooks.fieldUrlHelp")}
            </p>
            <Input
              id="webhook-settings-url"
              ref={urlRef}
              value={url}
              onChange={(e) => {
                setUrl(e.target.value);
                clearField("url");
              }}
              autoComplete="off"
              inputMode="url"
              disabled={!canManage}
              aria-invalid={!!errors.url}
              aria-describedby={
                errors.url ? "webhook-settings-url-error" : undefined
              }
            />
            {errors.url ? (
              <p
                id="webhook-settings-url-error"
                className="text-destructive text-sm"
              >
                {errors.url}
              </p>
            ) : null}
          </div>
          <div className="space-y-1">
            <Label>{t("webhooks.secretLabel")}</Label>
            {/* Mint-once contract kept by the t006 decision — deliberate
                divergence from Render's anytime-reveal, recorded in
                docs/render-artifacts/webhooks-ui.md + ADR018. */}
            <p className="text-muted-foreground text-sm">
              {t("webhooks.secretMintOnceNote")}
            </p>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("webhooks.settingsEvents")}</CardTitle>
          <CardDescription>{t("webhooks.fieldEventsHelp")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div ref={eventsRef} tabIndex={-1} aria-invalid={!!errors.events}>
            <WebhookEventPickerField
              eventTypes={eventTypes}
              loading={typesLoading}
              error={typesError}
              retry={retryTypes}
              value={pickerValue}
              onChange={(next) => {
                const selectedAll =
                  eventTypes.length > 0 && next.size === eventTypes.length;
                setAllEvents(selectedAll);
                setChecked(selectedAll ? new Set() : next);
                clearField("events");
              }}
              disabled={saving || !canManage}
              describedBy={
                errors.events ? "webhook-settings-events-error" : undefined
              }
            />
          </div>
          {errors.events ? (
            <p
              id="webhook-settings-events-error"
              className="text-destructive text-sm"
            >
              {errors.events}
            </p>
          ) : null}
          {canManage ? (
            <div className="flex justify-end">
              <Button onClick={() => void handleSave()} disabled={!canSave}>
                {saving ? <Loader2 className="animate-spin" /> : null}
                {t("webhooks.saveChanges")}
              </Button>
            </div>
          ) : null}
          {errors.form ? (
            <p className="text-destructive text-sm" role="alert">
              {errors.form}
            </p>
          ) : null}
        </CardContent>
      </Card>

      {canManage ? (
        <Card>
          <CardHeader>
            <CardTitle className="text-destructive">
              {t("webhooks.deleteSection")}
            </CardTitle>
            <CardDescription>{t("webhooks.deleteConfirmBody")}</CardDescription>
          </CardHeader>
          <CardContent>
            <DeleteWebhookConfirm
              name={endpoint.name}
              deleting={deleting === endpoint.id}
              onConfirm={() => void handleDelete()}
            />
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}

/**
 * Render's "sudo command" delete confirmation: the destructive button stays
 * disabled until the literal `delete webhook <name>` is typed.
 */
function DeleteWebhookConfirm({
  name,
  deleting,
  onConfirm,
}: {
  name: string;
  deleting: boolean;
  onConfirm: () => void;
}) {
  const { t } = useTranslations();
  const [confirmText, setConfirmText] = useState("");
  const command = `delete webhook ${name}`;
  const confirmed = confirmText === command;

  return (
    <AlertDialog onOpenChange={(open) => !open && setConfirmText("")}>
      <AlertDialogTrigger asChild>
        <Button variant="destructive" disabled={deleting}>
          {deleting ? <Loader2 className="animate-spin" /> : null}
          {t("webhooks.delete")}
        </Button>
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>
            {t("webhooks.deleteConfirmTitle", { name })}
          </AlertDialogTitle>
          <AlertDialogDescription>
            {t("webhooks.deleteConfirmBody")}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <div className="space-y-2">
          <p className="text-sm">
            {t("webhooks.deleteTypeToConfirm", { command })}
          </p>
          <Label htmlFor="webhook-delete-confirm" className="sr-only">
            {t("webhooks.deleteCommandLabel")}
          </Label>
          <Input
            id="webhook-delete-confirm"
            value={confirmText}
            onChange={(e) => setConfirmText(e.target.value)}
            placeholder={command}
            autoComplete="off"
          />
        </div>
        <AlertDialogFooter>
          <AlertDialogCancel>{t("webhooks.deleteCancel")}</AlertDialogCancel>
          <AlertDialogAction disabled={!confirmed} onClick={onConfirm}>
            {t("webhooks.delete")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
