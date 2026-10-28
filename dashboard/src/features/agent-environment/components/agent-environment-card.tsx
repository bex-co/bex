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
import {
  AgentEnvironmentCardSkeleton,
  TIER_OPTION_CLASS,
} from "@/features/agent-environment/components/agent-environment-card-skeleton";
import {
  MAX_CUSTOM_HOSTS,
  buildPatch,
  draftFrom,
  draftProblems,
  hostsProblemMessage,
  parseHostLines,
  saveErrorFrom,
  type AgentEnvironmentDraft,
  type AgentEnvironmentView,
  type SaveError,
} from "@/features/agent-environment/lib/environment";

const TIERS = [
  {
    tier: AgentNetworkTier.Registries,
    titleKey: "agentEnvironment.tierRegistriesTitle",
    descriptionKey: "agentEnvironment.tierRegistriesDescription",
  },
  {
    tier: AgentNetworkTier.Custom,
    titleKey: "agentEnvironment.tierCustomTitle",
    descriptionKey: "agentEnvironment.tierCustomDescription",
  },
  {
    tier: AgentNetworkTier.Full,
    titleKey: "agentEnvironment.tierFullTitle",
    descriptionKey: "agentEnvironment.tierFullDescription",
  },
] as const;

const TIER_VALUES = TIERS.map(({ tier }) => tier);

/**
 * Settings → Agents (w10/m17/t005): the workspace agent environment — the
 * sandbox network tier, the Custom tier's extra hostnames and the default
 * model. Any member sees it; only admins (can_manage) can change it.
 */
export function AgentEnvironmentCard({ ownerId }: { ownerId: string }) {
  const { t } = useTranslations();
  const { environment, loading, refetch, update, saving } =
    useAgentEnvironment(ownerId);

  if (loading) return <AgentEnvironmentCardSkeleton />;
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
  const { t } = useTranslations();
  const { canManage, denied } = useCapabilities();
  const ids = useId();
  const hostsId = `${ids}-hosts`;
  const modelId = `${ids}-model`;

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
  const hostCount = parseHostLines(draft.hostsText).length;

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
          <div className="space-y-1">
            <h3 className="text-sm font-medium">
              {t("agentEnvironment.networkTitle")}
            </h3>
            <p className="text-muted-foreground text-sm">
              {t("agentEnvironment.networkDescription")}
            </p>
          </div>
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
                  <span className="flex items-center gap-2 text-sm font-medium">
                    {t(titleKey)}
                  </span>
                  <span className="text-muted-foreground text-sm">
                    {t(descriptionKey)}
                  </span>
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

        <div className="space-y-2 border-t pt-6">
          <Label htmlFor={modelId}>{t("agentEnvironment.modelLabel")}</Label>
          {saved.modelPinned ? (
            <>
              <Input
                id={modelId}
                value={saved.pinnedModel ?? ""}
                readOnly
                aria-describedby={`${modelId}-help`}
                className="max-w-sm font-mono"
              />
              <p
                id={`${modelId}-help`}
                className="text-muted-foreground text-xs"
              >
                {t("agentEnvironment.modelPinnedHelp", {
                  model: saved.pinnedModel ?? "",
                })}
              </p>
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
