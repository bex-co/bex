import { AlertTriangle, Bot } from "lucide-react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
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
import { useConnectedAgents } from "@/features/connected-agents/hooks/use-connected-agents";
import { ConnectedAgentRow } from "@/features/connected-agents/components/connected-agent-row";

/**
 * Settings → Security & Compliance "Connected agents" card (w4/m18): every
 * OAuth2 client the signed-in human has authorized — the revocation surface
 * the m16 remembered-consent flow shipped without. Account-scoped (the
 * subject is whoever's Kratos session this request carries), not
 * workspace-scoped, so it needs no workspace switcher wiring.
 */
export function ConnectedAgentsPanel() {
  const { t } = useTranslations();
  const { agents, loading, error, revoke, revoking } = useConnectedAgents();

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("connectedAgents.title")}</CardTitle>
        <CardDescription>{t("connectedAgents.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        {error ? (
          <PanelCenteredState
            icon={<AlertTriangle />}
            title={t("connectedAgents.errorTitle")}
            body={t("connectedAgents.errorBody")}
          />
        ) : loading ? (
          <PanelTableSkeleton />
        ) : agents.length === 0 ? (
          <PanelCenteredState
            icon={<Bot />}
            title={t("connectedAgents.emptyTitle")}
            body={t("connectedAgents.emptyBody")}
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("connectedAgents.colClient")}</TableHead>
                <TableHead>{t("connectedAgents.colScopes")}</TableHead>
                <TableHead>{t("connectedAgents.colGranted")}</TableHead>
                <TableHead className="sr-only text-right">actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {agents.map((agent) => (
                <ConnectedAgentRow
                  key={agent.clientId}
                  agent={agent}
                  onRevoke={revoke}
                  revoking={revoking === agent.clientId}
                />
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}
