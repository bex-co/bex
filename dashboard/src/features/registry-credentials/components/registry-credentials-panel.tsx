import { AlertTriangle, KeyRound, ShieldAlert } from "lucide-react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardAction,
  CardContent,
} from "@/common/components/ui/card";
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
} from "@/common/components/ui/table";
import {
  PanelCenteredState,
  PanelTableSkeleton,
  TableActionsHead,
} from "@/common/components/panel-states";
import { isForbiddenError } from "@/common/lib/graphql-error";
import { useTranslations } from "@/common/hooks/use-translations";
import { useRegistryCredentials } from "@/features/registry-credentials/hooks/use-registry-credentials";
import { useDeleteRegistryCredential } from "@/features/registry-credentials/hooks/use-delete-registry-credential";
import { RegistryCredentialRow } from "@/features/registry-credentials/components/registry-credential-row";
import { CreateRegistryCredentialDialog } from "@/features/registry-credentials/components/create-registry-credential-dialog";

/**
 * Settings → Integrations → Registry Credentials (w2/m14): a workspace's
 * stored credentials for private external image registries (Docker Hub,
 * GHCR, GitLab Container Registry, ECR, etc.), so an existing-image service
 * can pull from a non-public, non-Zot source. The first tenant of an
 * "Integrations" area — a future outbound-webhooks feature has an obvious
 * home to join here.
 */
export function RegistryCredentialsPanel() {
  const { t } = useTranslations();
  const { credentials, loading, error, refetch } = useRegistryCredentials();
  const { remove, deleting } = useDeleteRegistryCredential();

  const forbidden = isForbiddenError(error);
  const initialLoading = loading && credentials.length === 0;

  async function handleDelete(id: string, name: string) {
    const ok = await remove(id, name);
    if (ok) await refetch();
    return ok;
  }

  return (
    <Card id="registry-credentials" className="scroll-mt-4">
      <CardHeader>
        <CardTitle>{t("registryCredentials.title")}</CardTitle>
        <CardDescription>
          {t("registryCredentials.description")}
        </CardDescription>
        <CardAction>
          <CreateRegistryCredentialDialog onCreated={() => void refetch()} />
        </CardAction>
      </CardHeader>
      <CardContent>
        {error ? (
          <PanelCenteredState
            icon={forbidden ? <ShieldAlert /> : <AlertTriangle />}
            title={t(
              forbidden
                ? "registryCredentials.forbiddenTitle"
                : "registryCredentials.errorTitle",
            )}
            body={t(
              forbidden
                ? "registryCredentials.forbiddenBody"
                : "registryCredentials.errorBody",
            )}
          />
        ) : initialLoading ? (
          <PanelTableSkeleton />
        ) : credentials.length === 0 ? (
          <PanelCenteredState
            icon={<KeyRound />}
            title={t("registryCredentials.emptyTitle")}
            body={t("registryCredentials.emptyBody")}
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("registryCredentials.colName")}</TableHead>
                <TableHead>{t("registryCredentials.colUsername")}</TableHead>
                <TableHead>{t("registryCredentials.colStatus")}</TableHead>
                <TableHead>{t("registryCredentials.colCreated")}</TableHead>
                <TableActionsHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {credentials.map((entry) => (
                <RegistryCredentialRow
                  key={entry.id}
                  entry={entry}
                  onDelete={handleDelete}
                  deleting={deleting === entry.id}
                />
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}
