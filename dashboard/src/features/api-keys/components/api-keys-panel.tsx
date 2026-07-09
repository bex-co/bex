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
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { useApiKeys } from "@/features/api-keys/hooks/use-api-keys";
import { useRevokeApiKey } from "@/features/api-keys/hooks/use-revoke-api-key";
import { ApiKeyRow } from "@/features/api-keys/components/api-key-row";
import { CreateApiKeyDialog } from "@/features/api-keys/components/create-api-key-dialog";

type ErrorKind = "forbidden" | "generic";

function classifyApiKeysError(error: Error | undefined): ErrorKind | null {
  if (!error) return null;
  return error.message.toLowerCase().includes("forbidden")
    ? "forbidden"
    : "generic";
}

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

  const errorKind = classifyApiKeysError(error);
  const initialLoading = loading && keys.length === 0 && !error;

  async function handleRevoke(id: string, name: string) {
    const ok = await revoke(id, name);
    if (ok) await refetch();
    return ok;
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("apiKeys.title")}</CardTitle>
        <CardDescription>{t("apiKeys.description")}</CardDescription>
        <CardAction>
          <CreateApiKeyDialog onCreated={() => void refetch()} />
        </CardAction>
      </CardHeader>
      <CardContent>
        {errorKind ? (
          <StatePanel kind={errorKind} />
        ) : initialLoading ? (
          <TableSkeleton />
        ) : keys.length === 0 ? (
          <EmptyState />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("apiKeys.colName")}</TableHead>
                <TableHead>{t("apiKeys.colCreated")}</TableHead>
                <TableHead>{t("apiKeys.colCreatedBy")}</TableHead>
                <TableHead>{t("apiKeys.colLastUsed")}</TableHead>
                <TableHead className="sr-only text-right">actions</TableHead>
              </TableRow>
            </TableHeader>
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

function TableSkeleton() {
  return (
    <div className="space-y-2">
      {[0, 1].map((i) => (
        <div key={i} className="flex items-center gap-4">
          <Skeleton className="h-6 w-1/3" />
          <Skeleton className="h-6 flex-1" />
        </div>
      ))}
    </div>
  );
}

function EmptyState() {
  const { t } = useTranslations();
  return (
    <CenteredState
      icon={<KeyRound />}
      title={t("apiKeys.emptyTitle")}
      body={t("apiKeys.emptyBody")}
    />
  );
}

function StatePanel({ kind }: { kind: ErrorKind }) {
  const { t } = useTranslations();
  const copy = {
    forbidden: {
      icon: <ShieldAlert />,
      title: t("apiKeys.forbiddenTitle"),
      body: t("apiKeys.forbiddenBody"),
    },
    generic: {
      icon: <AlertTriangle />,
      title: t("apiKeys.errorTitle"),
      body: t("apiKeys.errorBody"),
    },
  }[kind];
  return <CenteredState icon={copy.icon} title={copy.title} body={copy.body} />;
}

function CenteredState({
  icon,
  title,
  body,
}: {
  icon: React.ReactNode;
  title: string;
  body: string;
}) {
  return (
    <div className="flex flex-col items-center justify-center py-12 text-center">
      <div className="text-muted-foreground/50 mb-3 [&_svg]:size-8">{icon}</div>
      <p className="mb-1 font-medium">{title}</p>
      <p className="text-muted-foreground text-sm">{body}</p>
    </div>
  );
}
