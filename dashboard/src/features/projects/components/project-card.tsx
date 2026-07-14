import { Link } from "@tanstack/react-router";
import { CheckCircle2, XCircle } from "lucide-react";
import { cn } from "@/common/lib/utils/utils.ts";
import { useTranslations } from "@/common/hooks/use-translations";
import { isRowHealthy } from "@/features/projects/lib/resource-health";
import type { ResourceRow } from "@/features/projects/types";

export interface ProjectCardProps {
  id: string;
  name: string;
  rows: ResourceRow[];
}

/**
 * One project tile on the Overview page's project grid (Render parity: the
 * workspace homepage shows a card per project with a rolled-up health pill —
 * verified against a live Render workspace). Links to the project's own
 * page, `/project/$projectId`.
 */
export function ProjectCard({ id, name, rows }: ProjectCardProps) {
  const { t } = useTranslations();
  const unhealthy = rows.filter((r) => !isRowHealthy(r)).length;

  return (
    <Link
      to="/project/$projectId"
      params={{ projectId: id }}
      className="flex flex-col gap-3 rounded-xl border p-5 transition-colors hover:bg-accent/50"
    >
      <span className="font-semibold">{name}</span>
      {rows.length === 0 ? (
        <span className="text-sm text-muted-foreground">
          {t("projects.cardEmpty")}
        </span>
      ) : unhealthy === 0 ? (
        <span className="flex items-center gap-1.5 text-sm text-emerald-600 dark:text-emerald-400">
          <CheckCircle2 className="size-4" />
          {t("projects.cardHealthy")}
        </span>
      ) : (
        <span
          className={cn(
            "flex items-center gap-1.5 text-sm text-destructive",
          )}
        >
          <XCircle className="size-4" />
          {t("projects.cardUnhealthy", { count: unhealthy })}
        </span>
      )}
    </Link>
  );
}
