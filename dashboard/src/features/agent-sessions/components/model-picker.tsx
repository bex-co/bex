import { Cpu } from "lucide-react";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { cn } from "@/common/lib/utils/utils";
import { useTranslations } from "@/common/hooks/use-translations";
import type { AgentModelOffer } from "@/features/agent-sessions/types";
import { shortModel, shownModel } from "@/features/agent-sessions/lib/models";

export interface ModelPickerProps {
  /** The agent's offer from the capabilities read; nothing renders without one. */
  offer: AgentModelOffer | undefined;
  /** The chosen model; "" keeps `current`, else the offer's default. */
  value: string;
  /** The picked model, or "" when the pick is what already runs. */
  onChange: (model: string) => void;
  /** The model running now (a session's), shown while nothing is chosen. */
  current?: string | null;
  disabled?: boolean;
  /** A compact control in a composer toolbar, or a full-width form field. */
  variant?: "toolbar" | "field";
  id?: string;
  /** Extra classes for the control (a narrower width in a tight row). */
  className?: string;
  /** A form field's error and help wiring. */
  invalid?: boolean;
  describedBy?: string;
}

/**
 * The model control (w10/m516, Devin's agent selector): shows the model that
 * will run and, where the agent offers a choice (the platform model catalog),
 * lets the person pick another. An agent with one fixed model shows it without
 * a choice. On the workspace's own key the offer lists nothing and the
 * control is not drawn: the workspace default (Settings → Agents) applies.
 * The list is the capabilities read's, so a model an admin turned off
 * (w10/m527) is never offered; a saved one still shows as the value.
 */
export function ModelPicker({
  offer,
  value,
  onChange,
  current,
  disabled,
  variant = "toolbar",
  id,
  className,
  invalid,
  describedBy,
}: ModelPickerProps) {
  const { t } = useTranslations();
  if (!offer || offer.models.length === 0) return null;
  const shown = shownModel(offer, value, current);
  const baseline = current === undefined ? offer.model : current;
  const toolbar = variant === "toolbar";
  const label = t("agentSessions.modelLabel");

  // One offered model is shown without a choice, unless what is shown is a
  // model the agent no longer offers (turned off by an admin, w10/m527):
  // then the one it offers must stay pickable.
  if (offer.models.length === 1 && offer.models[0] === shown) {
    return (
      <span
        id={id}
        title={t("agentSessions.modelFixed", { model: shown })}
        aria-label={label}
        data-testid="model-picker-fixed"
        className={cn(
          "text-muted-foreground inline-flex items-center gap-1.5 truncate font-mono text-xs",
          toolbar ? "h-8 max-w-[10.5rem] px-2" : "h-9",
          className,
        )}
      >
        <Cpu aria-hidden className="size-3.5 shrink-0" />
        <span className="truncate">{toolbar ? shortModel(shown) : shown}</span>
      </span>
    );
  }

  return (
    <Select
      value={shown}
      onValueChange={(model) => onChange(model === baseline ? "" : model)}
      disabled={disabled}
    >
      <SelectTrigger
        id={id}
        size={toolbar ? "sm" : "default"}
        aria-label={label}
        aria-invalid={invalid || undefined}
        aria-describedby={describedBy}
        title={shown}
        data-testid="model-picker"
        className={cn(
          toolbar
            ? "h-8 w-[10.5rem] gap-1.5 border-0 bg-transparent font-mono text-xs shadow-none"
            : "w-full font-mono",
          className,
        )}
      >
        {toolbar ? <Cpu aria-hidden className="size-3.5 shrink-0" /> : null}
        <SelectValue>
          <span className="truncate">
            {toolbar ? shortModel(shown) : shown}
          </span>
        </SelectValue>
      </SelectTrigger>
      <SelectContent>
        {offer.models.map((model) => (
          <SelectItem key={model} value={model} className="font-mono">
            {model === offer.model
              ? t("agentSessions.modelDefaultOption", { model })
              : model}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
