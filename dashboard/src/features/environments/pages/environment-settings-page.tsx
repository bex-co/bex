import type { ReactNode } from "react";
import { getRouteApi } from "@tanstack/react-router";
import { PageFrame, PageHeader } from "@/common/components/page-frame";
import { SECTION_NAVIGATION_TARGET_CLASS } from "@/common/components/section-navigation";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  useEnvironments,
  type EnvironmentView,
} from "@/features/environments/hooks/use-environments";
import { useProtectedPlacement } from "@/features/environments/hooks/use-protected-placement";
import { inEnvironment } from "@/features/environments/lib/link-id";
import { aclBearing } from "@/features/environments/lib/placement";
import {
  EnvironmentConnectionsCard,
  EnvironmentDeleteCard,
  EnvironmentNameCard,
  EnvironmentPermissionsCard,
} from "@/features/environments/components/environment-settings-cards";
import { EnvironmentEnvGroupsCard } from "@/features/environments/components/environment-env-groups-card";
import { EnvironmentNetworkingCard } from "@/features/environments/components/environment-networking-card";
import { EnvironmentSettingsNavigation } from "@/features/environments/components/environment-settings-navigation";
import {
  environmentSettingsSections,
  isolationGated,
  type EnvironmentSettingsSection,
} from "@/features/environments/lib/settings";
import { useWorkspace } from "@/features/workspaces/context";
import { inboundIpRulesPlan } from "@/features/workspaces/types";

const settingsRoute = getRouteApi(
  "/project/$projectId/environment/$environmentId/settings",
);

/**
 * Render's environment settings page: Name, Permissions,
 * Cross-Environment Connections, Environment Groups, Networking and Delete
 * Environment, under "Environment settings". bex has no dedicated static IPs,
 * so Render's Dedicated IPs card has no counterpart (ADR018).
 */
export function EnvironmentSettingsPage() {
  const { t } = useTranslations();
  const { projectId, environmentId } = settingsRoute.useParams();
  const loaded = settingsRoute.useLoaderData();
  // The loader primed the list, and every save writes the environment it
  // answers into it, so the page reads it once rather than polling.
  const { environments } = useEnvironments(projectId, { poll: false });
  const capabilities = useCapabilities();
  const placement = useProtectedPlacement();
  const { currentWorkspace } = useWorkspace();
  // The live list keeps the cards current after a save; the loader's copy
  // covers its first paint and a moment the list does not have it.
  const environment: EnvironmentView | undefined =
    environments.find((candidate) =>
      inEnvironment(environmentId, candidate.id),
    ) ??
    (loaded.environment.state === "ready"
      ? loaded.environment.resource
      : undefined);

  if (!environment) {
    return (
      <PageFrame
        header={<PageHeader title={t("environments.settingsPageTitle")} />}
      >
        <p className="text-sm text-destructive">
          {t("environments.errorBody")}
        </p>
      </PageFrame>
    );
  }

  const adminDenied = capabilities.denied("can_manage");
  const sections = environmentSettingsSections(adminDenied);
  const createReasonKey = capabilities.reasonKey("can_create");
  const createReason = createReasonKey ? t(createReasonKey) : undefined;
  const deleteReason =
    createReason ??
    (adminDenied && aclBearing(environment)
      ? t("environments.protectedDeleteAdminOnly")
      : undefined);
  const groupReason =
    createReason ??
    (placement.restricts(environment) ? placement.reason : undefined);
  const plan = currentWorkspace?.plan;
  const cards: Record<EnvironmentSettingsSection, ReactNode> = {
    name: (
      <EnvironmentNameCard environment={environment} reason={createReason} />
    ),
    permissions: (
      <EnvironmentPermissionsCard
        key={environment.protectedStatus}
        environment={environment}
      />
    ),
    "cross-environment-connections": (
      <EnvironmentConnectionsCard
        key={String(environment.networkIsolationEnabled)}
        environment={environment}
        projectName={loaded.projectName}
        gated={isolationGated(plan, environment.networkIsolationEnabled)}
      />
    ),
    "environment-groups": (
      <EnvironmentEnvGroupsCard
        environment={environment}
        createReason={groupReason}
      />
    ),
    networking: (
      <EnvironmentNetworkingCard
        environment={environment}
        belowScale={!inboundIpRulesPlan(plan)}
      />
    ),
    "delete-environment": (
      <EnvironmentDeleteCard
        environment={environment}
        // A project's only environment is the one Render will not delete
        // (the loader primed the list this reads).
        onlyEnvironment={environments.length === 1}
        deniedReason={deleteReason}
      />
    ),
  };

  return (
    <PageFrame
      header={<PageHeader title={t("environments.settingsPageTitle")} />}
      navigation={<EnvironmentSettingsNavigation sections={sections} />}
    >
      {sections.map((section) => (
        <section
          key={section}
          id={section}
          className={SECTION_NAVIGATION_TARGET_CLASS}
        >
          {cards[section]}
        </section>
      ))}
    </PageFrame>
  );
}
