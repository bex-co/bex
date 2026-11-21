import { useState } from "react";
import { AlertTriangle, KeyRound, ShieldAlert } from "lucide-react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardAction,
  CardContent,
} from "@/common/components/ui/card";
import { Table, TableBody } from "@/common/components/ui/table";
import {
  PanelCenteredState,
  PanelTableHeader,
} from "@/common/components/panel-states";
import { isForbiddenError } from "@/common/lib/graphql-error";
import { useTranslations } from "@/common/hooks/use-translations";
import { useSelfSubjects } from "@/common/hooks/use-self-subjects";
import { useApiKeys } from "@/features/api-keys/hooks/use-api-keys";
import { useRevokeApiKey } from "@/features/api-keys/hooks/use-revoke-api-key";
import { useWorkspaceAdmin } from "@/features/api-keys/hooks/use-workspace-admin";
import { ApiKeyRow } from "@/features/api-keys/components/api-key-row";
import { CreateApiKeyDialog } from "@/features/api-keys/components/create-api-key-dialog";
import { ApiKeysTableFrame } from "@/features/api-keys/components/api-keys-panel-skeleton";
import {
  ApiKeysScopeTabs,
  type ApiKeysScope,
} from "@/features/api-keys/components/api-keys-scope-tabs";
import { apiKeysColumns } from "@/features/api-keys/lib/api-keys-columns";

/**
 * Settings → API Keys (w4/m8): lists the caller's bex-minted machine
 * credentials and lets them mint or revoke them — the m3 lifecycle's
 * human-facing surface, so handing an agent a credential no longer requires
 * `curl`. Every member role mints keys here and sees the keys they created,
 * as on Render's Account Settings (w9/m514, w9/m539). An Admin also gets a
 * switch to every member's keys, where each row names its creator, only
 * their own rows carry the gear (a grant is its creator's to change) and
 * every row can be revoked. The switch holds its place while the role is
 * still unknown, as the skeleton does.
 */
export function ApiKeysPanel() {
  const { t } = useTranslations();
  const admin = useWorkspaceAdmin();
  const [scope, setScope] = useState<ApiKeysScope>("own");
  const allMembers = admin === true && scope === "all";
  const { keys, loading, error, refetch } = useApiKeys(allMembers);
  const { revoke, revoking } = useRevokeApiKey();
  const { subjects } = useSelfSubjects();

  const columns = apiKeysColumns(t);
  const forbidden = isForbiddenError(error);
  const initialLoading = loading && keys.length === 0;

  async function handleRevoke(id: string, name: string) {
    const ok = await revoke(id, name);
    if (ok) await refetch();
    return ok;
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("apiKeys.title")}</CardTitle>
        <CardDescription>
          {t("apiKeys.description")}{" "}
          <a
            className="underline underline-offset-4 hover:text-foreground"
            href="https://bex.co/docs/cli"
            target="_blank"
            rel="noreferrer"
          >
            {t("apiKeys.cliGuide")}
          </a>
        </CardDescription>
        <CardAction>
          <CreateApiKeyDialog onCreated={() => void refetch()} />
        </CardAction>
      </CardHeader>
      <CardContent className="space-y-4">
        {admin === false ? null : (
          <ApiKeysScopeTabs
            value={allMembers ? "all" : "own"}
            onValueChange={setScope}
            disabled={admin !== true}
          />
        )}
        {error ? (
          <PanelCenteredState
            icon={forbidden ? <ShieldAlert /> : <AlertTriangle />}
            title={t(
              forbidden ? "apiKeys.forbiddenTitle" : "apiKeys.errorTitle",
            )}
            body={t(forbidden ? "apiKeys.forbiddenBody" : "apiKeys.errorBody")}
          />
        ) : initialLoading ? (
          <ApiKeysTableFrame columns={columns} />
        ) : keys.length === 0 ? (
          <PanelCenteredState
            icon={<KeyRound />}
            title={t("apiKeys.emptyTitle")}
            body={t("apiKeys.emptyBody")}
          />
        ) : (
          <Table>
            <PanelTableHeader columns={columns} />
            <TableBody>
              {keys.map((entry) => (
                <ApiKeyRow
                  key={entry.id}
                  entry={entry}
                  own={
                    !allMembers ||
                    (!!entry.createdBy && subjects.has(entry.createdBy))
                  }
                  onRevoke={handleRevoke}
                  revoking={revoking === entry.id}
                />
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}
