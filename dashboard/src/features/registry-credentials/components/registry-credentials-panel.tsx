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
} from "@/common/components/panel-states";
import { useTranslations } from "@/common/hooks/use-translations";
import { useRegistryCredentials } from "@/features/registry-credentials/hooks/use-registry-credentials";
import { useDeleteRegistryCredential } from "@/features/registry-credentials/hooks/use-delete-registry-credential";
import { RegistryCredentialRow } from "@/features/registry-credentials/components/registry-credential-row";
import { CreateRegistryCredentialDialog } from "@/features/registry-credentials/components/create-registry-credential-dialog";

type ErrorKind = "forbidden" | "generic";

function classifyError(error: Error | undefined): ErrorKind | null {
  if (!error) return null;
  return error.message.toLowerCase().includes("forbidden")
    ? "forbidden"
    : "generic";
}

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

  const errorKind = classifyError(error);
  const initialLoading = loading && credentials.length === 0 && !error;

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
        {errorKind ? (
          <StatePanel kind={errorKind} />
        ) : initialLoading ? (
          <PanelTableSkeleton />
        ) : credentials.length === 0 ? (
          <EmptyState />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("registryCredentials.colName")}</TableHead>
                <TableHead>{t("registryCredentials.colUsername")}</TableHead>
                <TableHead>{t("registryCredentials.colStatus")}</TableHead>
                <TableHead>{t("registryCredentials.colCreated")}</TableHead>
                <TableHead className="sr-only text-right">actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {credentials.map((entry) => (
                <RegistryCredentialRow
                  key={entry.id}
                  entry={entry}
                  onDelete={handleDelete}
                  onUpdated={() => void refetch()}
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

function EmptyState() {
  const { t } = useTranslations();
  return (
    <PanelCenteredState
      icon={<KeyRound />}
      title={t("registryCredentials.emptyTitle")}
      body={t("registryCredentials.emptyBody")}
    />
  );
}

function StatePanel({ kind }: { kind: ErrorKind }) {
  const { t } = useTranslations();
  const copy = {
    forbidden: {
      icon: <ShieldAlert />,
      title: t("registryCredentials.forbiddenTitle"),
      body: t("registryCredentials.forbiddenBody"),
    },
    generic: {
      icon: <AlertTriangle />,
      title: t("registryCredentials.errorTitle"),
      body: t("registryCredentials.errorBody"),
    },
  }[kind];
  return (
    <PanelCenteredState icon={copy.icon} title={copy.title} body={copy.body} />
  );
}
