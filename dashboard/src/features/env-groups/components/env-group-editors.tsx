import { useTranslations } from "@/common/hooks/use-translations";
import {
  EnvVarsEditor,
  type SensitiveEditorErrorKind,
} from "@/features/services/components/env-vars-panel";
import { SecretFilesEditor } from "@/features/services/components/secret-files-panel";
import {
  classifyEnvGroupError,
  envVarKeys,
  secretFileNames,
  useEnvGroupSecretFileMutations,
  useEnvGroupVarMutations,
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
  const revealVar = useRevealEnvGroupVar(group.id);
  const revealFile = useRevealEnvGroupSecretFile(group.id);
  const varMutations = useEnvGroupVarMutations(group.id, refetch);
  const fileMutations = useEnvGroupSecretFileMutations(group.id, refetch);
  const errorKind: SensitiveEditorErrorKind | null =
    classifyEnvGroupError(error);

  return (
    <>
      <EnvVarsEditor
        keys={envVarKeys(group)}
        loading={loading}
        errorKind={errorKind}
        reveal={revealVar}
        setVar={varMutations.setVar}
        deleteVar={varMutations.deleteVar}
        busy={varMutations.busy}
        copy={{
          title: t("envGroups.varsTitle"),
          description: t("envGroups.varsDescription"),
          emptyTitle: t("envGroups.varsEmptyTitle"),
          emptyBody: t("envGroups.varsEmptyBody"),
          unavailableTitle: t("envGroups.unavailableTitle"),
          unavailableBody: t("envGroups.unavailableBody"),
          forbiddenTitle: t("envGroups.forbiddenTitle"),
          forbiddenBody: t("envGroups.forbiddenBody"),
          errorTitle: t("envGroups.errorTitle"),
          errorBody: t("envGroups.errorBody"),
          deleteConfirmBody: t("envGroups.varDeleteConfirmBody"),
        }}
      />
      <SecretFilesEditor
        names={secretFileNames(group)}
        loading={loading}
        errorKind={errorKind}
        reveal={revealFile}
        setFile={fileMutations.setFile}
        deleteFile={fileMutations.deleteFile}
        busy={fileMutations.busy}
        copy={{
          title: t("envGroups.filesTitle"),
          description: t("envGroups.filesDescription"),
          emptyTitle: t("envGroups.filesEmptyTitle"),
          emptyBody: t("envGroups.filesEmptyBody"),
          unavailableTitle: t("envGroups.unavailableTitle"),
          unavailableBody: t("envGroups.unavailableBody"),
          forbiddenTitle: t("envGroups.forbiddenTitle"),
          forbiddenBody: t("envGroups.forbiddenBody"),
          errorTitle: t("envGroups.errorTitle"),
          errorBody: t("envGroups.errorBody"),
          deleteConfirmBody: t("envGroups.fileDeleteConfirmBody"),
        }}
      />
    </>
  );
}
