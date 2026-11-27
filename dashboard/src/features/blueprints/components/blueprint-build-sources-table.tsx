import { Link } from "@tanstack/react-router";
import { FolderKanban, FolderMinus, Package, Unlink } from "lucide-react";
import { RelativeAge } from "@/common/components/relative-time";
import { Badge } from "@/common/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils";
import { formatRepoLabel } from "@/features/services/lib/repo";
import type { BuildSourceView } from "@/features/build-sources/types";
import type { BuildSourceGroup } from "@/features/blueprints/lib/build-source-groups";

/**
 * A Blueprint's build sources in Render's Build Sources table (w3/m680): its
 * name column titled Build Source Name, each linked to its page and marked
 * Unlinked while no service builds from it, then its Source and when it last
 * changed, in a row per project above that project's build sources.
 */
export function BlueprintBuildSourcesTable({
  groups,
}: {
  groups: readonly BuildSourceGroup[];
}) {
  const { t } = useTranslations();
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t("blueprints.colBuildSourceName")}</TableHead>
          <TableHead>{t("buildSources.colSource")}</TableHead>
          <TableHead className="hidden sm:table-cell">
            {t("buildSources.colUpdated")}
          </TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {groups.map((group) => (
          <BuildSourceGroupRows key={group.project?.id ?? ""} group={group} />
        ))}
      </TableBody>
    </Table>
  );
}

function BuildSourceGroupRows({ group }: { group: BuildSourceGroup }) {
  const { t } = useTranslations();
  const GroupIcon = group.project ? FolderKanban : FolderMinus;
  return (
    <>
      <TableRow className="hover:bg-transparent">
        <TableCell colSpan={3}>
          <span className="flex items-center gap-2 font-medium">
            <GroupIcon
              aria-hidden="true"
              className="size-4 shrink-0 text-muted-foreground"
            />
            <span className="truncate">
              {group.project?.name ?? t("blueprints.buildSourcesUngrouped")}
            </span>
          </span>
        </TableCell>
      </TableRow>
      {group.sources.map((source) => (
        <TableRow key={source.id}>
          <TableCell className="pl-8">
            {/* min-h-[22px]: the Unlinked badge's height, so every row is as
                tall as its pending frame's. */}
            <span className="flex min-h-[22px] min-w-0 items-center gap-2">
              <Package
                aria-hidden="true"
                className="size-4 shrink-0 text-muted-foreground"
              />
              <Link
                to="/build-sources/$buildSourceId"
                params={{ buildSourceId: source.id }}
                className="max-w-32 truncate font-medium hover:underline sm:max-w-56"
              >
                {source.name}
              </Link>
              {source.serviceLinks.length === 0 ? (
                <Badge variant="outline" className="shrink-0 gap-1">
                  <Unlink aria-hidden="true" />
                  {t("blueprints.buildSourceUnlinked")}
                </Badge>
              ) : null}
            </span>
          </TableCell>
          <TableCell className="text-muted-foreground">
            <BuildSourceOrigin source={source} />
          </TableCell>
          <TableCell className="hidden tabular-nums text-muted-foreground sm:table-cell">
            <RelativeAge value={source.updatedAt} fallback="—" />
          </TableCell>
        </TableRow>
      ))}
    </>
  );
}

const ORIGIN_CLASS = "block max-w-40 truncate sm:max-w-md";

/** Where a build source builds from: its repository and branch, or its image. */
function BuildSourceOrigin({ source }: { source: BuildSourceView }) {
  if (source.git) {
    return (
      <span className={ORIGIN_CLASS} title={source.git.repoUrl}>
        {formatRepoLabel(source.git.repoUrl)}
        {source.git.branch ? ` · ${source.git.branch}` : ""}
      </span>
    );
  }
  return (
    <span
      className={cn(ORIGIN_CLASS, "font-mono text-xs")}
      title={source.image?.imageUrl}
    >
      {source.image?.imageUrl ?? "—"}
    </span>
  );
}
