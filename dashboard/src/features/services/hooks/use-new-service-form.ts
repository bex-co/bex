import { useCallback, useState } from "react";
import { useInstanceTypes } from "@/features/services/hooks/use-instance-types";
import { useBuildRuntimeFields } from "@/features/services/hooks/use-build-runtime-fields";
import { useServiceNameDraft } from "@/features/services/hooks/use-service-name-draft";
import type { RepoView } from "@/features/services/hooks/use-repos";
import type { SourceTab } from "@/features/services/components/service-source-picker";
import type { ServiceType } from "@/features/services/lib/create-context";
import type { NewServiceForm } from "@/features/services/lib/create-service-input";
import type {
  EnvVarEntry,
  SecretFileEntry,
} from "@/features/services/hooks/use-create-service";

/** The create form's plain fields — those with no rule beyond "hold a value". */
interface PlainFields {
  serviceType: ServiceType;
  tab: SourceTab;
  selectedRepo: RepoView | null;
  gitUrl: string;
  image: string;
  registryCredentialId: string;
  branch: string;
  rootDir: string;
  planOverride: string | null;
  autoDeploy: boolean;
  schedule: string;
  command: string;
  publishPath: string;
  staticBuildCommand: string;
  buildFilterPaths: string[];
  buildFilterIgnored: string[];
  envVars: EnvVarEntry[];
  secretFiles: SecretFileEntry[];
  projectId: string | null;
  environmentId: string | null;
}

/**
 * All create-form state behind one value and one setter.
 *
 * The coupled subsets keep their own hooks — the four build fields bound by the
 * runtime rule (useBuildRuntimeFields) and the name/availability/suggestion
 * chain (useServiceNameDraft) — and this composes them into the single
 * `NewServiceForm` the submit rules read, so the page holds no field state.
 */
export function useNewServiceForm(search: {
  type?: ServiceType;
  projectId?: string;
  environmentId?: string;
}) {
  const { instanceTypes } = useInstanceTypes();
  const build = useBuildRuntimeFields();
  const [fields, setFields] = useState<PlainFields>(() => ({
    serviceType: search.type ?? "web_service",
    tab: "github",
    selectedRepo: null,
    gitUrl: "",
    image: "",
    registryCredentialId: "",
    branch: "",
    rootDir: "",
    planOverride: null,
    autoDeploy: true,
    schedule: "",
    command: "",
    publishPath: "",
    staticBuildCommand: "",
    buildFilterPaths: [],
    buildFilterIgnored: [],
    envVars: [],
    secretFiles: [],
    projectId: search.projectId ?? null,
    environmentId: search.environmentId ?? null,
  }));

  const set = useCallback(
    (
      patch:
        | Partial<PlainFields>
        | ((current: PlainFields) => Partial<PlainFields>),
    ) =>
      setFields((current) => ({
        ...current,
        ...(typeof patch === "function" ? patch(current) : patch),
      })),
    [],
  );

  const name = useServiceNameDraft({
    tab: fields.tab,
    selectedRepo: fields.selectedRepo,
    gitUrl: fields.gitUrl,
    image: fields.image,
    onRepoDefaultBranch: (branch) =>
      set((current) => ({ branch: current.branch || branch })),
  });

  const form: NewServiceForm = {
    ...fields,
    name: name.name,
    runtime: build.runtime,
    buildCommand: build.buildCommand,
    startCommand: build.startCommand,
    dockerfilePath: build.dockerfilePath,
    plan: fields.planOverride ?? instanceTypes[0]?.id ?? "",
  };

  // Switching source tab abandons whatever the previous tab had selected.
  const setTab = useCallback(
    (tab: SourceTab) => set({ tab, selectedRepo: null, branch: "" }),
    [set],
  );

  return { form, set, setTab, build, name, instanceTypes };
}
