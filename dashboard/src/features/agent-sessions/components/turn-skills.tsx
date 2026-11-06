import { Sparkles } from "lucide-react";
import { Badge } from "@/common/components/ui/badge";
import { useTranslations } from "@/common/hooks/use-translations";
import type { AgentSessionSkill } from "@/graphql/schema-types";

/** The workspace skills a user turn invoked (ADR099 D4), as pills under its bubble. */
export function TurnSkills({
  skills,
}: {
  skills: Pick<AgentSessionSkill, "name" | "arguments">[];
}) {
  const { t } = useTranslations();
  if (skills.length === 0) return null;
  return (
    <ul
      className="mt-1.5 flex flex-wrap justify-end gap-1.5"
      aria-label={t("agentSessions.turnSkills")}
      data-testid="turn-skills"
    >
      {skills.map((skill) => (
        <li key={skill.name}>
          <Badge
            variant="outline"
            className="max-w-64 gap-1 font-mono"
            title={skill.arguments ?? undefined}
          >
            <Sparkles className="size-3" />
            <span className="truncate">
              {skill.name}
              {skill.arguments ? ` ${skill.arguments}` : ""}
            </span>
          </Badge>
        </li>
      ))}
    </ul>
  );
}
