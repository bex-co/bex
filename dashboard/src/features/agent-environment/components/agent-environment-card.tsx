import { useId, useState } from "react";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Alert, AlertDescription } from "@/common/components/ui/alert";
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
import { Textarea } from "@/common/components/ui/textarea";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCardRadioGroup } from "@/common/hooks/use-card-radio-group";
import { cn } from "@/common/lib/utils/utils";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  AgentNetworkTier,
  type AgentEnvironmentInput,
} from "@/graphql/definitions";
import { useAgentEnvironment } from "@/features/agent-environment/hooks/use-agent-environment";
import { ModelKeySection } from "@/features/agent-environment/components/model-key-section";
import { SpendCapSection } from "@/features/agent-environment/components/spend-cap-section";
import { AvailableAgentsSection } from "@/features/agent-environment/components/available-agents-section";
import { GithubCliSection } from "@/features/agent-environment/components/github-cli-section";
import { EnabledModelsSection } from "@/features/agent-environment/components/enabled-models-section";
import { IdleTimeoutSection } from "@/features/agent-environment/components/idle-timeout-section";
import { templateLabel } from "@/features/agent-sessions/lib/templates";
import {
  AgentEnvironmentCardSkeleton,
  AgentSectionHeading,
  TIER_OPTION_CLASS,
  TierOptionText,
} from "@/features/agent-environment/components/agent-environment-card-skeleton";
import {
  MAX_CUSTOM_HOSTS,
  TIERS,
  buildPatch,
  defaultModelUnoffered,
  draftFrom,
  draftProblems,
  hostsProblemMessage,
  parseHostLines,
  saveErrorFrom,
  type AgentEnvironmentDraft,
  type AgentEnvironmentView,
  type SaveError,
} from "@/features/agent-environment/lib/environment";

const TIER_VALUES = TIERS.map(({ tier }) => tier);

// The default-model picker's value for "no default": Radix Select refuses an
// empty item value, and "" is what the API stores for it.
const PLATFORM_DEFAULT = "__platform_default__";

/**
 * Settings → Agents (w10/m17/t005): the workspace agent environment — the
 * sandbox network tier, the Custom tier's extra hostnames, the sessions' gh
 * (w10/m524), the default model and the catalog models members can choose
 * (w10/m527), the default sandbox template (w10/m519, Devin's default
 * platform), on Enterprise the idle timeout of the sessions people start
 * (w6/m563), the model source with the write-only provider key (ADR091 D7),
 * and the agent-model spend against its monthly cap (ADR091 D5, D6). Any
 * member sees it; only admins (can_manage) can change it.
 */
export function AgentEnvironmentCard({ ownerId }: { ownerId: string }) {
  const { t } = useTranslations();
  const { denied } = useCapabilities();
  const { environment, loading, refetch, update, saving } =
    useAgentEnvironment(ownerId);

  if (loading) {
    return (
      <AgentEnvironmentCardSkeleton
        readOnly={denied("can_manage")}
        billingDenied={denied("can_view_billing")}
      />
    );
  }
  if (!environment) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("agentEnvironment.title")}</CardTitle>
        </CardHeader>
        <CardContent>
          <Alert variant="destructive">
            <AlertDescription className="flex flex-wrap items-center gap-3">
              <span>{t("agentEnvironment.loadFailed")}</span>
              <Button
                size="sm"
                variant="outline"
                onClick={() => void refetch()}
              >
                {t("agentEnvironment.retry")}
              </Button>
            </AlertDescription>
          </Alert>
        </CardContent>
      </Card>
    );
  }
  // Keyed by workspace so switching workspaces re-seeds the draft.
  return (
    <AgentEnvironmentForm
      key={environment.ownerId}
      saved={environment}
      update={update}
      saving={saving}
    />
  );
}

interface AgentEnvironmentFormProps {
  saved: AgentEnvironmentView;
  update: (input: AgentEnvironmentInput) => Promise<AgentEnvironmentView>;
  saving: boolean;
}

function AgentEnvironmentForm({
  saved,
  update,
  saving,
}: AgentEnvironmentFormProps) {
  const { t, i18n } = useTranslations();
  const { canManage, denied } = useCapabilities();
  const ids = useId();
  const hostsId = `${ids}-hosts`;
  const modelId = `${ids}-model`;
  const templateId = `${ids}-template`;

  const [draft, setDraft] = useState<AgentEnvironmentDraft>(() =>
    draftFrom(saved),
  );
  // Re-seed from a newer server copy (a save, or another admin's change seen
  // on refetch) only while the form has no unsaved edits of its own.
  const [base, setBase] = useState(saved);
  if (base !== saved) {
    setBase(saved);
    if (Object.keys(buildPatch(base, draft)).length === 0) {
      setDraft(draftFrom(saved));
    }
  }
  const [serverError, setServerError] = useState<SaveError | null>(null);

  const readOnly = !canManage;
  const patch = buildPatch(saved, draft);
  const dirty = Object.keys(patch).length > 0;
  const problems = draftProblems(draft);
  const valid = problems.hosts == null && !problems.model;
  const canSave = !readOnly && dirty && valid && !saving;

  function edit(
    next: Partial<AgentEnvironmentDraft>,
    clears: SaveError["field"][],
  ) {
    setDraft((prev) => ({ ...prev, ...next }));
    if (serverError && clears.includes(serverError.field)) setServerError(null);
  }

  const radio = useCardRadioGroup(
    TIER_VALUES,
    draft.networkTier,
    (tier) => edit({ networkTier: tier }, ["hosts", "form"]),
    readOnly,
  );

  async function handleSave() {
    if (!canSave) return;
    setServerError(null);
    try {
      const next = await update(patch);
      setBase(next);
      setDraft(draftFrom(next));
      toast.success(t("agentEnvironment.saved"));
    } catch (err) {
      setServerError(saveErrorFrom(err));
    }
  }

  const hostsProblem =
    problems.hosts ??
    (serverError?.field === "hosts" ? serverError.problem : null);
  const hostsMessage = hostsProblem ? hostsProblemMessage(hostsProblem) : null;
  const modelError = problems.model
    ? t("agentEnvironment.modelInvalid")
    : serverError?.field === "model"
      ? t(serverError.key, serverError.params)
      : null;
  const templateError =
    serverError?.field === "template"
      ? t(serverError.key, serverError.params)
      : null;
  const hostCount = parseHostLines(draft.hostsText).length;
  // A default an admin turned off, or one the catalog dropped, does not take
  // effect (w10/m527): it stays the picker's value, with a note naming the
  // catalog default that runs instead.
  const unofferedDefault = defaultModelUnoffered(saved, draft.defaultModel);
  const modelHelpIds = unofferedDefault
    ? `${modelId}-help ${modelId}-unoffered`
    : `${modelId}-help`;

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("agentEnvironment.title")}</CardTitle>
        <CardDescription>{t("agentEnvironment.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        {denied("can_manage") ? (
          <p className="text-muted-foreground text-sm" role="note">
            {t("agentEnvironment.readOnlyNote")}
          </p>
        ) : null}

        <div className="space-y-3">
          <AgentSectionHeading
            title={t("agentEnvironment.networkTitle")}
            description={t("agentEnvironment.networkDescription")}
          />
          <div
            role="radiogroup"
            aria-label={t("agentEnvironment.networkTitle")}
            className="grid grid-cols-1 gap-3"
          >
            {TIERS.map(({ tier, titleKey, descriptionKey }) => {
              const checked = draft.networkTier === tier;
              return (
                <button
                  key={tier}
                  {...radio(tier)}
                  type="button"
                  role="radio"
                  aria-checked={checked}
                  disabled={readOnly}
                  onClick={() => edit({ networkTier: tier }, ["hosts", "form"])}
                  className={cn(
                    TIER_OPTION_CLASS,
                    "text-left transition-colors disabled:cursor-not-allowed",
                    !readOnly && "hover:border-foreground/50",
                    checked
                      ? "border-primary ring-2 ring-primary"
                      : "border-border",
                  )}
                >
                  <TierOptionText
                    title={t(titleKey)}
                    description={t(descriptionKey)}
                  />
                </button>
              );
            })}
          </div>

          <details className="text-sm">
            <summary className="text-muted-foreground cursor-pointer">
              {t("agentEnvironment.registryHostsSummary", {
                count: saved.registryHosts.length,
              })}
            </summary>
            <ul className="mt-2 grid grid-cols-1 gap-1 font-mono text-xs sm:grid-cols-2">
              {saved.registryHosts.map((host) => (
                <li key={host}>{host}</li>
              ))}
            </ul>
          </details>

          {draft.networkTier === AgentNetworkTier.Custom ? (
            <div className="space-y-2">
              <Label htmlFor={hostsId}>
                {t("agentEnvironment.hostsLabel")}
              </Label>
              <Textarea
                id={hostsId}
                value={draft.hostsText}
                onChange={(e) => edit({ hostsText: e.target.value }, ["hosts"])}
                readOnly={readOnly}
                rows={4}
                spellCheck={false}
                autoCapitalize="off"
                autoComplete="off"
                placeholder="api.example.com"
                aria-invalid={hostsMessage != null}
                aria-describedby={`${hostsId}-help`}
                className="font-mono"
              />
              <p
                id={`${hostsId}-help`}
                className="text-muted-foreground flex justify-between gap-4 text-xs"
              >
                <span>
                  {t("agentEnvironment.hostsHelp", { max: MAX_CUSTOM_HOSTS })}
                </span>
                <span className="tabular-nums">
                  {hostCount}/{MAX_CUSTOM_HOSTS}
                </span>
              </p>
              {hostsMessage ? (
                <p className="text-destructive text-sm" role="alert">
                  {t(hostsMessage.key, hostsMessage.params)}
                </p>
              ) : null}
            </div>
          ) : null}
        </div>

        <GithubCliSection saved={saved} update={update} canManage={canManage} />

        <div className="space-y-2 border-t pt-6">
          <Label htmlFor={modelId}>{t("agentEnvironment.modelLabel")}</Label>
          {saved.models.length > 0 ? (
            <>
              {/* The platform model catalog (w10/m516): the default every
                  session and automation that names no model runs, one of
                  the models left on below (w10/m527); a member may still
                  pick another of those for one session. */}
              <Select
                value={draft.defaultModel || PLATFORM_DEFAULT}
                onValueChange={(value) =>
                  edit(
                    {
                      defaultModel: value === PLATFORM_DEFAULT ? "" : value,
                    },
                    ["model"],
                  )
                }
                disabled={readOnly}
              >
                <SelectTrigger
                  id={modelId}
                  aria-invalid={modelError != null}
                  aria-describedby={modelHelpIds}
                  className="w-full max-w-sm font-mono"
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={PLATFORM_DEFAULT} className="font-mono">
                    {t("agentEnvironment.modelPlatformDefault", {
                      model: saved.models[0],
                    })}
                  </SelectItem>
                  {saved.enabledModels.map((model) => (
                    <SelectItem key={model} value={model} className="font-mono">
                      {model}
                    </SelectItem>
                  ))}
                  {/* A default saved before an admin turned its model off,
                      or before it left the catalog, is still shown, so the
                      picker never misreads it. */}
                  {unofferedDefault ? (
                    <SelectItem
                      value={draft.defaultModel}
                      className="font-mono"
                    >
                      {unofferedDefault === "off"
                        ? t("agentEnvironment.modelOffOption", {
                            model: draft.defaultModel,
                          })
                        : draft.defaultModel}
                    </SelectItem>
                  ) : null}
                </SelectContent>
              </Select>
              <p
                id={`${modelId}-help`}
                className="text-muted-foreground text-xs"
              >
                {t("agentEnvironment.modelCatalogHelp")}
              </p>
              {unofferedDefault ? (
                <p
                  id={`${modelId}-unoffered`}
                  role="note"
                  className="text-xs text-amber-700 dark:text-amber-400"
                >
                  {t(
                    unofferedDefault === "off"
                      ? "agentEnvironment.modelDefaultOffNote"
                      : "agentEnvironment.modelDefaultRetiredNote",
                    { model: draft.defaultModel, fallback: saved.models[0] },
                  )}
                </p>
              ) : null}
            </>
          ) : (
            <>
              <Input
                id={modelId}
                value={draft.defaultModel}
                onChange={(e) =>
                  edit({ defaultModel: e.target.value }, ["model"])
                }
                readOnly={readOnly}
                placeholder={t("agentEnvironment.modelPlaceholder")}
                spellCheck={false}
                autoCapitalize="off"
                autoComplete="off"
                aria-invalid={modelError != null}
                aria-describedby={`${modelId}-help`}
                className="max-w-sm font-mono"
              />
              <p
                id={`${modelId}-help`}
                className="text-muted-foreground text-xs"
              >
                {t("agentEnvironment.modelHelp")}
              </p>
            </>
          )}
          {modelError ? (
            <p className="text-destructive text-sm" role="alert">
              {modelError}
            </p>
          ) : null}
        </div>

        {saved.models.length > 0 ? (
          <EnabledModelsSection
            saved={saved}
            update={update}
            canManage={canManage}
          />
        ) : null}

        {saved.templates.length > 0 ? (
          <div
            className="space-y-2 border-t pt-6"
            data-testid="default-template-section"
          >
            <Label htmlFor={templateId}>
              {t("agentEnvironment.templateLabel")}
            </Label>
            {/* The sandbox template every session and automation run that
                names none runs on (w10/m519); a member may still pick
                another for one session. */}
            <Select
              value={draft.defaultTemplate}
              onValueChange={(value) =>
                edit({ defaultTemplate: value }, ["template"])
              }
              disabled={readOnly}
            >
              <SelectTrigger
                id={templateId}
                aria-invalid={templateError != null}
                aria-describedby={`${templateId}-help`}
                className="w-full max-w-sm"
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {saved.templates.map((template) => (
                  <SelectItem key={template.name} value={template.name}>
                    {templateLabel(template, t, i18n.language)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <p
              id={`${templateId}-help`}
              className="text-muted-foreground text-xs"
            >
              {t("agentEnvironment.templateHelp")}
            </p>
            {templateError ? (
              <p className="text-destructive text-sm" role="alert">
                {templateError}
              </p>
            ) : null}
          </div>
        ) : null}

        {saved.idleTimeoutConfigurable ? (
          <IdleTimeoutSection
            saved={saved}
            update={update}
            canManage={canManage}
          />
        ) : null}

        <ModelKeySection saved={saved} update={update} canManage={canManage} />

        <SpendCapSection
          saved={saved}
          update={update}
          canManage={canManage}
          billingDenied={denied("can_view_billing")}
        />

        <AvailableAgentsSection
          saved={saved}
          update={update}
          canManage={canManage}
        />

        {serverError?.field === "form" ? (
          <Alert variant="destructive">
            <AlertDescription>
              {t(serverError.key, serverError.params)}
            </AlertDescription>
          </Alert>
        ) : null}
      </CardContent>
      <CardFooter className="justify-end border-t">
        <Button onClick={() => void handleSave()} disabled={!canSave}>
          {saving ? <Loader2 className="animate-spin" /> : null}
          {t("agentEnvironment.save")}
        </Button>
      </CardFooter>
    </Card>
  );
}
