import { Link } from "@tanstack/react-router";
import { LocalDateTime } from "@/common/components/local-time";
import {
  Table,
  TableBody,
  TableCell,
  TableRow,
} from "@/common/components/ui/table";
import { useTranslations } from "@/common/hooks/use-translations";
import { EnvGroupsTableHeader } from "@/features/env-groups/components/env-groups-table-header";
import { environmentNameIn } from "@/features/env-groups/lib/scope";
import type { EnvGroupView } from "@/features/env-groups/types";

/**
 * Environment Groups in Render's Env Groups table: the name over its id and
 * linked-service count, its Environment, its variable and secret file counts,
 * and when it last changed. The Environment Groups list and a Blueprint's
 * Resources page (w3/m680), where Render draws the same table, share it; the
 * list puts it in a border.
 */
export function EnvGroupsTable({
  groups,
  scope,
}: {
  groups: readonly EnvGroupView[];
  /** The workspace's environments and projects, which name each scope. */
  scope: Parameters<typeof environmentNameIn>[0];
}) {
  const { t } = useTranslations();
  const environmentLabel = (environmentId: string | null) => {
    if (!environmentId) return t("envGroups.workspaceScope");
    return (
      environmentNameIn(scope, environmentId) ??
      t("envGroups.unknownEnvironment", { id: environmentId })
    );
  };
  return (
    <Table>
      <EnvGroupsTableHeader />
      <TableBody>
        {groups.map((group) => (
          <TableRow key={group.id}>
            <TableCell className="min-w-48">
              <Link
                to="/env-groups/$groupId"
                params={{ groupId: group.id }}
                className="font-medium hover:underline"
              >
                {group.name}
              </Link>
              <p className="max-w-64 truncate font-mono text-xs text-muted-foreground">
                {group.id} ·{" "}
                {t("envGroups.serviceCount", {
                  count: group.serviceLinks.length,
                })}
              </p>
            </TableCell>
            {/* One line, as the column's label: the table scrolls on a
                phone rather than wrapping a row taller (w3/m680). */}
            <TableCell className="min-w-40 whitespace-nowrap">
              {environmentLabel(group.environmentId)}
            </TableCell>
            <TableCell className="text-right tabular-nums">
              {group.envVarKeys.length}
            </TableCell>
            <TableCell className="text-right tabular-nums">
              {group.secretFileNames.length}
            </TableCell>
            <TableCell className="whitespace-nowrap text-right text-muted-foreground">
              <LocalDateTime value={group.updatedAt} />
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
