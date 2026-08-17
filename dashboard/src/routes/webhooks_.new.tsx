import { useState } from "react";
import { Link, createFileRoute, useNavigate } from "@tanstack/react-router";
import { FormPageSkeleton } from "@/common/components/detail-skeletons";
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
import { EventPicker } from "@/features/webhooks/components/event-picker";
import { useCreateWebhook } from "@/features/webhooks/hooks/use-create-webhook";
import { useWebhookEventTypes } from "@/features/webhooks/hooks/use-webhook-event-types";
import type { CreatedWebhookEndpoint } from "@/features/webhooks/types";
import { translatedTitleHead } from "@/common/lib/document-head";

// The trailing underscore deliberately escapes the /webhooks list route's
// component hierarchy while keeping the public URL `/webhooks/new` — the list
// is a page without an <Outlet />, so nesting would leave it mounted (the
// env-groups_.$groupId precedent).
export const Route = createFileRoute("/webhooks_/new")({
  staticData: { chrome: true },
  component: NewWebhookPage,
  pendingComponent: FormPageSkeleton,
  beforeLoad: requireAuth(),
  head: ({ match }) => translatedTitleHead("webhooks.newTitle", match),
});

/**
 * Render's "Create a new Webhook" page (`/webhooks/new`,
 * docs/render-artifacts/webhooks-ui.md) — a full page, not a dialog (w1/m49;
 * the dialog it replaces additionally had a blocking hit-test bug, see t007).
 * Name + Endpoint URL + the grouped event picker; on success the signing
 * secret shows once in-page (the mint-once contract, kept by the t006
 * decision — Render redirects straight to the detail page because its secret
 * stays retrievable), then hands off to `/webhook/$webhookId`.
 */
export function NewWebhookPage() {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { create, busy } = useCreateWebhook();
  const { eventTypes, loading: typesLoading } = useWebhookEventTypes();

  const [name, setName] = useState("");
  const [url, setUrl] = useState("");
  const [checked, setChecked] = useState<Set<string>>(() => new Set());
  const [enabled, setEnabled] = useState(true);
  const [created, setCreated] = useState<CreatedWebhookEndpoint | null>(null);

  const submittable =
    name.trim() !== "" && url.trim() !== "" && checked.size > 0 && !busy;

  async function handleSubmit() {
    if (!submittable) return;
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
              <CardContent className="space-y-6">
                <div className="space-y-2">
                  <Label htmlFor="webhook-name">
                    {t("webhooks.fieldName")}
                  </Label>
                  <p className="text-muted-foreground text-sm">
                    {t("webhooks.fieldNameHelp")}
                  </p>
                  <Input
                    id="webhook-name"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    placeholder={t("webhooks.fieldNamePlaceholder")}
                    autoComplete="off"
                    autoFocus
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="webhook-url">{t("webhooks.fieldUrl")}</Label>
                  <p className="text-muted-foreground text-sm">
                    {t("webhooks.fieldUrlHelp")}
                  </p>
                  <Input
                    id="webhook-url"
                    value={url}
                    onChange={(e) => setUrl(e.target.value)}
                    placeholder={t("webhooks.fieldUrlPlaceholder")}
                    autoComplete="off"
                    inputMode="url"
                  />
                </div>
                <div className="space-y-2">
                  <Label>{t("webhooks.fieldEvents")}</Label>
                  <p className="text-muted-foreground text-sm">
                    {t("webhooks.fieldEventsHelp")}
                  </p>
                  {typesLoading && eventTypes.length === 0 ? (
                    <p className="text-muted-foreground py-2 text-sm">
                      {t("webhooks.eventsLoading")}
                    </p>
                  ) : (
                    <EventPicker
                      eventTypes={eventTypes}
                      value={checked}
                      onChange={setChecked}
                      disabled={busy}
                    />
                  )}
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
                    onClick={() => void handleSubmit()}
                    disabled={!submittable}
                  >
                    {busy ? <Loader2 className="animate-spin" /> : null}
                    {t("webhooks.createSubmit")}
                  </Button>
                </div>
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
