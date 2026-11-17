import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { PanelTableFrame } from "@/common/components/panel-states";
import { useTranslations } from "@/common/hooks/use-translations";
import { connectedAgentsColumns } from "@/features/connected-agents/lib/connected-agents-columns";

/**
 * ConnectedAgentsPanel's frame before it mounts (w3/067): its static title
 * and description, then the table frame the panel draws while it loads. An
 * account with no authorized agent settles to the shorter empty state, and
 * each agent past the first is a row: both are data-dependent.
 */
export function ConnectedAgentsPanelSkeleton() {
  const { t } = useTranslations();
  return (
    <Card aria-hidden="true" data-testid="connected-agents-panel-skeleton">
      <CardHeader>
        <CardTitle>{t("connectedAgents.title")}</CardTitle>
        <CardDescription>{t("connectedAgents.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        <PanelTableFrame columns={connectedAgentsColumns(t)} />
      </CardContent>
    </Card>
  );
}
