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
import { sshKeysColumns } from "@/features/ssh-keys/lib/ssh-keys-columns";

/**
 * SSHKeysPanel's frame before it mounts (w3/067): its static title and
 * description, the Add button's box, then the table frame the panel draws
 * while its keys load. Each key past the first is a data-dependent row.
 */
export function SSHKeysPanelSkeleton() {
  const { t } = useTranslations();
  return (
    <Card aria-hidden="true" data-testid="ssh-keys-panel-skeleton">
      <CardHeader>
        <CardTitle>{t("sshKeys.title")}</CardTitle>
        <CardDescription>{t("sshKeys.description")}</CardDescription>
        <CardAction>
          <ReservedButton variant="outline" size="sm">
            <Plus />
            {t("sshKeys.add")}
          </ReservedButton>
        </CardAction>
      </CardHeader>
      <CardContent>
        <PanelTableFrame
          columns={sshKeysColumns(t)}
          actionsLabel={t("sshKeys.actions")}
        />
      </CardContent>
    </Card>
  );
}
