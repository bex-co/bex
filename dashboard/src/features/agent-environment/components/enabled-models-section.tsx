import { useId } from "react";
import { Label } from "@/common/components/ui/label";
import { Switch } from "@/common/components/ui/switch";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils";
import type { AgentEnvironmentInput } from "@/graphql/definitions";
import {
  enabledModelsErrorKey,
  MODEL_FILTERS,
  modelFilterOf,
  providerModels,
  providersOfEnabledModels,
  withModelToggled,
  withProviderToggled,
  type AgentEnvironmentView,
  type ModelFilter,
} from "@/features/agent-environment/lib/environment";
import {
  AgentSectionHeading,
  MODEL_FILTER_OPTION_CLASS,
  MODEL_LABEL_CLASS,
  ModelFilterFrame,
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
 * Settings → Agents' model list (w10/m527, Devin's allowlisted models): the
 * platform catalog models members may pick for a session, a message or an
 * automation, filtered by model or by provider, one filter type at a time as
 * Devin's (w10/m573).
 *
 * By model, one switch per catalog model, on while members may pick it. The
 * catalog's default (models[0]) stays on and locked, since sessions fall back
 * to it. A model off only because it joined the catalog after an admin
 * narrowed the list (newModels, w10/m568) is labelled "New in the catalog"
 * until a switch turns it on.
 *
 * By provider, one switch per catalog provider (a model id's prefix before
 * "/"), on while members may pick every model of that provider, a later one
 * included; each lists its models, the default marked always on. The last
 * provider allowed is locked on, since the server refuses an empty filter.
 * Switching to "By provider" allows the providers of the models on now, and
 * switching back keeps the models on now as the list.
 *
 * Each control saves on change, as the available-agents switches do; members
 * see them disabled. Drawn only on the platform catalog: on the workspace's
 * own key any model id stands and there is no list.
 */
export function EnabledModelsSection({
  saved,
  update,
  canManage,
}: EnabledModelsSectionProps) {
  const { t } = useTranslations();
  const ids = useId();
  // The filter while it saves; a refused save leaves the stored one.
  const { pending, errorKey, save } = useToggleSave<ModelFilter>(
    update,
    enabledModelsErrorKey,
  );
  const shown = pending ?? modelFilterOf(saved);
  const locked = !canManage || pending !== null;
  const helpId = `${ids}-filter-help`;

  function chooseFilter(byProvider: boolean) {
    if (byProvider === shown.byProvider) return;
    if (byProvider) {
      const providers = providersOfEnabledModels(saved);
      void save(
        { byProvider, models: saved.enabledModels, providers },
        { enabledProviders: providers },
        () => t("agentEnvironment.modelsFilteredByProvider"),
      );
      return;
    }
    void save(
      { byProvider, models: saved.enabledModels, providers: [] },
      { enabledModels: saved.enabledModels },
      () => t("agentEnvironment.modelsFilteredByModel"),
    );
  }

  function toggleModel(model: string, on: boolean) {
    const list = withModelToggled(saved, model, on);
    void save({ ...shown, models: list }, { enabledModels: list }, (next) =>
      t(
        next.enabledModels.includes(model)
          ? "agentEnvironment.modelsTurnedOn"
          : "agentEnvironment.modelsTurnedOff",
        { model },
      ),
    );
  }

  function toggleProvider(provider: string, on: boolean) {
    const list = withProviderToggled(saved, provider, on);
    void save(
      { ...shown, providers: list },
      { enabledProviders: list },
      (next) =>
        t(
          next.enabledProviders?.includes(provider)
            ? "agentEnvironment.modelsProviderTurnedOn"
            : "agentEnvironment.modelsProviderTurnedOff",
          { provider },
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
      <ModelFilterFrame
        role="radiogroup"
        aria-label={t("agentEnvironment.modelsFilterLabel")}
        aria-describedby={helpId}
      >
        {MODEL_FILTERS.map(({ byProvider, labelKey }) => (
          <label
            key={labelKey}
            className={cn(
              MODEL_FILTER_OPTION_CLASS,
              "text-muted-foreground hover:text-foreground cursor-pointer transition-colors",
              "has-checked:bg-primary has-checked:text-primary-foreground",
              "has-focus-visible:ring-ring has-focus-visible:ring-2",
              "has-disabled:hover:text-muted-foreground has-disabled:cursor-not-allowed has-disabled:opacity-50",
            )}
          >
            <input
              type="radio"
              className="sr-only"
              name={`${ids}-filter`}
              checked={shown.byProvider === byProvider}
              disabled={locked}
              onChange={() => chooseFilter(byProvider)}
            />
            {t(labelKey)}
          </label>
        ))}
      </ModelFilterFrame>
      <p id={helpId} className="text-muted-foreground text-xs">
        {t(
          shown.byProvider
            ? "agentEnvironment.modelsByProviderHelp"
            : "agentEnvironment.modelsByModelHelp",
        )}
      </p>
      <SwitchList>
        {shown.byProvider
          ? saved.providers.map((provider, i) => {
              const id = `${ids}-provider-${i}`;
              const checked = shown.providers.includes(provider);
              const last = checked && shown.providers.length === 1;
              const models = providerModels(saved, provider)
                .map((m) =>
                  m === saved.models[0]
                    ? t("agentEnvironment.modelsProviderDefault", { model: m })
                    : m,
                )
                .join(", ");
              const noteId = `${id}-note`;
              return (
                <SwitchListRow
                  key={provider}
                  label={
                    <Label htmlFor={id} className={MODEL_LABEL_CLASS}>
                      {provider}
                    </Label>
                  }
                  note={
                    <>
                      <span className="block [overflow-wrap:anywhere]">
                        {models}
                      </span>
                      {last ? (
                        <span className="block">
                          {t("agentEnvironment.modelsProviderLastLocked")}
                        </span>
                      ) : null}
                    </>
                  }
                  noteId={noteId}
                  control={
                    <Switch
                      id={id}
                      checked={checked}
                      disabled={last || locked}
                      aria-describedby={noteId}
                      onCheckedChange={(on) => toggleProvider(provider, on)}
                    />
                  }
                />
              );
            })
          : saved.models.map((model, i) => {
              const id = `${ids}-model-${i}`;
              const isDefault = i === 0;
              const checked = isDefault || shown.models.includes(model);
              const note = isDefault
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
                      disabled={isDefault || locked}
                      aria-describedby={note ? noteId : undefined}
                      onCheckedChange={(on) => toggleModel(model, on)}
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
