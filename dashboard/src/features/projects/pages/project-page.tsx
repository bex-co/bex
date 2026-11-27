import { useCallback, useEffect, useRef, useState } from "react";
import {
  getRouteApi,
  useNavigate,
  useParams,
  useRouter,
} from "@tanstack/react-router";
import { PageScroll } from "@/common/components/page-frame";
import { ResourceLoadError } from "@/common/components/resource-load-error";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useServiceLifecycle } from "@/features/services/hooks/use-service-lifecycle";
import { useGroupedResources } from "@/features/projects/hooks/use-grouped-resources";
import { useProjectResourceCatalog } from "@/features/projects/hooks/use-project-resource-catalog";
import {
  AddEnvironmentButton,
  ProjectPageHeader,
} from "@/features/projects/components/project-page-header";
import { EnvironmentsPanel } from "@/features/environments/components/environments-panel";
import { namesSection } from "@/features/environments/lib/sections";
import { NewEnvironmentDialog } from "@/features/environments/components/new-environment-dialog";
import { useEnvironments } from "@/features/environments/hooks/use-environments";
import { ProjectBuildSourcesSection } from "@/features/build-sources/components/project-build-sources-section";
import { useWorkspace } from "@/features/workspaces/context";
import { environmentsPerProjectLimit } from "@/features/workspaces/types";

const projectRoute = getRouteApi("/project/$projectId");

/** How long Render outlines the section its environment URL names. */
const FOCUS_OUTLINE_MS = 2000;

/**
 * A project's page, as Render's (dashboard.render.com/project/{id}, w1/m682):
 * Render's header over every environment of the project, each as its own
 * section, then bex's Build Sources. Render's
 * `/project/<id>/environment/<envId>` is this page with that section
 * scrolled to and outlined for two seconds; the user's next scroll after that
 * puts `/project/<id>` back in the address bar, as Render's does. Renaming is
 * Project Settings' (the name links there); delete lives there too.
 */
export function ProjectPage() {
  const { projectId } = projectRoute.useParams();
  const environmentParam = useParams({
    strict: false,
    select: (params) => params.environmentId,
  });
  const router = useRouter();
  const navigate = useNavigate();
  const { t } = useTranslations();

  const {
    services,
    databases,
    keyValues,
    refetchServices,
    refetchDatabases,
    refetchKeyValues,
    catalog,
  } = useProjectResourceCatalog();
  const { pending, run } = useServiceLifecycle({ refetch: refetchServices });
  const projectResult = projectRoute.useLoaderData();
  const project =
    projectResult.state === "ready" ? projectResult.resource : undefined;
  const { groups } = useGroupedResources({
    projects: project ? [project] : [],
    services,
    databases,
    keyValues,
  });
  const group = groups.find((g) => g.id === projectId);

  // The panel's own read polls; this one only counts for the header.
  const { environments, loading: environmentsLoading } = useEnvironments(
    projectId,
    { poll: false },
  );
  const { currentWorkspace } = useWorkspace();
  const capabilities = useCapabilities();
  const createReasonKey = capabilities.reasonKey("can_create");
  const addEnvironmentReason = createReasonKey ? t(createReasonKey) : undefined;
  const [newEnvironmentOpen, setNewEnvironmentOpen] = useState(false);

  const focus = useSectionFocus({
    environmentParam,
    known: environmentsLoading
      ? null
      : (id: string) => namesSection(environments, id),
    resetAddress: useCallback(
      () =>
        void navigate({
          to: "/project/$projectId",
          params: { projectId },
          replace: true,
          resetScroll: false,
        }),
      [navigate, projectId],
    ),
  });

  function refetchAll() {
    void refetchServices();
    void refetchDatabases();
    void refetchKeyValues();
    void router.invalidate();
  }

  const showNotFound = projectResult.state === "not-found";
  const showError = projectResult.state === "error";
  const showAuthError = projectResult.state === "unauthenticated";

  return (
    <PageScroll onScroll={focus.onScroll}>
      <div className="w-full space-y-10">
        {showAuthError ? (
          <ResourceLoadError variant="unauthenticated" />
        ) : showError ? (
          <p className="text-sm text-destructive">{t("projects.errorTitle")}</p>
        ) : showNotFound ? null : ( // the layout's redirect home is in flight (w9/m55)
          <>
            <ProjectPageHeader
              projectId={projectId}
              name={project?.name ?? projectId}
              addEnvironment={
                <AddEnvironmentButton
                  count={environments.length}
                  limit={environmentsPerProjectLimit(currentWorkspace?.plan)}
                  reason={addEnvironmentReason}
                  onClick={() => setNewEnvironmentOpen(true)}
                />
              }
            />

            <EnvironmentsPanel
              projectId={projectId}
              projectName={project?.name ?? ""}
              services={services}
              databases={databases}
              keyValues={keyValues}
              projectRows={group?.rows ?? []}
              catalog={catalog}
              servicePending={pending}
              onRunServiceAction={run}
              onDatabaseDeleted={refetchAll}
              onKeyValueDeleted={refetchAll}
              focusedId={focus.focusedId}
              onAddEnvironment={() => setNewEnvironmentOpen(true)}
              addEnvironmentReason={addEnvironmentReason}
            />

            <ProjectBuildSourcesSection projectId={projectId} />
          </>
        )}
      </div>
      <NewEnvironmentDialog
        projectId={projectId}
        open={newEnvironmentOpen}
        onOpenChange={setNewEnvironmentOpen}
        onCreated={focus.focus}
      />
    </PageScroll>
  );
}

/**
 * Which section is outlined, and when the address goes back to the project's
 * own URL. A section named by the URL (or just created, as Render outlines a
 * new environment) stays outlined for `FOCUS_OUTLINE_MS` once the page knows
 * it exists; after that, the next scroll resets the address. An id the
 * project has no section for resets it at once, with no error.
 */
function useSectionFocus({
  environmentParam,
  known,
  resetAddress,
}: {
  environmentParam: string | undefined;
  /** Whether an id names a section; `null` until the environments load. */
  known: ((id: string) => boolean) | null;
  resetAddress: () => void;
}) {
  const [focusedId, setFocusedId] = useState<string | null>(
    environmentParam ?? null,
  );
  // A later environment URL (the breadcrumb's switcher) focuses again.
  const [seenParam, setSeenParam] = useState(environmentParam);
  if (seenParam !== environmentParam) {
    setSeenParam(environmentParam);
    if (environmentParam) setFocusedId(environmentParam);
  }
  const armed = useRef(false);
  const loaded = known !== null;
  const exists = focusedId !== null && known !== null && known(focusedId);
  const unknownParam =
    loaded &&
    environmentParam !== undefined &&
    focusedId === environmentParam &&
    !exists;

  // An id the project has no section for: back to the project's own URL.
  useEffect(() => {
    if (unknownParam) resetAddress();
  }, [unknownParam, resetAddress]);

  useEffect(() => {
    if (!exists) return;
    const timer = setTimeout(() => {
      setFocusedId((current) => (current === focusedId ? null : current));
      armed.current = environmentParam !== undefined;
    }, FOCUS_OUTLINE_MS);
    return () => clearTimeout(timer);
  }, [exists, focusedId, environmentParam]);

  return {
    focusedId: exists ? focusedId : null,
    focus: (id: string) => {
      armed.current = false;
      setFocusedId(id);
    },
    onScroll: () => {
      if (!armed.current) return;
      armed.current = false;
      resetAddress();
    },
  };
}
