import routerMessages from "@/features/router/locales/de";
import deCommon from "@/common/locales/de";
import deAuth from "@/features/auth/locales/de";
import deLogs from "@/features/logs/locales/de";
import deMetrics from "@/features/metrics/locales/de";
import deServices from "@/features/services/locales/de";
import deDatabases from "@/features/databases/locales/de";
import deKeyValue from "@/features/keyvalue/locales/de";
import deApiKeys from "@/features/api-keys/locales/de";
import deWorkspaces from "@/features/workspaces/locales/de";
import deAvatars from "@/features/avatars/locales/de";
import deTeam from "@/features/team/locales/de";
import deUsage from "@/features/usage/locales/de";
import deAudit from "@/features/audit/locales/de";
import deGit from "@/features/git/locales/de";
import deNotifications from "@/features/notifications/locales/de";
import deProjects from "@/features/projects/locales/de";
import deEnvironments from "@/features/environments/locales/de";
import deRegistryCredentials from "@/features/registry-credentials/locales/de";
import deConnectedAgents from "@/features/connected-agents/locales/de";
import deSessions from "@/features/sessions/locales/de";
import deBlueprints from "@/features/blueprints/locales/de";
import deEnvGroups from "@/features/env-groups/locales/de";
import deWebhooks from "@/features/webhooks/locales/de";
import deDeploys from "@/features/deploys/locales/de";
import deSSHKeys from "@/features/ssh-keys/locales/de";
import deInvites from "@/features/invites/locales/de";
import deAgentSessions from "@/features/agent-sessions/locales/de";
import deCapabilities from "@/features/capabilities/locales/de";
import deOnboarding from "@/features/onboarding/locales/de";
import deAgentEnvironment from "@/features/agent-environment/locales/de";
import deAgentAutomations from "@/features/agent-automations/locales/de";
import deSandboxGroups from "@/features/sandbox-groups/locales/de";
import deSandboxGroupPages from "@/features/sandbox-group-pages/locales/de";
import deBuildSources from "@/features/build-sources/locales/de";
import deLegal from "@/features/legal/locales/de";
import deOrganizations from "@/features/organizations/locales/de";
import deOryCopy from "@/features/auth/ory-copy/de";
import { extractMessages } from "./index";

/**
 * The flattened `de` `{ key: message }` catalog, in its **own module** so it is
 * lazy-loaded (`import("./resources-de")`) rather than bundled into the
 * always-mounted entry chunk (w9/m60 t003). An English session never downloads
 * these strings; a de session pulls this one async chunk before
 * `changeLanguage("de")` resolves (see `ensureLanguage` in `init.ts`). On the
 * server the import resolves synchronously inside the root `beforeLoad`, so a
 * de-preference session still SSRs fully translated with no hydration mismatch.
 */
const de: Record<string, string> = {
  ...extractMessages(deCommon),
  ...extractMessages(routerMessages),
  ...extractMessages(deAuth),
  ...extractMessages(deLogs),
  ...extractMessages(deMetrics),
  ...extractMessages(deServices),
  ...extractMessages(deDatabases),
  ...extractMessages(deKeyValue),
  ...extractMessages(deApiKeys),
  ...extractMessages(deWorkspaces),
  ...extractMessages(deAvatars),
  ...extractMessages(deTeam),
  ...extractMessages(deUsage),
  ...extractMessages(deAudit),
  ...extractMessages(deGit),
  ...extractMessages(deNotifications),
  ...extractMessages(deProjects),
  ...extractMessages(deEnvironments),
  ...extractMessages(deRegistryCredentials),
  ...extractMessages(deConnectedAgents),
  ...extractMessages(deSessions),
  ...extractMessages(deBlueprints),
  ...extractMessages(deEnvGroups),
  ...extractMessages(deWebhooks),
  ...extractMessages(deDeploys),
  ...extractMessages(deSSHKeys),
  ...extractMessages(deInvites),
  ...extractMessages(deAgentSessions),
  ...extractMessages(deCapabilities),
  ...extractMessages(deOnboarding),
  ...extractMessages(deAgentEnvironment),
  ...extractMessages(deAgentAutomations),
  ...extractMessages(deSandboxGroups),
  ...extractMessages(deSandboxGroupPages),
  ...extractMessages(deBuildSources),
  ...extractMessages(deLegal),
  ...extractMessages(deOrganizations),
  ...deOryCopy,
};

export default de;
