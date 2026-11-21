import { Plus } from "lucide-react";
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { ReservedButton } from "@/common/components/reserved-skeleton";
import { Skeleton } from "@/common/components/ui/skeleton";
import { TableCell, TableRow } from "@/common/components/ui/table";
import { useTranslations } from "@/common/hooks/use-translations";
import { ApiKeysScopeTabs } from "@/features/api-keys/components/api-keys-scope-tabs";
import {
  API_KEYS_ACTIONS_CELL,
  API_KEYS_NAME_CELL,
  ApiKeysTable,
} from "@/features/api-keys/components/api-keys-table";
import { useWorkspaceAdmin } from "@/features/api-keys/hooks/use-workspace-admin";

/**
 * The keys table while its keys load (w4/686, w9/132): the ready table's
 * columns and header, then one row shaped like a key's — the name (over its
 * creator's line in an Admin's every-member view), the permissions summary
 * beside its 32 px gear (w9/m496), the client ID beside its 32 px copy
 * button, the two ages, and the 36 px revoke slot, the name and revoke held
 * as the ready row's are. The panel and its frame below both draw it.
 */
export function ApiKeysTableFrame({
  namesCreator = false,
}: {
  /** The ready rows name their creator: an Admin's every-member view. */
  namesCreator?: boolean;
}) {
  return (
    <ApiKeysTable aria-hidden="true" data-testid="panel-table-frame">
      <TableRow data-testid="panel-table-row-placeholder">
        <TableCell className={API_KEYS_NAME_CELL}>
          <Skeleton className="h-5 w-24 max-w-full" />
          {namesCreator ? (
            <Skeleton
              className="h-4 w-28 max-w-full"
              data-testid="api-key-creator-placeholder"
            />
          ) : null}
        </TableCell>
        <TableCell>
          <div
            className="flex items-center gap-1"
            data-testid="api-key-permissions-placeholder"
          >
            <Skeleton className="h-5 w-16" />
            <Skeleton className="size-8" />
          </div>
        </TableCell>
        <TableCell>
          <div className="flex items-center gap-1">
            <Skeleton className="h-5 min-w-0 flex-1" />
            <Skeleton className="size-8" />
          </div>
        </TableCell>
        <TableCell>
          <Skeleton className="h-5 w-8" />
        </TableCell>
        <TableCell>
          <Skeleton className="h-5 w-10" />
        </TableCell>
        <TableCell className={API_KEYS_ACTIONS_CELL}>
          <Skeleton className="ml-auto size-9" />
        </TableCell>
      </TableRow>
    </ApiKeysTable>
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
        <ApiKeysTableFrame />
      </CardContent>
    </Card>
  );
}
