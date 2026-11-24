import routerMessages from "@/features/router/locales/es";
import esCommon from "@/common/locales/es";
import esAuth from "@/features/auth/locales/es";
import esLogs from "@/features/logs/locales/es";
import esMetrics from "@/features/metrics/locales/es";
import esServices from "@/features/services/locales/es";
import esDatabases from "@/features/databases/locales/es";
import esKeyValue from "@/features/keyvalue/locales/es";
import esApiKeys from "@/features/api-keys/locales/es";
import esWorkspaces from "@/features/workspaces/locales/es";
import esTeam from "@/features/team/locales/es";
import esUsage from "@/features/usage/locales/es";
import esAudit from "@/features/audit/locales/es";
import esGit from "@/features/git/locales/es";
import esNotifications from "@/features/notifications/locales/es";
import esProjects from "@/features/projects/locales/es";
import esEnvironments from "@/features/environments/locales/es";
import esRegistryCredentials from "@/features/registry-credentials/locales/es";
import esConnectedAgents from "@/features/connected-agents/locales/es";
import esSessions from "@/features/sessions/locales/es";
import esBlueprints from "@/features/blueprints/locales/es";
import esEnvGroups from "@/features/env-groups/locales/es";
import esWebhooks from "@/features/webhooks/locales/es";
import esDeploys from "@/features/deploys/locales/es";
import esSSHKeys from "@/features/ssh-keys/locales/es";
import esInvites from "@/features/invites/locales/es";
import esAgentSessions from "@/features/agent-sessions/locales/es";
import esCapabilities from "@/features/capabilities/locales/es";
import esOnboarding from "@/features/onboarding/locales/es";
import esAgentEnvironment from "@/features/agent-environment/locales/es";
import esAgentAutomations from "@/features/agent-automations/locales/es";
import esSandboxGroups from "@/features/sandbox-groups/locales/es";
import esBuildSources from "@/features/build-sources/locales/es";
import esLegal from "@/features/legal/locales/es";
import esOrganizations from "@/features/organizations/locales/es";
import esOryCopy from "@/features/auth/ory-copy/es";
import { extractMessages } from "./index";

/**
 * The flattened `es` `{ key: message }` catalog, in its **own module** so it is
 * lazy-loaded (`import("./resources-es")`) rather than bundled into the
 * always-mounted entry chunk (w9/m60 t003). An English session never downloads
 * these strings; a es session pulls this one async chunk before
 * `changeLanguage("es")` resolves (see `ensureLanguage` in `init.ts`). On the
 * server the import resolves synchronously inside the root `beforeLoad`, so a
 * es-preference session still SSRs fully translated with no hydration mismatch.
 */
const es: Record<string, string> = {
  ...extractMessages(esCommon),
  ...extractMessages(routerMessages),
  ...extractMessages(esAuth),
  ...extractMessages(esLogs),
  ...extractMessages(esMetrics),
  ...extractMessages(esServices),
  ...extractMessages(esDatabases),
  ...extractMessages(esKeyValue),
  ...extractMessages(esApiKeys),
  ...extractMessages(esWorkspaces),
  ...extractMessages(esTeam),
  ...extractMessages(esUsage),
  ...extractMessages(esAudit),
  ...extractMessages(esGit),
  ...extractMessages(esNotifications),
  ...extractMessages(esProjects),
  ...extractMessages(esEnvironments),
  ...extractMessages(esRegistryCredentials),
  ...extractMessages(esConnectedAgents),
  ...extractMessages(esSessions),
  ...extractMessages(esBlueprints),
  ...extractMessages(esEnvGroups),
  ...extractMessages(esWebhooks),
  ...extractMessages(esDeploys),
  ...extractMessages(esSSHKeys),
  ...extractMessages(esInvites),
  ...extractMessages(esAgentSessions),
  ...extractMessages(esCapabilities),
  ...extractMessages(esOnboarding),
  ...extractMessages(esAgentEnvironment),
  ...extractMessages(esAgentAutomations),
  ...extractMessages(esSandboxGroups),
  ...extractMessages(esBuildSources),
  ...extractMessages(esLegal),
  ...extractMessages(esOrganizations),
  ...esOryCopy,
};

export default es;
