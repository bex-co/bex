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
import { useApiKeys } from "@/features/api-keys/hooks/use-api-keys";
import { useRevokeApiKey } from "@/features/api-keys/hooks/use-revoke-api-key";
import { ApiKeyRow } from "@/features/api-keys/components/api-key-row";
import { CreateApiKeyDialog } from "@/features/api-keys/components/create-api-key-dialog";
import { ApiKeysTableFrame } from "@/features/api-keys/components/api-keys-panel-skeleton";
import { apiKeysColumns } from "@/features/api-keys/lib/api-keys-columns";

/**
 * Settings → API Keys (w4/m8): lists the workspace's bex-minted machine
 * credentials and lets a permitted session mint or revoke them — the m3
 * lifecycle's human-facing surface, so handing an agent a credential no longer
 * requires `curl`. Keys are workspace-shared (no per-user owner), so this list
 * is not "my keys" — it's every key any `can_manage_keys` caller minted.
 */
export function ApiKeysPanel() {
  const { t } = useTranslations();
  const { keys, loading, error, refetch } = useApiKeys();
  const { revoke, revoking } = useRevokeApiKey();

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
      <CardContent>
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
