import routerMessages from "@/features/router/locales/ru";
import ruCommon from "@/common/locales/ru";
import ruAuth from "@/features/auth/locales/ru";
import ruLogs from "@/features/logs/locales/ru";
import ruMetrics from "@/features/metrics/locales/ru";
import ruServices from "@/features/services/locales/ru";
import ruDatabases from "@/features/databases/locales/ru";
import ruKeyValue from "@/features/keyvalue/locales/ru";
import ruApiKeys from "@/features/api-keys/locales/ru";
import ruWorkspaces from "@/features/workspaces/locales/ru";
import ruAvatars from "@/features/avatars/locales/ru";
import ruTeam from "@/features/team/locales/ru";
import ruUsage from "@/features/usage/locales/ru";
import ruAudit from "@/features/audit/locales/ru";
import ruGit from "@/features/git/locales/ru";
import ruNotifications from "@/features/notifications/locales/ru";
import ruProjects from "@/features/projects/locales/ru";
import ruEnvironments from "@/features/environments/locales/ru";
import ruRegistryCredentials from "@/features/registry-credentials/locales/ru";
import ruConnectedAgents from "@/features/connected-agents/locales/ru";
import ruSessions from "@/features/sessions/locales/ru";
import ruBlueprints from "@/features/blueprints/locales/ru";
import ruEnvGroups from "@/features/env-groups/locales/ru";
import ruWebhooks from "@/features/webhooks/locales/ru";
import ruDeploys from "@/features/deploys/locales/ru";
import ruSSHKeys from "@/features/ssh-keys/locales/ru";
import ruInvites from "@/features/invites/locales/ru";
import ruAgentSessions from "@/features/agent-sessions/locales/ru";
import ruCapabilities from "@/features/capabilities/locales/ru";
import ruOnboarding from "@/features/onboarding/locales/ru";
import ruAgentEnvironment from "@/features/agent-environment/locales/ru";
import ruAgentAutomations from "@/features/agent-automations/locales/ru";
import ruSandboxGroups from "@/features/sandbox-groups/locales/ru";
import ruSandboxGroupPages from "@/features/sandbox-group-pages/locales/ru";
import ruBuildSources from "@/features/build-sources/locales/ru";
import ruLegal from "@/features/legal/locales/ru";
import ruOrganizations from "@/features/organizations/locales/ru";
import ruOryCopy from "@/features/auth/ory-copy/ru";
import { extractMessages } from "./index";

/**
 * The flattened `ru` `{ key: message }` catalog, in its **own module** so it is
 * lazy-loaded (`import("./resources-ru")`) rather than bundled into the
 * always-mounted entry chunk (w9/m60 t003). An English session never downloads
 * these strings; a ru session pulls this one async chunk before
 * `changeLanguage("ru")` resolves (see `ensureLanguage` in `init.ts`). On the
 * server the import resolves synchronously inside the root `beforeLoad`, so a
 * ru-preference session still SSRs fully translated with no hydration mismatch.
 */
const ru: Record<string, string> = {
  ...extractMessages(ruCommon),
  ...extractMessages(routerMessages),
  ...extractMessages(ruAuth),
  ...extractMessages(ruLogs),
  ...extractMessages(ruMetrics),
  ...extractMessages(ruServices),
  ...extractMessages(ruDatabases),
  ...extractMessages(ruKeyValue),
  ...extractMessages(ruApiKeys),
  ...extractMessages(ruWorkspaces),
  ...extractMessages(ruAvatars),
  ...extractMessages(ruTeam),
  ...extractMessages(ruUsage),
  ...extractMessages(ruAudit),
  ...extractMessages(ruGit),
  ...extractMessages(ruNotifications),
  ...extractMessages(ruProjects),
  ...extractMessages(ruEnvironments),
  ...extractMessages(ruRegistryCredentials),
  ...extractMessages(ruConnectedAgents),
  ...extractMessages(ruSessions),
  ...extractMessages(ruBlueprints),
  ...extractMessages(ruEnvGroups),
  ...extractMessages(ruWebhooks),
  ...extractMessages(ruDeploys),
  ...extractMessages(ruSSHKeys),
  ...extractMessages(ruInvites),
  ...extractMessages(ruAgentSessions),
  ...extractMessages(ruCapabilities),
  ...extractMessages(ruOnboarding),
  ...extractMessages(ruAgentEnvironment),
  ...extractMessages(ruAgentAutomations),
  ...extractMessages(ruSandboxGroups),
  ...extractMessages(ruSandboxGroupPages),
  ...extractMessages(ruBuildSources),
  ...extractMessages(ruLegal),
  ...extractMessages(ruOrganizations),
  ...ruOryCopy,
};

export default ru;
