import { useMemo, useState } from "react";
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
import { Table, TableBody } from "@/common/components/ui/table";
import {
  PanelCenteredState,
  PanelTableSkeleton,
} from "@/common/components/panel-states";
import { isForbiddenError } from "@/common/lib/graphql-error";
import { useTranslations } from "@/common/hooks/use-translations";
import { useIsHydrated } from "@/common/hooks/use-is-hydrated";
import { useWebhooks } from "@/features/webhooks/hooks/use-webhooks";
import { useSetWebhookEnabled } from "@/features/webhooks/hooks/use-set-webhook-enabled";
import { WebhookRow } from "@/features/webhooks/components/webhook-row";
import { WebhooksTableHeader } from "@/features/webhooks/components/webhooks-table-header";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useWorkspace } from "@/features/workspaces/context";
import { proFeaturesPlan } from "@/features/workspaces/types";
import { eventLabelKey } from "@/features/webhooks/event-catalog";

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
  const { canManage } = useCapabilities();
  // Fail-closed: only show create when affirmatively allowed.
  const { currentWorkspace } = useWorkspace();
  const proPlan = proFeaturesPlan(currentWorkspace?.plan);
  const showCreate = canManage && proPlan;
  const hydrated = useIsHydrated();
  const [search, setSearch] = useState("");

  const forbidden = isForbiddenError(error);
  const initialLoading = loading && endpoints.length === 0;
  const searching = search.trim().length > 0;
  const searchableEndpoints = useMemo(
    () =>
      searching
        ? endpoints.map((endpoint) => ({
            endpoint,
            text: [
              endpoint.name,
              endpoint.url,
              ...(endpoint.eventTypes.length === 0
                ? [t("webhooks.allEvents")]
                : endpoint.eventTypes.map((eventType) => {
                    const key = eventLabelKey(eventType);
                    return key ? t(key) : eventType;
                  })),
            ]
              .join("\n")
              .toLocaleLowerCase(),
          }))
        : [],
    [endpoints, searching, t],
  );
  const filteredEndpoints = useMemo(() => {
    const needle = search.trim().toLocaleLowerCase();
    if (!needle) return endpoints;
    return searchableEndpoints
      .filter(({ text }) => text.includes(needle))
      .map(({ endpoint }) => endpoint);
  }, [endpoints, search, searchableEndpoints]);

  return (
    <Card>
      <CardHeader>
        <CardTitle as="h1">{t("webhooks.title")}</CardTitle>
        <CardDescription>
          {proPlan ? t("webhooks.description") : t("webhooks.proOnly")}
        </CardDescription>
        <CardAction>
          {/* Render's create flow is a page, not a dialog (w1/m49/t003). */}
          {showCreate ? (
            <Button variant="outline" size="sm" asChild>
              <Link to="/webhooks/new">
                <Plus /> {t("webhooks.create")}
              </Link>
            </Button>
          ) : null}
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
            body={t(proPlan ? "webhooks.emptyBody" : "webhooks.planGate")}
          />
        ) : (
          <div className="space-y-4">
            <div className="max-w-md space-y-1.5">
              <Label htmlFor="webhook-search" className="sr-only">
                {t("webhooks.searchEndpoints")}
              </Label>
              <Input
                id="webhook-search"
                type="search"
                value={search}
                // Uneditable until handlers attach: a query typed into the
                // server HTML is wiped by hydration (w4/m219).
                disabled={!hydrated}
                onChange={(event) => setSearch(event.target.value)}
                placeholder={t("webhooks.searchEndpoints")}
                aria-describedby="webhook-search-help"
              />
              <p
                id="webhook-search-help"
                className="text-muted-foreground text-xs"
              >
                {t("webhooks.searchEndpointsHelp")}
              </p>
            </div>
            {filteredEndpoints.length === 0 ? (
              <p
                className="text-muted-foreground py-6 text-center text-sm"
                role="status"
              >
                {t("webhooks.searchEndpointsEmpty")}
              </p>
            ) : (
              <Table>
                <WebhooksTableHeader />
                <TableBody>
                  {filteredEndpoints.map((entry) => (
                    // No refetch on toggle: the mutation's selection set
                    // (id/enabled/disabledReason) updates the normalized Apollo
                    // cache row in place; only create/delete change membership.
                    <WebhookRow
                      key={entry.id}
                      entry={entry}
                      onToggle={setEnabled}
                      toggling={toggling === entry.id}
                      canManage={canManage}
                    />
                  ))}
                </TableBody>
              </Table>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
