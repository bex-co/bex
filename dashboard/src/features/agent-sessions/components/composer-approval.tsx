import { Check, Loader2, ShieldCheck } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/common/components/ui/popover";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils";
import { useAgentEnvironment } from "@/features/agent-environment/hooks/use-agent-environment";
import {
  APPROVAL_MODES,
  APPROVAL_MODE_COPY,
  parseApprovalMode,
  sessionModeAllowed,
  type ApprovalMode,
} from "@/features/agent-environment/lib/approval";

/**
 * A per-session approval choice (ADR100 D3), with the workspace mode it was
 * made against so the create can tell a raise from a bypass. Null follows
 * the workspace.
 */
export interface ComposerApprovalChoice {
  mode: ApprovalMode;
  workspaceMode: ApprovalMode;
}

interface ComposerApprovalProps {
  ownerId: string | null;
  value: ComposerApprovalChoice | null;
  onChange: (value: ComposerApprovalChoice | null) => void;
  disabled?: boolean;
}

/**
 * The composer's approval control (ADR100 D6): Devin's per-session approval
 * mode and bypass_approval. A fixed-size icon button on the toolbar, like
 * Secrets and Skills, so the prompt box keeps its skeleton geometry; the
 * workspace setting loads only when the popover opens. Modes the workspace
 * does not let a member pick are listed but disabled.
 */
export function ComposerApproval({
  ownerId,
  value,
  onChange,
  disabled,
}: ComposerApprovalProps) {
  const { t } = useTranslations();
  const label = value
    ? t("agentSessions.approvalButtonMode", {
        mode: t(APPROVAL_MODE_COPY[value.mode].titleKey),
      })
    : t("agentSessions.approvalButton");
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button
          type="button"
          variant={value ? "secondary" : "ghost"}
          size="icon"
          className="size-8"
          disabled={disabled}
          aria-label={label}
          title={label}
          data-testid="agent-composer-approval"
        >
          <ShieldCheck className="size-4" />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-80 p-3">
        <ApprovalChoices ownerId={ownerId} value={value} onChange={onChange} />
      </PopoverContent>
    </Popover>
  );
}

function ApprovalChoices({
  ownerId,
  value,
  onChange,
}: Pick<ComposerApprovalProps, "ownerId" | "value" | "onChange">) {
  const { t } = useTranslations();
  const { environment, loading } = useAgentEnvironment(ownerId);
  // A refetch keeps the options on screen: swapping them for the spinner
  // flickered the popover and dropped a pending click.
  if (loading && !environment) {
    return (
      <p className="text-muted-foreground flex items-center gap-2 text-sm">
        <Loader2 className="size-4 animate-spin" aria-hidden />
        {t("agentSessions.approvalLoading")}
      </p>
    );
  }
  const workspaceMode = parseApprovalMode(environment?.approvalMode);
  const bypassAllowed = environment?.approvalBypassAllowed ?? false;
  const selected = value?.mode ?? null;

  const option = (
    key: string,
    mode: ApprovalMode | null,
    title: string,
    description: string,
  ) => {
    const allowed =
      mode == null || sessionModeAllowed(mode, workspaceMode, bypassAllowed);
    const checked = selected === mode;
    return (
      <button
        key={key}
        type="button"
        role="radio"
        aria-checked={checked}
        disabled={!allowed}
        onClick={() => onChange(mode == null ? null : { mode, workspaceMode })}
        className={cn(
          "hover:bg-muted flex w-full items-start gap-2 rounded-md px-2 py-1.5 text-left disabled:cursor-not-allowed disabled:opacity-50",
          checked && "bg-muted",
        )}
      >
        <Check
          className={cn("mt-0.5 size-4 shrink-0", !checked && "invisible")}
          aria-hidden
        />
        <span className="min-w-0">
          <span className="block text-sm font-medium">{title}</span>
          <span className="text-muted-foreground block text-xs">
            {description}
          </span>
        </span>
      </button>
    );
  };

  return (
    <div className="space-y-2">
      <p className="text-sm font-medium">{t("agentSessions.approvalTitle")}</p>
      <div
        role="radiogroup"
        aria-label={t("agentSessions.approvalTitle")}
        className="space-y-0.5"
      >
        {option(
          "workspace",
          null,
          t("agentSessions.approvalWorkspaceDefault", {
            mode: t(APPROVAL_MODE_COPY[workspaceMode].titleKey),
          }),
          t(APPROVAL_MODE_COPY[workspaceMode].descriptionKey),
        )}
        {APPROVAL_MODES.filter((mode) => mode !== workspaceMode).map((mode) =>
          option(
            mode,
            mode,
            t(APPROVAL_MODE_COPY[mode].titleKey),
            t(APPROVAL_MODE_COPY[mode].descriptionKey),
          ),
        )}
      </div>
      {!bypassAllowed && workspaceMode !== "autonomous" ? (
        <p className="text-muted-foreground text-xs" role="note">
          {t("agentSessions.approvalBypassLocked")}
        </p>
      ) : null}
    </div>
  );
}
