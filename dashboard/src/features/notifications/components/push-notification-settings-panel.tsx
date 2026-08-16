import { useMemo, useState } from "react";
import { AlertTriangle, Plus, Trash2 } from "lucide-react";
import {
  PushNotificationEvent,
  PushNotificationUrgency,
  PushNotificationWeekday,
  type PushNotificationClockRangeInput,
  type PushNotificationServiceOverrideInput,
  type PushNotificationSettingsInput,
} from "@/graphql/definitions";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Checkbox } from "@/common/components/ui/checkbox";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { Skeleton } from "@/common/components/ui/skeleton";
import { Switch } from "@/common/components/ui/switch";
import { cn } from "@/common/lib/utils/utils";
import { PanelCenteredState } from "@/common/components/panel-states";
import { useTranslations } from "@/common/hooks/use-translations";
import { useServices } from "@/features/services/hooks/use-services";
import {
  clonePushSettings,
  usePushNotificationSettings,
} from "@/features/notifications/hooks/use-push-notification-settings";
import { useUpdatePushNotificationSettings } from "@/features/notifications/hooks/use-update-push-notification-settings";
import {
  pushEvents,
  pushUrgencies,
  pushWeekdays,
  toggleListValue,
  validatePushSettings,
} from "@/features/notifications/lib/push-policy";

const eventKeys: Record<PushNotificationEvent, string> = {
  [PushNotificationEvent.DeployFailed]: "notifications.pushEventDeployFailed",
  [PushNotificationEvent.ServerFailed]: "notifications.pushEventServerFailed",
  [PushNotificationEvent.CronFailed]: "notifications.pushEventCronFailed",
  [PushNotificationEvent.DeployStarted]: "notifications.pushEventDeployStarted",
  [PushNotificationEvent.DeploySucceeded]:
    "notifications.pushEventDeploySucceeded",
  [PushNotificationEvent.ServerAvailable]:
    "notifications.pushEventServerAvailable",
  [PushNotificationEvent.ServiceSuspended]:
    "notifications.pushEventServiceSuspended",
  [PushNotificationEvent.ServiceResumed]:
    "notifications.pushEventServiceResumed",
  [PushNotificationEvent.UsageThreshold]:
    "notifications.pushEventUsageThreshold",
  [PushNotificationEvent.AgentNeedsDecision]:
    "notifications.pushEventAgentNeedsDecision",
  [PushNotificationEvent.AgentPrReady]: "notifications.pushEventAgentPrReady",
  [PushNotificationEvent.AgentFailed]: "notifications.pushEventAgentFailed",
};

const urgencyKeys: Record<PushNotificationUrgency, string> = {
  [PushNotificationUrgency.Routine]: "notifications.pushUrgencyRoutine",
  [PushNotificationUrgency.Important]: "notifications.pushUrgencyImportant",
  [PushNotificationUrgency.Critical]: "notifications.pushUrgencyCritical",
};

const weekdayKeys: Record<PushNotificationWeekday, string> = {
  [PushNotificationWeekday.Monday]: "notifications.weekdayMonday",
  [PushNotificationWeekday.Tuesday]: "notifications.weekdayTuesday",
  [PushNotificationWeekday.Wednesday]: "notifications.weekdayWednesday",
  [PushNotificationWeekday.Thursday]: "notifications.weekdayThursday",
  [PushNotificationWeekday.Friday]: "notifications.weekdayFriday",
  [PushNotificationWeekday.Saturday]: "notifications.weekdaySaturday",
  [PushNotificationWeekday.Sunday]: "notifications.weekdaySunday",
};

/**
 * Native select styled to match the Input/Button kit. Native rather than the
 * Radix `Select` on purpose: these are dense settings controls where the OS
 * picker is the better affordance, especially on mobile.
 */
function NativeSelect({ className, ...props }: React.ComponentProps<"select">) {
  return (
    <select
      className={cn(
        "border-input h-9 w-full rounded-md border bg-transparent px-3 text-sm",
        className,
      )}
      {...props}
    />
  );
}

export function PushNotificationSettingsPanel() {
  const { t } = useTranslations();
  const { settings, available, loading, error } = usePushNotificationSettings();

  if (error) {
    return (
      <Card>
        <CardContent className="pt-6">
          <PanelCenteredState
            icon={<AlertTriangle />}
            title={t("notifications.pushErrorTitle")}
            body={t("notifications.errorBody")}
          />
        </CardContent>
      </Card>
    );
  }

  if (loading) {
    return (
      <Card>
        <CardHeader>
          <Skeleton className="h-6 w-48" />
          <Skeleton className="h-4 w-full" />
        </CardHeader>
        <CardContent className="space-y-3">
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-32 w-full" />
        </CardContent>
      </Card>
    );
  }

  return (
    <PushSettingsForm
      key={JSON.stringify(settings)}
      initialSettings={settings}
      available={available}
    />
  );
}

function PushSettingsForm({
  initialSettings,
  available,
}: {
  initialSettings: PushNotificationSettingsInput;
  available: boolean;
}) {
  const { t } = useTranslations();
  const { update, busy } = useUpdatePushNotificationSettings();
  const { services } = useServices();
  const [draft, setDraft] = useState(() => clonePushSettings(initialSettings));
  const [validationKey, setValidationKey] = useState<string | null>(null);

  const serviceNames = useMemo(
    () => new Map(services.map((service) => [service.id, service.name])),
    [services],
  );
  const availableServices = services.filter(
    (service) =>
      !draft.serviceOverrides.some(
        (override) => override.serviceId === service.id,
      ),
  );

  async function save() {
    const candidate = {
      ...clonePushSettings(draft),
      timeZone: draft.timeZone.trim(),
    };
    const invalid = validatePushSettings(candidate);
    if (invalid) {
      setValidationKey(invalid);
      return;
    }
    setValidationKey(null);
    await update(candidate);
  }

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <CardTitle>{t("notifications.pushTitle")}</CardTitle>
          <span className="rounded-full bg-primary/10 px-2 py-0.5 text-xs font-medium text-primary">
            {t("notifications.bexExtension")}
          </span>
        </div>
        <CardDescription>{t("notifications.pushDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        {!available && (
          <div
            role="status"
            className="rounded-md border border-amber-500/40 bg-amber-500/10 p-3"
          >
            <p className="text-sm font-medium">
              {t("notifications.pushUnavailable")}
            </p>
            <p className="text-sm text-muted-foreground">
              {t("notifications.pushUnavailableHint")}
            </p>
          </div>
        )}
        <div className="flex items-center justify-between rounded-md border p-3">
          <div className="space-y-0.5 pr-4">
            <Label htmlFor="push-enabled">
              {t("notifications.pushEnabled")}
            </Label>
            <p className="text-sm text-muted-foreground">
              {t("notifications.pushEnabledHint")}
            </p>
          </div>
          <Switch
            id="push-enabled"
            checked={draft.enabled}
            disabled={busy}
            onCheckedChange={(enabled) => setDraft({ ...draft, enabled })}
          />
        </div>

        <EventFilter
          events={draft.events}
          disabled={busy || !draft.enabled}
          onChange={(events) => setDraft({ ...draft, events })}
        />

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <div className="space-y-2">
            <Label htmlFor="push-urgency">
              {t("notifications.pushMinimumUrgency")}
            </Label>
            <NativeSelect
              id="push-urgency"
              value={draft.minimumUrgency}
              disabled={busy || !draft.enabled}
              onChange={(event) =>
                setDraft({
                  ...draft,
                  minimumUrgency: event.target.value as PushNotificationUrgency,
                })
              }
            >
              {pushUrgencies.map((urgency) => (
                <option key={urgency} value={urgency}>
                  {t(urgencyKeys[urgency])}
                </option>
              ))}
            </NativeSelect>
          </div>
          <div className="space-y-2">
            <Label htmlFor="push-timezone">
              {t("notifications.pushTimezone")}
            </Label>
            <Input
              id="push-timezone"
              value={draft.timeZone}
              disabled={busy || !draft.enabled}
              placeholder={t("notifications.pushTimezonePlaceholder")}
              onChange={(event) =>
                setDraft({ ...draft, timeZone: event.target.value })
              }
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="push-deferral">
              {t("notifications.pushMaxDeferral")}
            </Label>
            <Input
              id="push-deferral"
              type="number"
              min="0.5"
              max="168"
              step="0.5"
              value={draft.maxDeferralSeconds / 3600}
              disabled={busy || !draft.enabled}
              onChange={(event) =>
                setDraft({
                  ...draft,
                  maxDeferralSeconds: Math.round(
                    Number(event.target.value) * 3600,
                  ),
                })
              }
            />
          </div>
        </div>

        <ScheduleEditor
          title={t("notifications.pushWorkingHours")}
          hint={t("notifications.pushWorkingHoursHint")}
          ranges={draft.workingHours}
          disabled={busy || !draft.enabled}
          onChange={(workingHours) => setDraft({ ...draft, workingHours })}
        />
        <ScheduleEditor
          title={t("notifications.pushQuietHours")}
          hint={t("notifications.pushQuietHoursHint")}
          ranges={draft.quietHours}
          disabled={busy || !draft.enabled}
          onChange={(quietHours) => setDraft({ ...draft, quietHours })}
        />

        <ServiceOverridesEditor
          overrides={draft.serviceOverrides}
          availableServices={availableServices}
          serviceNames={serviceNames}
          disabled={busy || !draft.enabled}
          onChange={(serviceOverrides) =>
            setDraft({ ...draft, serviceOverrides })
          }
        />

        {validationKey && (
          <p role="alert" className="text-sm text-destructive">
            {t(validationKey)}
          </p>
        )}
        <div className="flex justify-end">
          <Button loading={busy} onClick={() => void save()}>
            {t("notifications.pushSave")}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

function EventFilter({
  events,
  disabled,
  onChange,
}: {
  events: PushNotificationEvent[];
  disabled: boolean;
  onChange: (events: PushNotificationEvent[]) => void;
}) {
  const { t } = useTranslations();
  return (
    <fieldset className="space-y-2" disabled={disabled}>
      <legend className="text-sm font-medium">
        {t("notifications.pushEvents")}
      </legend>
      <p className="text-sm text-muted-foreground">
        {t("notifications.pushEventsHint")}
      </p>
      <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
        {pushEvents.map((event) => (
          <label
            key={event}
            className="flex items-center gap-2 rounded-md border p-2 text-sm"
          >
            <Checkbox
              checked={events.includes(event)}
              onCheckedChange={() => onChange(toggleListValue(events, event))}
            />
            {t(eventKeys[event])}
          </label>
        ))}
      </div>
    </fieldset>
  );
}

function ScheduleEditor({
  title,
  hint,
  ranges,
  disabled,
  onChange,
}: {
  title: string;
  hint: string;
  ranges: PushNotificationClockRangeInput[];
  disabled: boolean;
  onChange: (ranges: PushNotificationClockRangeInput[]) => void;
}) {
  const { t } = useTranslations();
  function update(index: number, range: PushNotificationClockRangeInput) {
    onChange(
      ranges.map((item, position) => (position === index ? range : item)),
    );
  }
  return (
    <fieldset className="space-y-3" disabled={disabled}>
      <div>
        <legend className="text-sm font-medium">{title}</legend>
        <p className="text-sm text-muted-foreground">{hint}</p>
      </div>
      {ranges.map((range, index) => (
        <div key={index} className="space-y-2 rounded-md border p-3">
          <div className="flex flex-wrap gap-1.5">
            {pushWeekdays.map((weekday) => (
              <label key={weekday} className="flex items-center gap-1 text-xs">
                <Checkbox
                  checked={range.weekdays.includes(weekday)}
                  onCheckedChange={() =>
                    update(index, {
                      ...range,
                      weekdays: toggleListValue(range.weekdays, weekday),
                    })
                  }
                />
                {t(weekdayKeys[weekday])}
              </label>
            ))}
          </div>
          <div className="flex items-center gap-2">
            <Input
              type="time"
              aria-label={t("notifications.pushRangeStart")}
              value={range.start}
              onChange={(event) =>
                update(index, { ...range, start: event.target.value })
              }
            />
            <span className="text-muted-foreground">–</span>
            <Input
              type="time"
              aria-label={t("notifications.pushRangeEnd")}
              value={range.end}
              onChange={(event) =>
                update(index, { ...range, end: event.target.value })
              }
            />
            <Button
              type="button"
              size="icon-sm"
              variant="ghost"
              aria-label={t("notifications.pushRemoveRange")}
              onClick={() =>
                onChange(ranges.filter((_, position) => position !== index))
              }
            >
              <Trash2 />
            </Button>
          </div>
        </div>
      ))}
      <Button
        type="button"
        size="sm"
        variant="outline"
        onClick={() =>
          onChange([
            ...ranges,
            {
              weekdays: [
                PushNotificationWeekday.Monday,
                PushNotificationWeekday.Tuesday,
                PushNotificationWeekday.Wednesday,
                PushNotificationWeekday.Thursday,
                PushNotificationWeekday.Friday,
              ],
              start: "09:00",
              end: "17:00",
            },
          ])
        }
      >
        <Plus /> {t("notifications.pushAddRange")}
      </Button>
    </fieldset>
  );
}

function ServiceOverridesEditor({
  overrides,
  availableServices,
  serviceNames,
  disabled,
  onChange,
}: {
  overrides: PushNotificationServiceOverrideInput[];
  availableServices: Array<{ id: string; name: string }>;
  serviceNames: Map<string, string>;
  disabled: boolean;
  onChange: (overrides: PushNotificationServiceOverrideInput[]) => void;
}) {
  const { t } = useTranslations();
  function update(index: number, value: PushNotificationServiceOverrideInput) {
    onChange(
      overrides.map((item, position) => (position === index ? value : item)),
    );
  }
  return (
    <fieldset className="space-y-3" disabled={disabled}>
      <div>
        <legend className="text-sm font-medium">
          {t("notifications.pushOverrides")}
        </legend>
        <p className="text-sm text-muted-foreground">
          {t("notifications.pushOverridesHint")}
        </p>
      </div>
      {overrides.map((override, index) => (
        <div
          key={override.serviceId}
          className="space-y-3 rounded-md border p-3"
        >
          <div className="flex items-center justify-between gap-2">
            <div>
              <p className="text-sm font-medium">
                {serviceNames.get(override.serviceId) ?? override.serviceId}
              </p>
              <p className="text-xs text-muted-foreground">
                {override.serviceId}
              </p>
            </div>
            <Button
              type="button"
              size="icon-sm"
              variant="ghost"
              aria-label={t("notifications.pushRemoveOverride")}
              onClick={() =>
                onChange(overrides.filter((_, position) => position !== index))
              }
            >
              <Trash2 />
            </Button>
          </div>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <label className="space-y-1 text-xs">
              {t("notifications.pushOverrideEnabled")}
              <NativeSelect
                value={
                  override.enabled == null
                    ? "inherit"
                    : override.enabled
                      ? "on"
                      : "off"
                }
                onChange={(event) =>
                  update(index, {
                    ...override,
                    enabled:
                      event.target.value === "inherit"
                        ? undefined
                        : event.target.value === "on",
                  })
                }
              >
                <option value="inherit">
                  {t("notifications.pushInherit")}
                </option>
                <option value="on">{t("notifications.pushOn")}</option>
                <option value="off">{t("notifications.pushOff")}</option>
              </NativeSelect>
            </label>
            <label className="space-y-1 text-xs">
              {t("notifications.pushMinimumUrgency")}
              <NativeSelect
                value={override.minimumUrgency ?? "inherit"}
                onChange={(event) =>
                  update(index, {
                    ...override,
                    minimumUrgency:
                      event.target.value === "inherit"
                        ? undefined
                        : (event.target.value as PushNotificationUrgency),
                  })
                }
              >
                <option value="inherit">
                  {t("notifications.pushInherit")}
                </option>
                {pushUrgencies.map((urgency) => (
                  <option key={urgency} value={urgency}>
                    {t(urgencyKeys[urgency])}
                  </option>
                ))}
              </NativeSelect>
            </label>
          </div>
          <label className="flex items-center gap-2 text-sm">
            <Checkbox
              checked={override.events != null}
              onCheckedChange={(checked) =>
                update(index, {
                  ...override,
                  events: checked ? [...(override.events ?? [])] : undefined,
                })
              }
            />
            {t("notifications.pushExactEvents")}
          </label>
          {override.events != null && (
            <div className="grid grid-cols-1 gap-1.5 sm:grid-cols-2">
              {pushEvents.map((event) => (
                <label key={event} className="flex items-center gap-2 text-xs">
                  <Checkbox
                    checked={override.events?.includes(event)}
                    onCheckedChange={() =>
                      update(index, {
                        ...override,
                        events: toggleListValue(override.events ?? [], event),
                      })
                    }
                  />
                  {t(eventKeys[event])}
                </label>
              ))}
            </div>
          )}
        </div>
      ))}
      {availableServices.length > 0 && (
        <NativeSelect
          aria-label={t("notifications.pushAddOverride")}
          value=""
          onChange={(event) => {
            if (!event.target.value) return;
            onChange([
              ...overrides,
              { serviceId: event.target.value, enabled: true },
            ]);
          }}
        >
          <option value="">{t("notifications.pushAddOverride")}</option>
          {availableServices.map((service) => (
            <option key={service.id} value={service.id}>
              {service.name} ({service.id})
            </option>
          ))}
        </NativeSelect>
      )}
    </fieldset>
  );
}
