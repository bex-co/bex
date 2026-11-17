import { Plus } from "lucide-react";
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { PanelTableFrame } from "@/common/components/panel-states";
import { ReservedButton } from "@/common/components/reserved-skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { apiKeysColumns } from "@/features/api-keys/lib/api-keys-columns";

/**
 * ApiKeysPanel's frame before it mounts (w3/067): its static title and
 * description (the CLI guide link's words included), the Create button's box,
 * then the table frame the panel draws while its keys load. Each key past the
 * first is a data-dependent row.
 */
export function ApiKeysPanelSkeleton() {
  const { t } = useTranslations();
  return (
    <Card aria-hidden="true" data-testid="api-keys-panel-skeleton">
      <CardHeader>
        <CardTitle>{t("apiKeys.title")}</CardTitle>
        <CardDescription>
          {t("apiKeys.description")}{" "}
          <span className="underline underline-offset-4">
            {t("apiKeys.cliGuide")}
          </span>
        </CardDescription>
        <CardAction>
          <ReservedButton variant="outline" size="sm">
            <Plus /> {t("apiKeys.create")}
          </ReservedButton>
        </CardAction>
      </CardHeader>
      <CardContent>
        <PanelTableFrame columns={apiKeysColumns(t)} />
      </CardContent>
    </Card>
  );
}
