import { Plus } from "lucide-react";
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { PanelTableFrame } from "@/common/components/panel-states";
import { ReservedButton } from "@/common/components/reserved-skeleton";
import { Skeleton } from "@/common/components/ui/skeleton";
import { TableCell, TableRow } from "@/common/components/ui/table";
import { useTranslations } from "@/common/hooks/use-translations";
import { ApiKeysScopeTabs } from "@/features/api-keys/components/api-keys-scope-tabs";
import { useWorkspaceAdmin } from "@/features/api-keys/hooks/use-workspace-admin";
import {
  API_KEYS_PERMISSIONS_COLUMN,
  apiKeysColumns,
} from "@/features/api-keys/lib/api-keys-columns";

/**
 * The keys table while its keys load (w4/686): the ready header, then one row
 * shaped like a key's — a text line per column, the permissions summary
 * beside its 32 px gear (w9/m496), and the 36 px revoke slot. The panel and
 * its frame below both draw it.
 */
export function ApiKeysTableFrame({ columns }: { columns: string[] }) {
  return (
    <PanelTableFrame columns={columns}>
      <TableRow data-testid="panel-table-row-placeholder">
        {columns.map((column, i) => (
          <TableCell key={column}>
            {i === API_KEYS_PERMISSIONS_COLUMN ? (
              <div
                className="flex items-center gap-1"
                data-testid="api-key-permissions-placeholder"
              >
                <Skeleton className="h-5 w-16" />
                <Skeleton className="size-8" />
              </div>
            ) : (
              <Skeleton className="h-5 w-16" />
            )}
          </TableCell>
        ))}
        <TableCell>
          <Skeleton className="ml-auto size-9" />
        </TableCell>
      </TableRow>
    </PanelTableFrame>
  );
}

/**
 * ApiKeysPanel's frame before it mounts (w3/067): its static title and
 * description (the CLI guide link's words included), the Create button's box,
 * the Admin's disabled scope switch unless the caller is known not to be an
 * Admin (w9/m539, as the panel holds it), then the table frame the panel
 * draws while its keys load. Each key past the first is a data-dependent row.
 */
export function ApiKeysPanelSkeleton() {
  const { t } = useTranslations();
  const admin = useWorkspaceAdmin();
  return (
    <Card aria-hidden="true" data-testid="api-keys-panel-skeleton">
      <CardHeader>
        <CardTitle>{t("apiKeys.title")}</CardTitle>
        <CardDescription>
          {t("apiKeys.description")}{" "}
          <span className="underline underline-offset-4">
            {t("apiKeys.cliGuide")}
          </span>
        </CardDescription>
        <CardAction>
          <ReservedButton variant="outline" size="sm">
            <Plus /> {t("apiKeys.create")}
          </ReservedButton>
        </CardAction>
      </CardHeader>
      <CardContent className="space-y-4">
        {admin === false ? null : <ApiKeysScopeTabs value="own" disabled />}
        <ApiKeysTableFrame columns={apiKeysColumns(t)} />
      </CardContent>
    </Card>
  );
}
