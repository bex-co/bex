import { Link } from "@tanstack/react-router";
import { AlertTriangle, Plus, ShieldAlert, Webhook } from "lucide-react";
import { Button } from "@/common/components/ui/button";
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
import { useWebhooks } from "@/features/webhooks/hooks/use-webhooks";
import { useSetWebhookEnabled } from "@/features/webhooks/hooks/use-set-webhook-enabled";
import { WebhookRow } from "@/features/webhooks/components/webhook-row";

/**
 * Settings → Integrations → Webhooks (w3/m11): the workspace's outbound
 * webhook destinations — register a URL + event subscription, see the signing
 * secret once, toggle/delete endpoints, and review per-endpoint delivery
 * history. Modeled on Render's Integrations → Webhooks page.
 */
export function WebhooksPanel() {
  const { t } = useTranslations();
  const { endpoints, loading, error } = useWebhooks();
  const { setEnabled, toggling } = useSetWebhookEnabled();

  const forbidden = isForbiddenError(error);
  const initialLoading = loading && endpoints.length === 0;

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("webhooks.title")}</CardTitle>
        <CardDescription>{t("webhooks.description")}</CardDescription>
        <CardAction>
          {/* Render's create flow is a page, not a dialog (w1/m49/t003). */}
          <Button variant="outline" size="sm" asChild>
            <Link to="/webhooks/new">
              <Plus /> {t("webhooks.create")}
            </Link>
          </Button>
        </CardAction>
      </CardHeader>
      <CardContent>
        {error ? (
          <PanelCenteredState
            icon={forbidden ? <ShieldAlert /> : <AlertTriangle />}
            title={t(
              forbidden ? "webhooks.forbiddenTitle" : "webhooks.errorTitle",
            )}
            body={t(
              forbidden ? "webhooks.forbiddenBody" : "webhooks.errorBody",
            )}
          />
        ) : initialLoading ? (
          <PanelTableSkeleton />
        ) : endpoints.length === 0 ? (
          <PanelCenteredState
            icon={<Webhook />}
            title={t("webhooks.emptyTitle")}
            body={t("webhooks.emptyBody")}
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("webhooks.colEndpoint")}</TableHead>
                <TableHead>{t("webhooks.colEvents")}</TableHead>
                <TableHead>{t("webhooks.colEnabled")}</TableHead>
                <TableActionsHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {endpoints.map((entry) => (
                // No refetch on toggle: the mutation's selection set
                // (id/enabled/disabledReason) updates the normalized Apollo
                // cache row in place; only create/delete change membership.
                <WebhookRow
                  key={entry.id}
                  entry={entry}
                  onToggle={setEnabled}
                  toggling={toggling === entry.id}
                />
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}
