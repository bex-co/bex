import { useTranslations } from "@/common/hooks/use-translations";
import { classifySecretStoreError } from "@/features/services/lib/secret-store";
import { EnvironmentEditor } from "@/features/services/components/service-environment-editor";
import { useAdminOnlyActions } from "@/features/capabilities/hooks/use-admin-only-actions";
import { isAdminOnlyRefusal } from "@/features/capabilities/lib/admin-only";
import {
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
  // In a protected environment only an Admin views or changes a group's
  // variables and secret files (w4/m594).
  const adminOnly = useAdminOnlyActions("envGroup", group.id);

  return (
    <EnvironmentEditor
      // A draft belongs to one group: another group starts a fresh editor.
      key={group.id}
      resourceId={group.id}
      envKeys={envVarKeys(group)}
      secretFileNames={secretFileNames(group)}
      loading={loading}
      errorKind={classifySecretStoreError(error)}
      refusalBody={
        isAdminOnlyRefusal(error)
          ? t("capabilities.actionAdminOnly")
          : undefined
      }
      writeBlockedReason={adminOnly("update_env_vars")}
      revealBlockedReason={adminOnly("view_env_vars")}
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
