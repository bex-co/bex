import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { EditableFieldRow } from "@/features/services/components/editable-field-row";
import { useStartCommand } from "@/features/services/hooks/use-start-command";

/**
 * The Deploy card of an Existing Image web/private/worker service: its Docker
 * Command, the override an image whose default CMD needs arguments (an echo
 * server, a one-shot CLI) cannot run without. Repo-backed services edit theirs
 * in Build & Deploy; this is the same setStartCommand path for the services
 * that have no build.
 */
export function ImageCommandSection({
  serviceId,
  startCommand,
}: {
  serviceId: string;
  startCommand: string | null | undefined;
}) {
  const { t } = useTranslations();
  const { setStartCommand, busy } = useStartCommand();
  // Choosing what a service runs is can_create, as in Build & Deploy.
  const capabilities = useCapabilities();
  const reasonKey = capabilities.reasonKey("can_create");

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("services.deployTitle")}</CardTitle>
        <CardDescription>
          {t("services.imageDeployDescription")}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <EditableFieldRow
          label={t("services.dockerCommandLabel")}
          hint={t("services.imageCommandHint")}
          value={startCommand ?? ""}
          placeholder={t("services.imageCommandPlaceholder")}
          editLabel={t("services.dockerCommandEdit")}
          optional
          mono
          busy={busy}
          confirm={{
            title: (value) =>
              t("services.dockerCommandConfirmTitle", { value }),
            body: t("services.startCommandConfirmBody"),
            emptyValue: t("services.startCommandConfirmEmpty"),
          }}
          disabled={!capabilities.canCreate}
          disabledReason={reasonKey ? t(reasonKey) : undefined}
          onSave={(value) => setStartCommand(serviceId, value)}
        />
      </CardContent>
    </Card>
  );
}
