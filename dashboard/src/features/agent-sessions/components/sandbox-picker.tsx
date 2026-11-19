import { Server } from "lucide-react";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { cn } from "@/common/lib/utils/utils";
import { useTranslations } from "@/common/hooks/use-translations";
import type { AgentTemplateOffer } from "@/features/agent-sessions/types";
import {
  namedTemplateLabel,
  templateLabel,
} from "@/features/agent-sessions/lib/templates";

// The "follow the workspace default" choice's value: Radix Select refuses an
// empty item value, and "" is what the API stores for it.
const WORKSPACE_DEFAULT = "__workspace_default__";

export interface SandboxPickerProps {
  /** The workspace's templates from the capabilities read; nothing renders without one. */
  offer: AgentTemplateOffer | undefined;
  /** The chosen template; "" runs the workspace default. */
  value: string;
  /** The picked template, or "" for the workspace default. */
  onChange: (template: string) => void;
  /**
   * How the workspace default is offered. "mark" (a session, which starts now)
   * lists it among the templates marked as the default, and picking it is "".
   * "follow" (an automation, which fires later) offers "Workspace default" as
   * its own first choice, resolved at each run, beside every template pinned
   * by name.
   */
  defaultChoice?: "mark" | "follow";
  disabled?: boolean;
  /** A compact control in a composer toolbar, or a full-width form field. */
  variant?: "toolbar" | "field";
  id?: string;
  /** A form field's error and help wiring. */
  invalid?: boolean;
  describedBy?: string;
}

/**
 * The sandbox control (w10/m519, Devin's Configuration → Virtual environment):
 * the template a session runs on, each with its size, the workspace default
 * preselected. The list is bex-api's; a workspace with no templates offers no
 * choice and the control is not drawn. A stored template the list no longer
 * carries is still shown, so the control never misreads it.
 */
export function SandboxPicker({
  offer,
  value,
  onChange,
  defaultChoice = "mark",
  disabled,
  variant = "toolbar",
  id,
  invalid,
  describedBy,
}: SandboxPickerProps) {
  const { t, i18n } = useTranslations();
  if (!offer || offer.templates.length === 0) return null;
  const { templates, defaultTemplate } = offer;
  const follow = defaultChoice === "follow";
  const toolbar = variant === "toolbar";
  const language = i18n.language;
  // The template the session runs on: the chosen one, else the default.
  const running = value || defaultTemplate;
  const selected = value === "" && follow ? WORKSPACE_DEFAULT : running;
  const label = (name: string) =>
    namedTemplateLabel(name, templates, t, language);
  const followLabel = follow
    ? t("agentSessions.sandboxWorkspaceDefault", {
        template: label(defaultTemplate),
      })
    : "";
  const shown =
    selected === WORKSPACE_DEFAULT
      ? followLabel
      : toolbar
        ? running
        : label(running);
  const stale = value !== "" && !templates.some((tpl) => tpl.name === value);

  return (
    <Select
      value={selected}
      onValueChange={(next) =>
        onChange(
          next === WORKSPACE_DEFAULT || (!follow && next === defaultTemplate)
            ? ""
            : next,
        )
      }
      disabled={disabled}
    >
      <SelectTrigger
        id={id}
        size={toolbar ? "sm" : "default"}
        aria-label={t("agentSessions.sandboxLabel")}
        aria-invalid={invalid || undefined}
        aria-describedby={describedBy}
        title={t("agentSessions.metaSandbox", { template: label(running) })}
        className={cn(
          toolbar
            ? // The composer skeleton holds this width (detail-skeletons.tsx).
              "h-8 w-[9.5rem] gap-1.5 border-0 bg-transparent font-mono text-xs shadow-none"
            : "w-full",
        )}
      >
        {toolbar ? <Server aria-hidden className="size-3.5 shrink-0" /> : null}
        <SelectValue>
          <span className="truncate">{shown}</span>
        </SelectValue>
      </SelectTrigger>
      <SelectContent>
        {follow ? (
          <SelectItem value={WORKSPACE_DEFAULT}>{followLabel}</SelectItem>
        ) : null}
        {templates.map((template) => (
          <SelectItem key={template.name} value={template.name}>
            {!follow && template.name === defaultTemplate
              ? t("agentSessions.sandboxDefaultOption", {
                  template: templateLabel(template, t, language),
                })
              : templateLabel(template, t, language)}
          </SelectItem>
        ))}
        {stale ? (
          <SelectItem value={value} className="font-mono">
            {value}
          </SelectItem>
        ) : null}
      </SelectContent>
    </Select>
  );
}
