import { useMemo } from "react";
import { useNavigate } from "@tanstack/react-router";
import { toast } from "sonner";
import { Alert, AlertDescription } from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { AutomationForm } from "@/features/agent-automations/components/automation-form";
import { AutomationFormSkeleton } from "@/features/agent-automations/components/automation-form-skeleton";
import { AutomationSubPageFrame } from "@/features/agent-automations/components/automation-frame";
import { AutomationLoadError } from "@/features/agent-automations/components/automation-load-state";
import {
  useAgentAutomation,
  useAutomationMutations,
} from "@/features/agent-automations/hooks/use-agent-automation";
import { useAutomationViewer } from "@/features/agent-automations/hooks/use-automation-viewer";
import {
  emptyFormValues,
  valuesFromAutomation,
} from "@/features/agent-automations/lib/form";

/** /automations/new (ADR101 D9). */
export function AutomationCreatePage() {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const mutations = useAutomationMutations();
  const viewer = useAutomationViewer();
  const initial = useMemo(() => emptyFormValues(), []);
  const denied = !viewer.resolving && !viewer.canCreate;

  return (
    <AutomationSubPageFrame
      title={t("agentAutomations.createTitle")}
      backLabel={t("agentAutomations.backToList")}
      width="form"
    >
      {denied ? (
        <Alert variant="destructive" role="alert">
          <AlertDescription>
            {t("agentAutomations.createDenied")}
          </AlertDescription>
        </Alert>
      ) : (
        <AutomationForm
          mode="create"
          initial={initial}
          cancelTo={{ to: "/automations" }}
          onSave={async (input) => {
            const created = await mutations.create(input);
            toast.success(
              t("agentAutomations.createdToast", { name: created.name }),
            );
            await navigate({
              to: "/automations/$automationId",
              params: { automationId: created.id },
            });
          }}
        />
      )}
    </AutomationSubPageFrame>
  );
}

/** /automations/<id>/edit: the creator's edit form. */
export function AutomationEditPage({ automationId }: { automationId: string }) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { automation, loading, error, refetch } =
    useAgentAutomation(automationId);
  const mutations = useAutomationMutations();
  const viewer = useAutomationViewer();
  const initial = useMemo(
    () => (automation ? valuesFromAutomation(automation) : null),
    [automation],
  );

  if (loading || (automation && viewer.resolving)) {
    return <AutomationFormSkeleton mode="edit" />;
  }
  if (!automation || !initial) {
    return (
      <AutomationSubPageFrame
        title={t("agentAutomations.editTitle")}
        backLabel={t("agentAutomations.backToList")}
        width="form"
      >
        <AutomationLoadError error={error} onRetry={() => void refetch()} />
      </AutomationSubPageFrame>
    );
  }
  if (!viewer.actionsFor(automation.createdBy).edit) {
    return (
      <AutomationSubPageFrame
        title={t("agentAutomations.editTitle")}
        backLabel={t("agentAutomations.backToList")}
        width="form"
      >
        <Alert role="alert" data-testid="automation-edit-denied">
          <AlertDescription className="flex flex-wrap items-center gap-3">
            <span>{t("agentAutomations.editCreatorOnly")}</span>
            <Button
              size="sm"
              variant="outline"
              onClick={() =>
                void navigate({
                  to: "/automations/$automationId",
                  params: { automationId: automation.id },
                })
              }
            >
              {t("agentAutomations.backToAutomation")}
            </Button>
          </AlertDescription>
        </Alert>
      </AutomationSubPageFrame>
    );
  }
  return (
    <AutomationSubPageFrame
      title={t("agentAutomations.editTitle")}
      backLabel={t("agentAutomations.backToList")}
      width="form"
    >
      <AutomationForm
        key={automation.version}
        mode="edit"
        initial={initial}
        version={automation.version}
        cancelTo={{ to: "/automations/$automationId", id: automation.id }}
        onSave={async (input) => {
          const updated = await mutations.update(automation.id, input);
          toast.success(
            t("agentAutomations.updatedToast", { name: updated.name }),
          );
          await navigate({
            to: "/automations/$automationId",
            params: { automationId: updated.id },
          });
        }}
      />
    </AutomationSubPageFrame>
  );
}
