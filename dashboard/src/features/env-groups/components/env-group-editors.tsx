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
  const revealEnv = useRevealEnvGroupVar(group.id);
  const revealFile = useRevealEnvGroupSecretFile(group.id);
  const patch = useEnvGroupEnvironmentPatch(group.id, group.revision, refetch);

  return (
    <EnvironmentEditor
      resourceId={group.id}
      envKeys={envVarKeys(group)}
      secretFileNames={secretFileNames(group)}
      loading={loading}
      errorKind={classifyEnvGroupError(error)}
      revealEnv={revealEnv}
      revealFile={revealFile}
      saving={patch.saving}
      generateOnServer
      save={(environmentPatch, choice) =>
        patch.save(environmentPatch, choice === "only" ? "save_only" : choice)
      }
      retryRollout={patch.retryRollout}
    />
  );
}
