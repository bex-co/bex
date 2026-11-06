import { useId } from "react";
import { Label } from "@/common/components/ui/label";
import { Switch } from "@/common/components/ui/switch";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCardRadioGroup } from "@/common/hooks/use-card-radio-group";
import { cn } from "@/common/lib/utils/utils";
import {
  APPROVAL_BYPASS_ROW_CLASS,
  AgentSectionHeading,
  TIER_OPTION_CLASS,
  TierOptionText,
} from "@/features/agent-environment/components/agent-environment-card-skeleton";
import {
  APPROVAL_MODES,
  APPROVAL_MODE_COPY,
  type ApprovalMode,
} from "@/features/agent-environment/lib/approval";

export interface ApprovalModeSectionProps {
  mode: ApprovalMode;
  bypassAllowed: boolean;
  onModeChange: (mode: ApprovalMode) => void;
  onBypassAllowedChange: (allowed: boolean) => void;
  readOnly: boolean;
}

/**
 * Settings → Agents: when cloud agents pause to ask (ADR100 D3), Devin's
 * approval workflow. Autonomous never asks; Risky asks before destructive,
 * publishing and infrastructure actions; All asks before every action that
 * changes something. The switch decides whether a member may start a session
 * that asks less than this, or bypasses approval. Saved with the card.
 */
export function ApprovalModeSection({
  mode,
  bypassAllowed,
  onModeChange,
  onBypassAllowedChange,
  readOnly,
}: ApprovalModeSectionProps) {
  const { t } = useTranslations();
  const bypassId = useId();
  const radio = useCardRadioGroup(APPROVAL_MODES, mode, onModeChange, readOnly);
  return (
    <div
      className="space-y-3 border-t pt-6"
      data-testid="approval-mode-section"
    >
      <AgentSectionHeading
        title={t("agentEnvironment.approvalTitle")}
        description={t("agentEnvironment.approvalDescription")}
      />
      <div
        role="radiogroup"
        aria-label={t("agentEnvironment.approvalTitle")}
        className="grid grid-cols-1 gap-3"
      >
        {APPROVAL_MODES.map((option) => {
          const checked = mode === option;
          const copy = APPROVAL_MODE_COPY[option];
          return (
            <button
              key={option}
              {...radio(option)}
              type="button"
              role="radio"
              aria-checked={checked}
              disabled={readOnly}
              onClick={() => onModeChange(option)}
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
                title={t(copy.titleKey)}
                description={t(copy.descriptionKey)}
              />
            </button>
          );
        })}
      </div>
      <div className={APPROVAL_BYPASS_ROW_CLASS}>
        <Switch
          id={bypassId}
          checked={bypassAllowed}
          onCheckedChange={onBypassAllowedChange}
          disabled={readOnly}
          aria-describedby={`${bypassId}-help`}
        />
        <div className="space-y-1">
          <Label htmlFor={bypassId}>
            {t("agentEnvironment.approvalBypassLabel")}
          </Label>
          <p id={`${bypassId}-help`} className="text-muted-foreground text-xs">
            {t("agentEnvironment.approvalBypassHelp")}
          </p>
        </div>
      </div>
    </div>
  );
}
