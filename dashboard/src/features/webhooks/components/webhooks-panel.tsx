import { AlertTriangle, ShieldAlert, Webhook } from "lucide-react";
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
import { useWebhooks } from "@/features/webhooks/hooks/use-webhooks";
import { useDeleteWebhook } from "@/features/webhooks/hooks/use-delete-webhook";
import { useSetWebhookEnabled } from "@/features/webhooks/hooks/use-set-webhook-enabled";
import { WebhookRow } from "@/features/webhooks/components/webhook-row";
import { CreateWebhookDialog } from "@/features/webhooks/components/create-webhook-dialog";

type ErrorKind = "forbidden" | "generic";

function classifyWebhooksError(error: Error | undefined): ErrorKind | null {
  if (!error) return null;
  return error.message.toLowerCase().includes("forbidden")
    ? "forbidden"
    : "generic";
}

/**
 * Settings → Integrations → Webhooks (w3/m11): the workspace's outbound
 * webhook destinations — register a URL + event subscription, see the signing
 * secret once, toggle/delete endpoints, and review per-endpoint delivery
 * history. Modeled on Render's Integrations → Webhooks page.
 */
export function WebhooksPanel() {
  const { t } = useTranslations();
  const { endpoints, loading, error, refetch } = useWebhooks();
  const { remove, deleting } = useDeleteWebhook();
  const { setEnabled, toggling } = useSetWebhookEnabled();

  const errorKind = classifyWebhooksError(error);
  const initialLoading = loading && endpoints.length === 0 && !error;

  async function handleDelete(id: string, name: string) {
    const ok = await remove(id, name);
    if (ok) await refetch();
    return ok;
  }

  // No refetch on toggle: the mutation's selection set (id/enabled/
  // disabledReason) updates the normalized Apollo cache row in place; only
  // create/delete change list membership.
  async function handleToggle(id: string, name: string, enabled: boolean) {
    return setEnabled(id, name, enabled);
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("webhooks.title")}</CardTitle>
        <CardDescription>{t("webhooks.description")}</CardDescription>
        <CardAction>
          <CreateWebhookDialog onCreated={() => void refetch()} />
        </CardAction>
      </CardHeader>
      <CardContent>
        {errorKind ? (
          <StatePanel kind={errorKind} />
        ) : initialLoading ? (
          <PanelTableSkeleton />
        ) : endpoints.length === 0 ? (
          <EmptyState />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("webhooks.colEndpoint")}</TableHead>
                <TableHead>{t("webhooks.colEvents")}</TableHead>
                <TableHead>{t("webhooks.colEnabled")}</TableHead>
                <TableHead className="sr-only text-right">actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {endpoints.map((entry) => (
                <WebhookRow
                  key={entry.id}
                  entry={entry}
                  onToggle={handleToggle}
                  onDelete={handleDelete}
                  toggling={toggling === entry.id}
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
      icon={<Webhook />}
      title={t("webhooks.emptyTitle")}
      body={t("webhooks.emptyBody")}
    />
  );
}

function StatePanel({ kind }: { kind: ErrorKind }) {
  const { t } = useTranslations();
  const copy = {
    forbidden: {
      icon: <ShieldAlert />,
      title: t("webhooks.forbiddenTitle"),
      body: t("webhooks.forbiddenBody"),
    },
    generic: {
      icon: <AlertTriangle />,
      title: t("webhooks.errorTitle"),
      body: t("webhooks.errorBody"),
    },
  }[kind];
  return (
    <PanelCenteredState icon={copy.icon} title={copy.title} body={copy.body} />
  );
}
