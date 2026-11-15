import routerMessages from "@/features/router/locales/pt";
import ptCommon from "@/common/locales/pt";
import ptAuth from "@/features/auth/locales/pt";
import ptLogs from "@/features/logs/locales/pt";
import ptMetrics from "@/features/metrics/locales/pt";
import ptServices from "@/features/services/locales/pt";
import ptDatabases from "@/features/databases/locales/pt";
import ptKeyValue from "@/features/keyvalue/locales/pt";
import ptApiKeys from "@/features/api-keys/locales/pt";
import ptWorkspaces from "@/features/workspaces/locales/pt";
import ptTeam from "@/features/team/locales/pt";
import ptUsage from "@/features/usage/locales/pt";
import ptAudit from "@/features/audit/locales/pt";
import ptGit from "@/features/git/locales/pt";
import ptNotifications from "@/features/notifications/locales/pt";
import ptProjects from "@/features/projects/locales/pt";
import ptEnvironments from "@/features/environments/locales/pt";
import ptRegistryCredentials from "@/features/registry-credentials/locales/pt";
import ptConnectedAgents from "@/features/connected-agents/locales/pt";
import ptSessions from "@/features/sessions/locales/pt";
import ptBlueprints from "@/features/blueprints/locales/pt";
import ptEnvGroups from "@/features/env-groups/locales/pt";
import ptWebhooks from "@/features/webhooks/locales/pt";
import ptDeploys from "@/features/deploys/locales/pt";
import ptSSHKeys from "@/features/ssh-keys/locales/pt";
import ptInvites from "@/features/invites/locales/pt";
import ptAgentSessions from "@/features/agent-sessions/locales/pt";
import ptCapabilities from "@/features/capabilities/locales/pt";
import ptOnboarding from "@/features/onboarding/locales/pt";
import ptAgentEnvironment from "@/features/agent-environment/locales/pt";
import ptAgentAutomations from "@/features/agent-automations/locales/pt";
import ptSandboxGroups from "@/features/sandbox-groups/locales/pt";
import ptLegal from "@/features/legal/locales/pt";
import { extractMessages } from "./index";

/**
 * The flattened `pt` `{ key: message }` catalog, in its **own module** so it is
 * lazy-loaded (`import("./resources-pt")`) rather than bundled into the
 * always-mounted entry chunk (w9/m60 t003). An English session never downloads
 * these strings; a pt session pulls this one async chunk before
 * `changeLanguage("pt")` resolves (see `ensureLanguage` in `init.ts`). On the
 * server the import resolves synchronously inside the root `beforeLoad`, so a
 * pt-preference session still SSRs fully translated with no hydration mismatch.
 */
const pt: Record<string, string> = {
  ...extractMessages(ptCommon),
  ...extractMessages(routerMessages),
  ...extractMessages(ptAuth),
  ...extractMessages(ptLogs),
  ...extractMessages(ptMetrics),
  ...extractMessages(ptServices),
  ...extractMessages(ptDatabases),
  ...extractMessages(ptKeyValue),
  ...extractMessages(ptApiKeys),
  ...extractMessages(ptWorkspaces),
  ...extractMessages(ptTeam),
  ...extractMessages(ptUsage),
  ...extractMessages(ptAudit),
  ...extractMessages(ptGit),
  ...extractMessages(ptNotifications),
  ...extractMessages(ptProjects),
  ...extractMessages(ptEnvironments),
  ...extractMessages(ptRegistryCredentials),
  ...extractMessages(ptConnectedAgents),
  ...extractMessages(ptSessions),
  ...extractMessages(ptBlueprints),
  ...extractMessages(ptEnvGroups),
  ...extractMessages(ptWebhooks),
  ...extractMessages(ptDeploys),
  ...extractMessages(ptSSHKeys),
  ...extractMessages(ptInvites),
  ...extractMessages(ptAgentSessions),
  ...extractMessages(ptCapabilities),
  ...extractMessages(ptOnboarding),
  ...extractMessages(ptAgentEnvironment),
  ...extractMessages(ptAgentAutomations),
  ...extractMessages(ptSandboxGroups),
  ...extractMessages(ptLegal),
};

export default pt;
