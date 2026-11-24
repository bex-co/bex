import { Box, GitBranch } from "lucide-react";
import { Badge } from "@/common/components/ui/badge";
import { useTranslations } from "@/common/hooks/use-translations";
import { formatRepoLabel } from "@/features/services/lib/repo";
import type { BuildSourceView } from "@/features/build-sources/types";

/**
 * Shared source-kind provenance summary for `/build-sources` (desktop + mobile)
 * and Project Overview's Build Sources section.
 */
export function BuildSourceKindSummary({
  source,
  showRuntime = true,
}: {
  source: BuildSourceView;
  showRuntime?: boolean;
}) {
  const { t } = useTranslations();

  if (source.kind === "git" && source.git) {
    const repo = formatRepoLabel(source.git.repoUrl);
    return (
      <div className="min-w-0 space-y-1">
        <div className="flex flex-wrap items-center gap-1.5">
          <Badge variant="secondary" className="gap-1">
            <GitBranch className="size-3" aria-hidden="true" />
            {t("buildSources.kindGit")}
          </Badge>
          {showRuntime && source.git.runtime ? (
            <Badge variant="outline" className="font-mono text-xs">
              {source.git.runtime}
            </Badge>
          ) : null}
        </div>
        <p
          className="text-muted-foreground truncate text-xs"
          title={source.git.repoUrl}
        >
          <span className="text-foreground font-medium">{repo}</span>
          {source.git.branch ? ` · ${source.git.branch}` : ""}
        </p>
      </div>
    );
  }

  return (
    <div className="min-w-0 space-y-1">
      <div className="flex flex-wrap items-center gap-1.5">
        <Badge variant="secondary" className="gap-1">
          <Box className="size-3" aria-hidden="true" />
          {t("buildSources.kindImage")}
        </Badge>
      </div>
      <p
        className="text-foreground truncate font-mono text-xs"
        title={source.image?.imageUrl ?? ""}
      >
        {source.image?.imageUrl ?? "—"}
      </p>
    </div>
  );
}
