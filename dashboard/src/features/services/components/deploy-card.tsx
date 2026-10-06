import type { ReactNode } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { DeployHookRows } from "@/features/services/components/deploy-hook-section";
import { EditableFieldRow } from "@/features/services/components/editable-field-row";
import { usePreDeployCommand } from "@/features/services/hooks/use-pre-deploy-command";
import { useStartCommand } from "@/features/services/hooks/use-start-command";
import { commandPromptPrefix } from "@/features/services/lib/format";

/**
 * Which command the Deploy card edits: a native build's Start Command, a
 * Dockerfile build's Docker Command, or an Existing Image's Docker Command,
 * which overrides the image's own CMD.
 */
type DeployCommandKind = "start" | "docker" | "image";

const COMMAND_COPY = {
  start: {
    label: "services.startCommandLabel",
    hint: "services.startCommandHint",
    placeholder: "services.startCommandPlaceholder",
    edit: "services.startCommandEdit",
    confirmTitle: "services.startCommandConfirmTitle",
  },
  docker: {
    label: "services.dockerCommandLabel",
    hint: "services.dockerCommandHint",
    placeholder: "services.dockerCommandPlaceholder",
    edit: "services.dockerCommandEdit",
    confirmTitle: "services.dockerCommandConfirmTitle",
  },
  image: {
    label: "services.dockerCommandLabel",
    hint: "services.imageCommandHint",
    placeholder: "services.imageCommandPlaceholder",
    edit: "services.dockerCommandEdit",
    confirmTitle: "services.dockerCommandConfirmTitle",
  },
} as const;

/**
 * The Settings tab's Deploy card (Render's Build/Deploy split, w5/m52): the
 * Pre-Deploy Command, the service's command, Auto-Deploy and the Deploy Hook.
 * A repo-backed service shows it under Build. An Existing Image service, which
 * has no build, shows the same card on its own (w4/m166), without Auto-Deploy:
 * there is no git push to follow (w5/m124).
 */
export function DeployCard({
  serviceId,
  commandKind,
  command,
  rootDir = null,
  preDeployCommand,
  showPreDeployCommand,
  showCommand,
  autoDeploy,
}: {
  serviceId: string;
  commandKind: DeployCommandKind;
  /** spec.startCommand. */
  command: string | null;
  /** spec.rootDir: Pre-Deploy and a native Start Command run from it, so they carry its prompt. */
  rootDir?: string | null;
  /** spec.preDeployCommand; empty means no pre-deploy step (w1/m33). */
  preDeployCommand: string | null;
  /** False for a static site, which runs no container. */
  showPreDeployCommand: boolean;
  /** False for a static site, which runs no container. */
  showCommand: boolean;
  /** The Auto-Deploy row of a repo-backed service. */
  autoDeploy?: ReactNode;
}) {
  const { t } = useTranslations();
  // Choosing what a service runs is can_create; a contributor is refused on
  // save (docs/ADR024-members.md), so the rows say why instead (w9/m84).
  const capabilities = useCapabilities();
  const createDisabled = !capabilities.canCreate;
  const createReasonKey = capabilities.reasonKey("can_create");
  const createReason = createReasonKey ? t(createReasonKey) : undefined;
  const { setStartCommand, busy: commandBusy } = useStartCommand();
  const { setPreDeployCommand, busy: preDeployBusy } = usePreDeployCommand();
  const copy = COMMAND_COPY[commandKind];
  const startCommand = commandKind === "start";

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("services.deploySectionTitle")}</CardTitle>
        <CardDescription>
          {t("services.deploySectionDescription")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        {showPreDeployCommand && (
          <EditableFieldRow
            label={t("services.preDeployLabel")}
            hint={t("services.preDeployHint")}
            value={preDeployCommand ?? ""}
            // Pre-deploy runs from rootDir too: the same "<rootDir>/ $" prompt (w5/m51).
            valuePrefix={commandPromptPrefix(rootDir)}
            placeholder={t("services.preDeployPlaceholder")}
            editLabel={t("services.preDeployEdit")}
            optional
            mono
            busy={preDeployBusy}
            disabled={createDisabled}
            disabledReason={createReason}
            onSave={(value) => setPreDeployCommand(serviceId, value)}
          />
        )}

        {showCommand && (
          <EditableFieldRow
            label={t(copy.label)}
            hint={t(copy.hint)}
            value={command ?? ""}
            // A native Start Command runs from rootDir (Render's "<rootDir>/ $"
            // prompt); a Docker Command overrides the container's CMD and is
            // not a rootDir shell command, so it carries no prompt (w5/m51).
            valuePrefix={
              startCommand ? commandPromptPrefix(rootDir) : undefined
            }
            placeholder={t(copy.placeholder)}
            editLabel={t(copy.edit)}
            optional={!startCommand}
            mono
            busy={commandBusy}
            confirm={{
              title: (value) => t(copy.confirmTitle, { value }),
              body: t("services.startCommandConfirmBody"),
              emptyValue: t("services.startCommandConfirmEmpty"),
            }}
            disabled={createDisabled}
            disabledReason={createReason}
            onSave={(value) => setStartCommand(serviceId, value)}
          />
        )}

        {autoDeploy}

        <div className="space-y-4">
          <div>
            <div className="text-sm font-medium">
              {t("services.deployHookTitle")}
            </div>
            <p className="text-muted-foreground mt-1 text-sm">
              {t("services.deployHookDescription")}
            </p>
          </div>
          <DeployHookRows serviceId={serviceId} />
        </div>
      </CardContent>
    </Card>
  );
}
