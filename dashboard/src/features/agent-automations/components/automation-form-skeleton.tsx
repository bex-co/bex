import { Button } from "@/common/components/ui/button";
import { Label } from "@/common/components/ui/label";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { AutomationSubPageFrame } from "@/features/agent-automations/components/automation-frame";
import {
  FormField,
  FormSection,
} from "@/features/agent-automations/components/form-field";

/** A ready control's box as a bar: label and help are the real copy. */
const INPUT = <Skeleton className="h-9 w-full" />;

/**
 * /automations/new and the edit page while they load: the form's frame, real
 * section titles, labels and help (through the same FormField and FormSection
 * the ready form uses) and bars where controls go. It previews the form's
 * default state (a daily schedule); a stored automation with another frequency
 * swaps that section's inputs on load.
 */
export function AutomationFormSkeleton({ mode }: { mode: "create" | "edit" }) {
  const { t } = useTranslations();
  const title = t(
    mode === "create"
      ? "agentAutomations.createTitle"
      : "agentAutomations.editTitle",
  );
  return (
    <AutomationSubPageFrame
      title={title}
      backLabel={t("agentAutomations.backToList")}
      width="form"
    >
      <div
        aria-busy="true"
        data-skeleton-frame="true"
        data-route-skeleton={
          mode === "create" ? "automation-create" : "automation-edit"
        }
        className="space-y-8"
      >
        <div data-skeleton-region="details">
          <FormSection title={t("agentAutomations.section.details")}>
            <FormField
              id="sk-name"
              label={t("agentAutomations.field.name")}
              help={t("agentAutomations.help.name")}
            >
              {INPUT}
            </FormField>
          </FormSection>
        </div>

        <div data-skeleton-region="schedule">
          <FormSection
            title={t("agentAutomations.section.schedule")}
            description={t("agentAutomations.section.scheduleDescription")}
          >
            <div className="space-y-4">
              <FormField
                id="sk-frequency"
                label={t("agentAutomations.field.frequency")}
              >
                <Skeleton className="h-9 w-full sm:w-64" />
              </FormField>
              <FormField id="sk-at" label={t("agentAutomations.field.at")}>
                <Skeleton className="h-9 w-36" />
              </FormField>
              <div className="grid gap-4 sm:grid-cols-2">
                <FormField
                  id="sk-timezone"
                  label={t("agentAutomations.field.timezone")}
                >
                  {INPUT}
                </FormField>
                <FormField
                  id="sk-ends"
                  label={t("agentAutomations.field.endsOn")}
                  help={t("agentAutomations.help.endsOn")}
                >
                  {INPUT}
                </FormField>
              </div>
              <p className="min-h-5" />
            </div>
          </FormSection>
        </div>

        <div data-skeleton-region="session">
          <FormSection
            title={t("agentAutomations.section.session")}
            description={t("agentAutomations.section.sessionDescription")}
          >
            <FormField
              id="sk-task"
              label={t("agentAutomations.field.task")}
              help={t("agentAutomations.help.task")}
            >
              <Skeleton className="h-40 w-full" />
            </FormField>
            <FormField
              id="sk-repo"
              label={t("agentAutomations.field.repo")}
              help={t("agentAutomations.help.repo")}
            >
              {INPUT}
            </FormField>
            <div className="grid gap-4 sm:grid-cols-2">
              <FormField
                id="sk-agent"
                label={t("agentAutomations.field.agent")}
              >
                {INPUT}
              </FormField>
              <FormField
                id="sk-spend"
                label={t("agentAutomations.field.spendLimit")}
                help={t("agentAutomations.help.spendLimit")}
              >
                {INPUT}
              </FormField>
            </div>
            <div className="grid gap-4 sm:grid-cols-2">
              <FormField
                id="sk-email"
                label={t("agentAutomations.field.emailNotifications")}
                help={t("agentAutomations.help.emailNotifications")}
              >
                {INPUT}
              </FormField>
            </div>
            <div className="space-y-1.5">
              <Label>{t("agentAutomations.field.secrets")}</Label>
              <div className="flex flex-wrap items-center gap-2">
                <Skeleton className="size-8" />
                <Skeleton className="h-4 w-24" />
              </div>
              <p className="text-muted-foreground text-xs">
                {t("agentAutomations.help.secrets")}
              </p>
            </div>
            <div className="space-y-1.5">
              <Label>{t("agentAutomations.field.skills")}</Label>
              <div className="flex flex-wrap items-center gap-2">
                <Skeleton className="size-8" />
                <Skeleton className="h-4 w-24" />
              </div>
            </div>
          </FormSection>
        </div>

        <div
          data-skeleton-region="actions"
          className="flex flex-wrap justify-end gap-2 border-t pt-4"
        >
          <Button variant="outline" disabled>
            {t("common.cancel")}
          </Button>
          <Button disabled>
            {t(
              mode === "create"
                ? "agentAutomations.createSubmit"
                : "agentAutomations.editSubmit",
            )}
          </Button>
        </div>
      </div>
    </AutomationSubPageFrame>
  );
}

/** The create route's pending component. */
export function AutomationCreateSkeleton() {
  return <AutomationFormSkeleton mode="create" />;
}

/** The edit route's pending component. */
export function AutomationEditSkeleton() {
  return <AutomationFormSkeleton mode="edit" />;
}
