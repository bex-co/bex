import { useId } from "react";
import { Label } from "@/common/components/ui/label";
import { Switch } from "@/common/components/ui/switch";
import { useTranslations } from "@/common/hooks/use-translations";
import type { AgentEnvironmentInput } from "@/graphql/definitions";
import {
  enabledModelsErrorKey,
  withModelToggled,
  type AgentEnvironmentView,
} from "@/features/agent-environment/lib/environment";
import {
  AgentSectionHeading,
  MODEL_LABEL_CLASS,
  SwitchList,
  SwitchListRow,
} from "@/features/agent-environment/components/agent-environment-card-skeleton";
import { useToggleSave } from "@/features/agent-environment/hooks/use-toggle-save";
import { ToggleSaveAlert } from "@/features/agent-environment/components/toggle-save-alert";

interface EnabledModelsSectionProps {
  saved: AgentEnvironmentView;
  update: (input: AgentEnvironmentInput) => Promise<AgentEnvironmentView>;
  canManage: boolean;
}

/**
 * Settings → Agents' model list (w10/m527, Devin's allowlisted models): one
 * switch per platform catalog model, on while members may pick it for a
 * session, a message or an automation. The catalog's default (models[0])
 * stays on and locked, since sessions fall back to it. A model off only
 * because it joined the catalog after an admin narrowed the list (newModels,
 * w10/m568) is labelled "New in the catalog" until a switch turns it on.
 * Each switch saves the whole list on toggle, as the available-agents
 * switches do; members see them disabled. Drawn only on the platform catalog:
 * on the workspace's own key any model id stands and there is no list.
 */
export function EnabledModelsSection({
  saved,
  update,
  canManage,
}: EnabledModelsSectionProps) {
  const { t } = useTranslations();
  const ids = useId();
  // The list while it saves; a refused save leaves the stored list.
  const { pending, errorKey, save } = useToggleSave<string[]>(
    update,
    enabledModelsErrorKey,
  );
  const enabled = pending ?? saved.enabledModels;

  function toggle(model: string, on: boolean) {
    const list = withModelToggled(saved, model, on);
    void save(list, { enabledModels: list }, (next) =>
      t(
        next.enabledModels.includes(model)
          ? "agentEnvironment.modelsTurnedOn"
          : "agentEnvironment.modelsTurnedOff",
        { model },
      ),
    );
  }

  return (
    <div
      className="space-y-3 border-t pt-6"
      data-testid="enabled-models-section"
    >
      <AgentSectionHeading
        title={t("agentEnvironment.modelsTitle")}
        description={t("agentEnvironment.modelsDescription")}
      />
      <SwitchList>
        {saved.models.map((model, i) => {
          const id = `${ids}-model-${i}`;
          const locked = i === 0;
          const checked = locked || enabled.includes(model);
          const note = locked
            ? t("agentEnvironment.modelsDefaultLocked")
            : !checked && saved.newModels.includes(model)
              ? t("agentEnvironment.modelsNew")
              : null;
          const noteId = `${id}-note`;
          return (
            <SwitchListRow
              key={model}
              label={
                <Label htmlFor={id} className={MODEL_LABEL_CLASS}>
                  {model}
                </Label>
              }
              note={note}
              noteId={noteId}
              control={
                <Switch
                  id={id}
                  checked={checked}
                  disabled={locked || !canManage || pending !== null}
                  aria-describedby={note ? noteId : undefined}
                  onCheckedChange={(on) => toggle(model, on)}
                />
              }
            />
          );
        })}
      </SwitchList>
      <ToggleSaveAlert errorKey={errorKey} />
    </div>
  );
}
