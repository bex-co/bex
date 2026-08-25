import { useState } from "react";
import { Link, createFileRoute, useNavigate } from "@tanstack/react-router";
import { WebhookCreatePageSkeleton } from "@/common/components/route-skeletons";
import { ArrowLeft, Loader2 } from "lucide-react";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { Switch } from "@/common/components/ui/switch";
import { CopyButton } from "@/common/components/copy-button";
import { WebhookEventPickerField } from "@/features/webhooks/components/webhook-event-picker-field";
import { useCreateWebhook } from "@/features/webhooks/hooks/use-create-webhook";
import { useWebhookEventTypes } from "@/features/webhooks/hooks/use-webhook-event-types";
import { useWebhookFormFeedback } from "@/features/webhooks/hooks/use-webhook-form-feedback";
import { useWebhooks } from "@/features/webhooks/hooks/use-webhooks";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import type { CreatedWebhookEndpoint } from "@/features/webhooks/types";
import { translatedTitleHead } from "@/common/lib/document-head";

// The trailing underscore deliberately escapes the /webhooks list route's
// component hierarchy while keeping the public URL `/webhooks/new` — the list
// is a page without an <Outlet />, so nesting would leave it mounted (the
// env-groups_.$groupId precedent).
export const Route = createFileRoute("/webhooks_/new")({
  staticData: { chrome: true },
  component: NewWebhookPage,
  pendingComponent: WebhookCreatePageSkeleton,
  beforeLoad: requireAuth(),
  head: ({ match }) => translatedTitleHead("webhooks.newTitle", match),
});

export function NewWebhookPage() {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { create, busy, error: mutationError, clearError } = useCreateWebhook();
  const {
    eventTypes,
    loading: typesLoading,
    error: typesError,
    retry: retryTypes,
  } = useWebhookEventTypes();
  const { endpoints } = useWebhooks({ poll: false });
  const { canManage, loaded: capabilitiesLoaded } = useCapabilities();

  const [name, setName] = useState("");
  const [url, setUrl] = useState("");
  const [checked, setChecked] = useState<Set<string>>(() => new Set());
  const [enabled, setEnabled] = useState(true);
  const [created, setCreated] = useState<CreatedWebhookEndpoint | null>(null);
  const { errors, nameRef, urlRef, eventsRef, validate, clearField } =
    useWebhookFormFeedback({
      mutationError,
      fallbackMessage: t("webhooks.createError"),
      clearMutationError: clearError,
    });

  const catalogReady = !typesLoading && !typesError && eventTypes.length > 0;
  const mutationAllowed = canManage && !busy;

  async function handleSubmit() {
    const valid = validate({
      name,
      url,
      selectedEventCount: checked.size,
      existingNames: endpoints.map((endpoint) => endpoint.name),
    });
    if (!valid) return;
    if (!catalogReady || !mutationAllowed) return;
    // Render's empty eventFilter is the durable "all current and future events"
    // representation. Keep partial selections explicit, but compact a checked
    // All-events picker so newly introduced event types are included automatically.
    const eventFilter = checked.size === eventTypes.length ? [] : [...checked];
    const endpoint = await create(
      name.trim(),
      url.trim(),
      eventFilter,
      enabled,
    );
    if (endpoint) setCreated(endpoint);
  }

  return (
    <DashboardLayout>
      <div className="flex items-center gap-3 border-b px-4 py-4 sm:px-6">
        <Button variant="ghost" size="icon" asChild>
          <Link to="/webhooks" aria-label={t("webhooks.backToList")}>
            <ArrowLeft />
          </Link>
        </Button>
        <h1 className="text-xl font-semibold">{t("webhooks.newTitle")}</h1>
      </div>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-2xl space-y-6">
          {created ? (
            <SecretStep
              created={created}
              onView={() =>
                void navigate({
                  to: "/webhook/$webhookId",
                  params: { webhookId: created.id },
                })
              }
            />
          ) : (
            <Card>
              <CardHeader>
                <CardTitle>{t("webhooks.createTitle")}</CardTitle>
                <CardDescription>
                  {t("webhooks.createDescription")}
                </CardDescription>
              </CardHeader>
              <CardContent>
                {capabilitiesLoaded && !canManage ? (
                  <p className="text-muted-foreground text-sm" role="status">
                    {t("webhooks.manageRequired")}
                  </p>
                ) : (
                  <form
                    className="space-y-6"
                    noValidate
                    onSubmit={(event) => {
                      event.preventDefault();
                      void handleSubmit();
                    }}
                  >
                    {Object.keys(errors).length > 0 ? (
                      <p className="text-destructive text-sm" role="alert">
                        {t("webhooks.formErrorsSummary")}
                      </p>
                    ) : null}
                    <div className="space-y-2">
                      <Label htmlFor="webhook-name">
                        {t("webhooks.fieldName")}
                      </Label>
                      <p
                        id="webhook-name-help"
                        className="text-muted-foreground text-sm"
                      >
                        {t("webhooks.fieldNameHelp")}
                      </p>
                      <Input
                        id="webhook-name"
                        ref={nameRef}
                        value={name}
                        onChange={(e) => {
                          setName(e.target.value);
                          clearField("name");
                        }}
                        placeholder={t("webhooks.fieldNamePlaceholder")}
                        autoComplete="off"
                        autoFocus
                        aria-invalid={!!errors.name}
                        aria-describedby={`webhook-name-help${errors.name ? " webhook-name-error" : ""}`}
                      />
                      {errors.name ? (
                        <p
                          id="webhook-name-error"
                          className="text-destructive text-sm"
                        >
                          {errors.name}
                        </p>
                      ) : null}
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="webhook-url">
                        {t("webhooks.fieldUrl")}
                      </Label>
                      <p
                        id="webhook-url-help"
                        className="text-muted-foreground text-sm"
                      >
                        {t("webhooks.fieldUrlHelp")}
                      </p>
                      <Input
                        id="webhook-url"
                        ref={urlRef}
                        value={url}
                        onChange={(e) => {
                          setUrl(e.target.value);
                          clearField("url");
                        }}
                        placeholder={t("webhooks.fieldUrlPlaceholder")}
                        autoComplete="off"
                        inputMode="url"
                        aria-invalid={!!errors.url}
                        aria-describedby={`webhook-url-help${errors.url ? " webhook-url-error" : ""}`}
                      />
                      {errors.url ? (
                        <p
                          id="webhook-url-error"
                          className="text-destructive text-sm"
                        >
                          {errors.url}
                        </p>
                      ) : null}
                    </div>
                    <div
                      ref={eventsRef}
                      className="space-y-2"
                      tabIndex={-1}
                      aria-invalid={!!errors.events}
                    >
                      <Label id="webhook-events-label">
                        {t("webhooks.fieldEvents")}
                      </Label>
                      <p
                        id="webhook-events-help"
                        className="text-muted-foreground text-sm"
                      >
                        {t("webhooks.fieldEventsHelp")}
                      </p>
                      <WebhookEventPickerField
                        eventTypes={eventTypes}
                        loading={typesLoading}
                        error={typesError}
                        retry={retryTypes}
                        value={checked}
                        onChange={(next) => {
                          setChecked(next);
                          clearField("events");
                        }}
                        disabled={busy}
                        describedBy={`webhook-events-help${errors.events ? " webhook-events-error" : ""}`}
                      />
                      {errors.events ? (
                        <p
                          id="webhook-events-error"
                          className="text-destructive text-sm"
                        >
                          {errors.events}
                        </p>
                      ) : null}
                    </div>
                    <div className="flex items-start justify-between gap-4">
                      <div>
                        <Label htmlFor="webhook-create-enabled">
                          {t("webhooks.settingsStatus")}
                        </Label>
                        <p className="text-muted-foreground text-sm">
                          {t("webhooks.createEnabledHelp")}
                        </p>
                      </div>
                      <Switch
                        id="webhook-create-enabled"
                        checked={enabled}
                        onCheckedChange={setEnabled}
                        disabled={busy}
                      />
                    </div>
                    <div className="flex justify-end">
                      <Button
                        type="submit"
                        disabled={!catalogReady || !mutationAllowed}
                        aria-describedby={
                          !catalogReady ? "webhook-create-disabled" : undefined
                        }
                      >
                        {busy ? <Loader2 className="animate-spin" /> : null}
                        {t("webhooks.createSubmit")}
                      </Button>
                    </div>
                    {!catalogReady ? (
                      <p
                        id="webhook-create-disabled"
                        className="text-muted-foreground text-right text-xs"
                      >
                        {t("webhooks.formUnavailableEvents")}
                      </p>
                    ) : null}
                    {errors.form ? (
                      <p className="text-destructive text-sm" role="alert">
                        {errors.form}
                      </p>
                    ) : null}
                  </form>
                )}
              </CardContent>
            </Card>
          )}
        </div>
      </div>
    </DashboardLayout>
  );
}

function SecretStep({
  created,
  onView,
}: {
  created: CreatedWebhookEndpoint;
  onView: () => void;
}) {
  const { t } = useTranslations();
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("webhooks.createdTitle")}</CardTitle>
        <CardDescription>{t("webhooks.createdWarning")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="bg-muted/50 flex items-center gap-2 rounded-md border p-3">
          <code className="flex-1 overflow-x-auto font-mono text-sm break-all">
            {created.secret}
          </code>
          <CopyButton
            value={created.secret}
            label={t("webhooks.copy")}
            successText={t("webhooks.copied")}
            errorText={t("webhooks.copyError")}
          />
        </div>
        <div className="flex justify-end">
          <Button onClick={onView}>{t("webhooks.createdView")}</Button>
        </div>
      </CardContent>
    </Card>
  );
}
