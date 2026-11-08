import { useId } from "react";
import { Button } from "@/common/components/ui/button";
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
import { FormField } from "@/features/agent-automations/components/form-field";
import { describedBy } from "@/features/agent-automations/lib/aria";
import {
  describeDraft,
  FREQUENCIES,
  timeZoneOptions,
  WEEKDAYS,
  type Frequency,
  type ScheduleDraft,
  type ScheduleProblems,
  type Weekday,
} from "@/features/agent-automations/lib/schedule";

interface SchedulePickerProps {
  value: ScheduleDraft;
  onChange: (next: ScheduleDraft) => void;
  /** Client problems (locale keys) and the server's messages, by field. */
  problems: ScheduleProblems;
  serverErrors: Partial<Record<keyof ScheduleProblems, string>>;
  /** A server message about the schedule as a whole. */
  serverScheduleError?: string | null;
  disabled?: boolean;
}

/**
 * The schedule trigger's picker (ADR101 D2): a frequency, the inputs that
 * frequency owns, a required IANA time zone defaulting to the browser's, an
 * optional end date, and the schedule in words beneath. Each frequency reveals
 * only its own inputs, matching what the server accepts.
 */
export function SchedulePicker({
  value,
  onChange,
  problems,
  serverErrors,
  serverScheduleError,
  disabled,
}: SchedulePickerProps) {
  const { t, i18n } = useTranslations();
  const base = useId();
  const patch = (next: Partial<ScheduleDraft>) =>
    onChange({ ...value, ...next });
  const errorFor = (field: keyof ScheduleProblems) =>
    serverErrors[field] ??
    (problems[field] ? t(problems[field] as string) : null);
  const summary = describeDraft(value, t, i18n.language);
  const zones = timeZoneOptions(value.timezone);
  const id = (name: string) => `${base}-${name}`;

  return (
    <div className="space-y-4">
      <FormField
        id={id("frequency")}
        label={t("agentAutomations.field.frequency")}
        error={errorFor("frequency")}
      >
        <Select
          value={value.frequency}
          onValueChange={(next) => patch({ frequency: next as Frequency })}
          disabled={disabled}
        >
          <SelectTrigger
            id={id("frequency")}
            className="w-full sm:w-64"
            aria-invalid={errorFor("frequency") != null}
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {FREQUENCIES.map((frequency) => (
              <SelectItem key={frequency} value={frequency}>
                {t(`agentAutomations.frequency.${frequency}`)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </FormField>

      {value.frequency === "hour" ? (
        <FormField
          id={id("minute")}
          label={t("agentAutomations.field.minute")}
          help={t("agentAutomations.help.minute")}
          error={errorFor("minute")}
        >
          <Input
            id={id("minute")}
            type="number"
            inputMode="numeric"
            min={0}
            max={59}
            value={value.minute}
            onChange={(e) => patch({ minute: e.target.value })}
            disabled={disabled}
            className="w-28"
            aria-invalid={errorFor("minute") != null}
            aria-describedby={describedBy(id("minute"), {
              hasError: errorFor("minute") != null,
              hasHelp: true,
            })}
          />
        </FormField>
      ) : null}

      {value.frequency === "day" || value.frequency === "week" ? (
        <FormField
          id={id("at")}
          label={t("agentAutomations.field.at")}
          error={errorFor("at")}
        >
          <Input
            id={id("at")}
            type="time"
            value={value.at}
            onChange={(e) => patch({ at: e.target.value })}
            disabled={disabled}
            className="w-36"
            aria-invalid={errorFor("at") != null}
            aria-describedby={describedBy(id("at"), {
              hasError: errorFor("at") != null,
              hasHelp: false,
            })}
          />
        </FormField>
      ) : null}

      {value.frequency === "week" ? (
        <WeekdayToggles
          id={id("weekdays")}
          value={value.weekdays}
          onChange={(weekdays) => patch({ weekdays })}
          error={errorFor("weekdays")}
          disabled={disabled}
        />
      ) : null}

      {value.frequency === "once" ? (
        <FormField
          id={id("runAt")}
          label={t("agentAutomations.field.runAt")}
          help={t("agentAutomations.help.runAt", { zone: value.timezone })}
          error={errorFor("runAt")}
        >
          <Input
            id={id("runAt")}
            type="datetime-local"
            value={value.runAt}
            onChange={(e) => patch({ runAt: e.target.value })}
            disabled={disabled}
            className="w-full sm:w-64"
            aria-invalid={errorFor("runAt") != null}
            aria-describedby={describedBy(id("runAt"), {
              hasError: errorFor("runAt") != null,
              hasHelp: true,
            })}
          />
        </FormField>
      ) : null}

      {value.frequency === "custom" ? (
        <FormField
          id={id("cron")}
          label={t("agentAutomations.field.cron")}
          help={t("agentAutomations.help.cron")}
          error={errorFor("cron")}
        >
          <Input
            id={id("cron")}
            value={value.cron}
            onChange={(e) => patch({ cron: e.target.value })}
            disabled={disabled}
            placeholder="0 9 * * 1-5"
            spellCheck={false}
            autoCapitalize="off"
            autoComplete="off"
            className="font-mono"
            aria-invalid={errorFor("cron") != null}
            aria-describedby={describedBy(id("cron"), {
              hasError: errorFor("cron") != null,
              hasHelp: true,
            })}
          />
        </FormField>
      ) : null}

      <div className="grid gap-4 sm:grid-cols-2">
        <FormField
          id={id("timezone")}
          label={t("agentAutomations.field.timezone")}
          error={errorFor("timezone")}
        >
          <Select
            value={value.timezone}
            onValueChange={(timezone) => patch({ timezone })}
            disabled={disabled}
          >
            <SelectTrigger
              id={id("timezone")}
              className="w-full"
              aria-invalid={errorFor("timezone") != null}
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent className="max-h-72">
              {zones.map((zone) => (
                <SelectItem key={zone} value={zone}>
                  {zone}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </FormField>

        {value.frequency !== "once" ? (
          <FormField
            id={id("endsOn")}
            label={t("agentAutomations.field.endsOn")}
            help={t("agentAutomations.help.endsOn")}
            error={errorFor("endsOn")}
          >
            <Input
              id={id("endsOn")}
              type="date"
              value={value.endsOn}
              onChange={(e) => patch({ endsOn: e.target.value })}
              disabled={disabled}
              aria-invalid={errorFor("endsOn") != null}
              aria-describedby={describedBy(id("endsOn"), {
                hasError: errorFor("endsOn") != null,
                hasHelp: true,
              })}
            />
          </FormField>
        ) : null}
      </div>

      {serverScheduleError ? (
        <p role="alert" className="text-destructive text-sm">
          {serverScheduleError}
        </p>
      ) : null}
      <p
        className="text-muted-foreground min-h-5 text-sm"
        aria-live="polite"
        data-testid="schedule-summary"
      >
        {summary ? t("agentAutomations.summary", { schedule: summary }) : ""}
      </p>
    </div>
  );
}

function WeekdayToggles({
  id,
  value,
  onChange,
  error,
  disabled,
}: {
  id: string;
  value: Weekday[];
  onChange: (next: Weekday[]) => void;
  error: string | null;
  disabled?: boolean;
}) {
  const { t } = useTranslations();
  return (
    <div className="space-y-1.5">
      <Label id={`${id}-label`}>{t("agentAutomations.field.weekdays")}</Label>
      <div
        role="group"
        aria-labelledby={`${id}-label`}
        aria-describedby={error ? `${id}-error` : undefined}
        className="flex flex-wrap gap-1.5"
      >
        {WEEKDAYS.map((day) => {
          const on = value.includes(day);
          return (
            <Button
              key={day}
              type="button"
              size="sm"
              variant={on ? "default" : "outline"}
              aria-pressed={on}
              disabled={disabled}
              onClick={() =>
                onChange(on ? value.filter((d) => d !== day) : [...value, day])
              }
            >
              {t(`agentAutomations.weekday.${day}`)}
            </Button>
          );
        })}
      </div>
      {error ? (
        <p id={`${id}-error`} role="alert" className="text-destructive text-sm">
          {error}
        </p>
      ) : null}
    </div>
  );
}
