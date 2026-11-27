import { Link } from "@tanstack/react-router";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
import { blueprintResourceHref } from "@/features/blueprints/lib/resource-href";
import { useBlueprintDetail } from "@/features/blueprints/lib/blueprint-detail";
import { blueprintPageRoute } from "@/features/blueprints/components/blueprint-nav";
import type { BlueprintView } from "@/features/blueprints/types";

/** Which empty copy the Blueprint's resource list shows, and whether it points at Syncs. */
function emptyState(blueprint: BlueprintView) {
  if (blueprint.status === "created") {
    return { key: "blueprints.resourcesEmptyAwaitingApproval", syncs: true };
  }
  if (blueprint.status === "error" && blueprint.lastSync) {
    return { key: "blueprints.resourcesEmptyAfterError", syncs: true };
  }
  return { key: "blueprints.resourcesEmpty", syncs: false };
}

/**
 * A Blueprint's Resources page, Render's bare `/blueprint/<id>` and
 * `/resources` (w3/m677): the resources it manages, each linked to its page.
 * Render's resource tables replace the two columns in w3/m680. A Blueprint
 * whose plan waits, or whose last sync failed, says so and links Syncs, where
 * the plan's Approve and the failed sync are now.
 */
export function BlueprintResourcesPage() {
  const { t } = useTranslations();
  const { blueprint } = useBlueprintDetail();
  const empty = emptyState(blueprint);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">
          {t("blueprints.resourcesTitle")}
        </CardTitle>
      </CardHeader>
      <CardContent>
        {blueprint.resources && blueprint.resources.length > 0 ? (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("blueprints.colName")}</TableHead>
                <TableHead>{t("blueprints.colType")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {blueprint.resources.map((r) => {
                const href = blueprintResourceHref(r.type, r.id);
                return (
                  <TableRow key={r.id}>
                    <TableCell className="font-medium">
                      {/* Every other resource table in the product links its
                          names; this one dead-ended (w4/101). A resource kind
                          this build has no page for still renders as plain
                          text — a broken href would be worse than none. */}
                      {href ? (
                        <Link to={href} className="hover:underline">
                          {r.name}
                        </Link>
                      ) : (
                        r.name
                      )}
                    </TableCell>
                    <TableCell className="text-muted-foreground capitalize">
                      {r.type.replace(/_/g, " ")}
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        ) : (
          <div className="space-y-2 text-sm">
            <p className="text-muted-foreground">{t(empty.key)}</p>
            {empty.syncs ? (
              <Link
                to={blueprintPageRoute("/syncs")}
                params={{ blueprintId: blueprint.id }}
                className="inline-flex font-medium text-primary underline-offset-4 hover:underline"
              >
                {t("blueprints.resourcesViewSyncs")}
              </Link>
            ) : null}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
