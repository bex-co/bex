import { useId } from "react";
import { Label } from "@/common/components/ui/label";
import { Switch } from "@/common/components/ui/switch";
import { useTranslations } from "@/common/hooks/use-translations";
import type { AgentEnvironmentInput } from "@/graphql/definitions";
import {
  automationsGithubCliInput,
  githubCliAutomationsErrorKey,
  githubCliErrorKey,
  githubCliMandatoryErrorKey,
  type AgentEnvironmentView,
} from "@/features/agent-environment/lib/environment";
import { useToggleSave } from "@/features/agent-environment/hooks/use-toggle-save";
import { ToggleSaveAlert } from "@/features/agent-environment/components/toggle-save-alert";
import {
  SETTING_SELECT_CLASS,
  SettingSelectField,
  SettingSwitchRow,
} from "@/features/agent-environment/components/agent-environment-card-skeleton";
import { GithubCliPicker } from "@/features/agent-sessions/components/github-cli-picker";
import { describedBy } from "@/features/agent-automations/lib/aria";
import {
  githubCliChoice,
  githubCliMandatoryOff,
  type GithubCliChoice,
} from "@/features/agent-sessions/lib/github-cli";

interface GithubCliSectionProps {
  saved: AgentEnvironmentView;
  update: (input: AgentEnvironmentInput) => Promise<AgentEnvironmentView>;
  canManage: boolean;
}

const AUTOMATIONS_SAVED_KEYS: Record<GithubCliChoice, string> = {
  inherit: "agentEnvironment.githubCliAutomationsInherited",
  on: "agentEnvironment.githubCliAutomationsTurnedOn",
  off: "agentEnvironment.githubCliAutomationsTurnedOff",
};

/**
 * Settings → Agents' GitHub CLI section (w10/m524, a Devin security profile's
 * gh API access): on, a repo session's gh reaches its repository through
 * bex; off removes gh from every session in the workspace, running ones
 * included, while Git pushes and bex's pull request tools keep working.
 * Beside it (w10/m561, Devin's mandatory profile and automations default):
 * whether that off is mandatory, a floor no automation or session turns back
 * on, and the gh of the sessions automations start, which inherits the
 * switch unless an admin binds it on or off ("on" is locked under a
 * mandatory off). Each saves on change, as the available-agents switches do;
 * members see them disabled.
 */
export function GithubCliSection({
  saved,
  update,
  canManage,
}: GithubCliSectionProps) {
  const { t } = useTranslations();
  const base = useId();
  const id = `${base}-github-cli`;
  const mandatoryId = `${base}-github-cli-mandatory`;
  const automationsId = `${base}-github-cli-automations`;
  // The admin's choice while it saves; a refused save leaves the stored value.
  const workspace = useToggleSave<boolean>(update, githubCliErrorKey);
  const mandatory = useToggleSave<boolean>(update, githubCliMandatoryErrorKey);
  const automations = useToggleSave<GithubCliChoice>(
    update,
    githubCliAutomationsErrorKey,
  );
  const checked = workspace.pending ?? saved.githubCliEnabled;
  const mandatoryChecked = mandatory.pending ?? saved.githubCliMandatory;
  const automationsChoice =
    automations.pending ?? githubCliChoice(saved.automationsGithubCliEnabled);

  function toggle(on: boolean) {
    void workspace.save(on, { githubCliEnabled: on }, (next) =>
      t(
        next.githubCliEnabled
          ? "agentEnvironment.githubCliTurnedOn"
          : "agentEnvironment.githubCliTurnedOff",
      ),
    );
  }

  function toggleMandatory(on: boolean) {
    void mandatory.save(on, { githubCliMandatory: on }, (next) =>
      t(
        next.githubCliMandatory
          ? "agentEnvironment.githubCliMandatoryTurnedOn"
          : "agentEnvironment.githubCliMandatoryTurnedOff",
      ),
    );
  }

  function pickAutomations(choice: GithubCliChoice) {
    if (choice === automationsChoice) return;
    void automations.save(choice, automationsGithubCliInput(choice), (next) =>
      t(
        AUTOMATIONS_SAVED_KEYS[
          githubCliChoice(next.automationsGithubCliEnabled)
        ],
      ),
    );
  }

  return (
    <div className="space-y-3 border-t pt-6" data-testid="github-cli-section">
      <SettingSwitchRow
        label={
          <Label htmlFor={id}>{t("agentEnvironment.githubCliLabel")}</Label>
        }
        help={t("agentEnvironment.githubCliHelp")}
        helpId={`${id}-help`}
        control={
          <Switch
            id={id}
            checked={checked}
            disabled={!canManage || workspace.pending !== null}
            aria-describedby={`${id}-help`}
            onCheckedChange={toggle}
          />
        }
      />
      <ToggleSaveAlert errorKey={workspace.errorKey} />
      <SettingSwitchRow
        label={
          <Label htmlFor={mandatoryId}>
            {t("agentEnvironment.githubCliMandatoryLabel")}
          </Label>
        }
        help={t("agentEnvironment.githubCliMandatoryHelp")}
        helpId={`${mandatoryId}-help`}
        control={
          <Switch
            id={mandatoryId}
            checked={mandatoryChecked}
            disabled={!canManage || mandatory.pending !== null}
            aria-describedby={`${mandatoryId}-help`}
            onCheckedChange={toggleMandatory}
          />
        }
      />
      <ToggleSaveAlert errorKey={mandatory.errorKey} />
      <SettingSelectField
        label={
          <Label htmlFor={automationsId}>
            {t("agentEnvironment.githubCliAutomationsLabel")}
          </Label>
        }
        control={
          <GithubCliPicker
            id={automationsId}
            variant="field"
            className={SETTING_SELECT_CLASS}
            value={automationsChoice}
            onChange={pickAutomations}
            inherited={saved.githubCliEnabled}
            onLock={githubCliMandatoryOff(saved) ? "mandatory" : null}
            disabled={!canManage || automations.pending !== null}
            invalid={automations.errorKey != null}
            describedBy={describedBy(automationsId, {
              hasError: automations.errorKey != null,
              hasHelp: true,
            })}
          />
        }
        help={t("agentEnvironment.githubCliAutomationsHelp")}
        helpId={`${automationsId}-help`}
        error={
          <ToggleSaveAlert
            errorKey={automations.errorKey}
            id={`${automationsId}-error`}
          />
        }
      />
    </div>
  );
}
