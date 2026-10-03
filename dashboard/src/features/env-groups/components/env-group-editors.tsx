import { useTranslations } from "@/common/hooks/use-translations";
import { EnvironmentEditor } from "@/features/services/components/service-environment-editor";
import {
  classifyEnvGroupError,
  envVarKeys,
  secretFileNames,
  useEnvGroupEnvironmentPatch,
  useRevealEnvGroupSecretFile,
  useRevealEnvGroupVar,
} from "@/features/env-groups/hooks/use-env-groups";
import type { EnvGroupView } from "@/features/env-groups/types";

export function EnvGroupEditors({
  group,
  loading,
  error,
  refetch,
}: {
  group: EnvGroupView;
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<unknown>;
}) {
  const { t } = useTranslations();
  const revealEnv = useRevealEnvGroupVar(group.id);
  const revealFile = useRevealEnvGroupSecretFile(group.id);
  const patch = useEnvGroupEnvironmentPatch(group.id, refetch);

  return (
    <EnvironmentEditor
      resourceId={group.id}
      envKeys={envVarKeys(group)}
      secretFileNames={secretFileNames(group)}
      loading={loading}
      errorKind={classifyEnvGroupError(error)}
      revealEnv={revealEnv}
      revealFile={revealFile}
      revision={group.revision}
      saving={patch.saving}
      generateOnServer
      copy={{
        envTitle: t("envGroups.varsTitle"),
        envDescription: t("envGroups.varsDescription"),
        envEmptyTitle: t("envGroups.varsEmptyTitle"),
        envEmptyBody: t("envGroups.varsEmptyBody"),
        secretFilesTitle: t("envGroups.filesTitle"),
        secretFilesDescription: t("envGroups.filesDescription"),
        secretFilesEmptyTitle: t("envGroups.filesEmptyTitle"),
        secretFilesEmptyBody: t("envGroups.filesEmptyBody"),
      }}
      save={(environmentPatch, choice, baseRevision) =>
        patch.save(
          environmentPatch,
          choice === "only" ? "save_only" : choice,
          baseRevision,
        )
      }
      retryRollout={patch.retryRollout}
    />
  );
}
