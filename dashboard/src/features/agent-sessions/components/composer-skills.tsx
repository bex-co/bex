import { useId } from "react";
import { Link } from "@tanstack/react-router";
import { Loader2, Sparkles, X } from "lucide-react";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import { Checkbox } from "@/common/components/ui/checkbox";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/common/components/ui/popover";
import { useTranslations } from "@/common/hooks/use-translations";
import { useAgentSkills } from "@/features/agent-environment/hooks/use-agent-skills";
import {
  MAX_TURN_SKILLS,
  isUserOnly,
  skillArgumentsProblem,
} from "@/features/agent-environment/lib/skills";
import type { ComposerSkill } from "@/features/agent-sessions/lib/composer-skills";

interface ComposerSkillsProps {
  ownerId: string | null;
  value: ComposerSkill[];
  onChange: (value: ComposerSkill[]) => void;
  disabled?: boolean;
}

/**
 * The composer's Skills control (ADR099 D4), Devin's playbook_id and
 * `@skills:name`: pick workspace skills the turn must follow, each with
 * optional arguments. A fixed-size icon button on the toolbar, so the prompt
 * box keeps its skeleton geometry; the workspace list loads only when opened.
 */
export function ComposerSkills({
  ownerId,
  value,
  onChange,
  disabled,
}: ComposerSkillsProps) {
  const { t } = useTranslations();
  const count = value.length;
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button
          type="button"
          variant={count > 0 ? "secondary" : "ghost"}
          size="icon"
          className="size-8"
          disabled={disabled}
          aria-label={
            count > 0
              ? t("agentSessions.skillsButtonCount", { count })
              : t("agentSessions.skillsButton")
          }
          title={t("agentSessions.skillsButton")}
          data-testid="agent-composer-skills"
        >
          <Sparkles className="size-4" />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-80 space-y-3">
        <SkillsPanel ownerId={ownerId} value={value} onChange={onChange} />
      </PopoverContent>
    </Popover>
  );
}

/**
 * The chosen skills as removable chips above the prompt. Nothing renders while
 * none is chosen, so the empty composer keeps its skeleton geometry.
 */
export function ComposerSkillChips({
  value,
  onChange,
  disabled,
}: Omit<ComposerSkillsProps, "ownerId">) {
  const { t } = useTranslations();
  if (value.length === 0) return null;
  return (
    <ul
      className="flex flex-wrap gap-1.5 px-2.5 pt-2"
      data-testid="agent-composer-skill-chips"
    >
      {value.map((skill) => (
        <li key={skill.name}>
          <Badge variant="secondary" className="gap-1 pr-0.5 font-mono">
            <Sparkles className="size-3" />
            <span className="max-w-48 truncate">
              {skill.name}
              {skill.arguments?.trim() ? ` ${skill.arguments.trim()}` : ""}
            </span>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="size-4"
              disabled={disabled}
              aria-label={t("agentSessions.skillsRemove", { name: skill.name })}
              onClick={() =>
                onChange(value.filter((v) => v.name !== skill.name))
              }
            >
              <X className="size-3" />
            </Button>
          </Badge>
        </li>
      ))}
    </ul>
  );
}

function SkillsPanel({
  ownerId,
  value,
  onChange,
}: Omit<ComposerSkillsProps, "disabled">) {
  const { t } = useTranslations();
  const ids = useId();
  const { skills, loading, error } = useAgentSkills(ownerId);
  const usable = skills.filter((skill) => skill.enabled);
  const full = value.length >= MAX_TURN_SKILLS;

  function toggle(name: string, checked: boolean) {
    onChange(
      checked
        ? [...value, { name }]
        : value.filter((skill) => skill.name !== name),
    );
  }

  function setArguments(name: string, args: string) {
    onChange(
      value.map((skill) =>
        skill.name === name ? { ...skill, arguments: args } : skill,
      ),
    );
  }

  const problem = value
    .map((skill) => skillArgumentsProblem(skill.arguments ?? ""))
    .find((p) => p != null);

  return (
    <>
      <div className="space-y-1">
        <p className="text-sm font-medium">{t("agentSessions.skillsTitle")}</p>
        <p className="text-muted-foreground text-xs">
          {t("agentSessions.skillsDescription")}
        </p>
      </div>
      {loading ? (
        <Loader2 className="text-muted-foreground size-4 animate-spin" />
      ) : error ? (
        <p className="text-destructive text-xs" role="alert">
          {t("agentSessions.skillsLoadFailed")}
        </p>
      ) : usable.length === 0 ? (
        <p className="text-muted-foreground text-xs">
          {t("agentSessions.skillsEmpty")}{" "}
          <Link
            to="/workspace/settings"
            hash="agents"
            className="underline underline-offset-4"
          >
            {t("agentSessions.skillsManage")}
          </Link>
        </p>
      ) : (
        <ul
          className="max-h-60 space-y-2 overflow-auto"
          data-testid="agent-composer-skill-list"
        >
          {usable.map((skill) => {
            const id = `${ids}-${skill.name}`;
            const chosen = value.find((v) => v.name === skill.name);
            return (
              <li key={skill.id} className="space-y-1.5">
                <div className="flex items-start gap-2">
                  <Checkbox
                    id={id}
                    checked={chosen != null}
                    disabled={chosen == null && full}
                    onCheckedChange={(c) => toggle(skill.name, c === true)}
                    className="mt-0.5"
                  />
                  <div className="min-w-0 flex-1">
                    <Label
                      htmlFor={id}
                      className="flex items-center gap-1.5 font-mono text-xs"
                    >
                      <span className="truncate">{skill.name}</span>
                      {isUserOnly(skill.triggers) ? (
                        <Badge
                          variant="outline"
                          className="px-1 py-0 font-sans text-[10px]"
                        >
                          {t("agentEnvironment.skills.onlyWhenAsked")}
                        </Badge>
                      ) : null}
                    </Label>
                    <p className="text-muted-foreground truncate text-xs">
                      {skill.description}
                    </p>
                  </div>
                </div>
                {chosen ? (
                  <Input
                    value={chosen.arguments ?? ""}
                    onChange={(e) => setArguments(skill.name, e.target.value)}
                    onKeyDown={(e) => {
                      // Enter must not submit the composer from the popover.
                      if (e.key === "Enter") e.preventDefault();
                    }}
                    placeholder={t("agentSessions.skillsArgumentsPlaceholder")}
                    aria-label={t("agentSessions.skillsArgumentsLabel", {
                      name: skill.name,
                    })}
                    aria-invalid={
                      skillArgumentsProblem(chosen.arguments ?? "") != null
                    }
                    className="ml-6 h-8 w-[calc(100%-1.5rem)] text-xs"
                  />
                ) : null}
              </li>
            );
          })}
        </ul>
      )}
      <p
        className={
          problem ? "text-destructive text-xs" : "text-muted-foreground text-xs"
        }
        role={problem ? "alert" : undefined}
      >
        {t(
          problem ??
            (full ? "agentSessions.skillsFull" : "agentSessions.skillsHint"),
        )}
      </p>
    </>
  );
}
