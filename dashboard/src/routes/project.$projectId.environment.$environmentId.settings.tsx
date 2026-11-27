import { createFileRoute, redirect } from "@tanstack/react-router";
import { EnvironmentSettingsPageSkeleton } from "@/common/components/route-skeletons";
import {
  loadRouteResource,
  routeResourceTitle,
  titleHead,
  titleLoaderFetchPolicy,
  translatedText,
  type RouteResource,
} from "@/common/lib/document-head";
import { EnvironmentsDocument } from "@/graphql/definitions";
import {
  mapEnvironments,
  type EnvironmentView,
} from "@/features/environments/hooks/use-environments";
import { inEnvironment } from "@/features/environments/lib/link-id";
import { EnvironmentSettingsPage } from "@/features/environments/pages/environment-settings-page";

export interface EnvironmentSettingsLoaderData {
  projectName: string;
  environment: RouteResource<EnvironmentView>;
}

/**
 * Render's environment settings page, `/project/<id>/environment/<envId>/
 * settings`, opened from the section's ••• menu → All settings. The
 * project layout loads the project; this loader reads its environments for
 * the title (`<project> / <env> / Settings`, Render's) and for the cards'
 * first paint. An id the project has no environment for lands on the
 * project's page.
 */
export const Route = createFileRoute(
  "/project/$projectId/environment/$environmentId/settings",
)({
  component: EnvironmentSettingsPage,
  pendingComponent: EnvironmentSettingsPageSkeleton,
  loader: async ({
    context,
    params,
    cause,
    parentMatchPromise,
  }): Promise<EnvironmentSettingsLoaderData> => {
    // The environments read does not wait for the project's.
    const [parent, environment] = await Promise.all([
      parentMatchPromise,
      loadRouteResource(
        () =>
          context.client.query({
            query: EnvironmentsDocument,
            variables: { projectId: params.projectId },
            fetchPolicy: titleLoaderFetchPolicy(cause),
            errorPolicy: "all",
          }),
        (data) =>
          mapEnvironments(data?.environments).find((candidate) =>
            inEnvironment(params.environmentId, candidate.id),
          ),
      ),
    ]);
    const project = parent.loaderData;
    const projectName = project?.state === "ready" ? project.resource.name : "";
    if (environment.state === "not-found" && project?.state === "ready") {
      throw redirect({
        to: "/project/$projectId",
        params: { projectId: params.projectId },
        replace: true,
      });
    }
    return { projectName, environment };
  },
  head: ({ loaderData, match }) =>
    titleHead(
      routeResourceTitle(loaderData?.environment, (environment) => [
        `${loaderData?.projectName ?? ""} / ${environment.name} / ${translatedText("common.navSettings")}`,
      ]),
      match,
    ),
});
