import routerMessages from "@/features/router/locales/ja";
import jaCommon from "@/common/locales/ja";
import jaAuth from "@/features/auth/locales/ja";
import jaLogs from "@/features/logs/locales/ja";
import jaMetrics from "@/features/metrics/locales/ja";
import jaServices from "@/features/services/locales/ja";
import jaDatabases from "@/features/databases/locales/ja";
import jaKeyValue from "@/features/keyvalue/locales/ja";
import jaApiKeys from "@/features/api-keys/locales/ja";
import jaWorkspaces from "@/features/workspaces/locales/ja";
import jaTeam from "@/features/team/locales/ja";
import jaUsage from "@/features/usage/locales/ja";
import jaAudit from "@/features/audit/locales/ja";
import jaGit from "@/features/git/locales/ja";
import jaNotifications from "@/features/notifications/locales/ja";
import jaProjects from "@/features/projects/locales/ja";
import jaEnvironments from "@/features/environments/locales/ja";
import jaRegistryCredentials from "@/features/registry-credentials/locales/ja";
import jaConnectedAgents from "@/features/connected-agents/locales/ja";
import jaSessions from "@/features/sessions/locales/ja";
import jaBlueprints from "@/features/blueprints/locales/ja";
import jaEnvGroups from "@/features/env-groups/locales/ja";
import jaWebhooks from "@/features/webhooks/locales/ja";
import jaDeploys from "@/features/deploys/locales/ja";
import jaSSHKeys from "@/features/ssh-keys/locales/ja";
import jaInvites from "@/features/invites/locales/ja";
import jaAgentSessions from "@/features/agent-sessions/locales/ja";
import jaCapabilities from "@/features/capabilities/locales/ja";
import jaOnboarding from "@/features/onboarding/locales/ja";
import jaAgentEnvironment from "@/features/agent-environment/locales/ja";
import jaAgentAutomations from "@/features/agent-automations/locales/ja";
import jaSandboxGroups from "@/features/sandbox-groups/locales/ja";
import jaLegal from "@/features/legal/locales/ja";
import { extractMessages } from "./index";

/**
 * The flattened `ja` `{ key: message }` catalog, in its **own module** so it is
 * lazy-loaded (`import("./resources-ja")`) rather than bundled into the
 * always-mounted entry chunk (w9/m60 t003). An English session never downloads
 * these strings; a ja session pulls this one async chunk before
 * `changeLanguage("ja")` resolves (see `ensureLanguage` in `init.ts`). On the
 * server the import resolves synchronously inside the root `beforeLoad`, so a
 * ja-preference session still SSRs fully translated with no hydration mismatch.
 */
const ja: Record<string, string> = {
  ...extractMessages(jaCommon),
  ...extractMessages(routerMessages),
  ...extractMessages(jaAuth),
  ...extractMessages(jaLogs),
  ...extractMessages(jaMetrics),
  ...extractMessages(jaServices),
  ...extractMessages(jaDatabases),
  ...extractMessages(jaKeyValue),
  ...extractMessages(jaApiKeys),
  ...extractMessages(jaWorkspaces),
  ...extractMessages(jaTeam),
  ...extractMessages(jaUsage),
  ...extractMessages(jaAudit),
  ...extractMessages(jaGit),
  ...extractMessages(jaNotifications),
  ...extractMessages(jaProjects),
  ...extractMessages(jaEnvironments),
  ...extractMessages(jaRegistryCredentials),
  ...extractMessages(jaConnectedAgents),
  ...extractMessages(jaSessions),
  ...extractMessages(jaBlueprints),
  ...extractMessages(jaEnvGroups),
  ...extractMessages(jaWebhooks),
  ...extractMessages(jaDeploys),
  ...extractMessages(jaSSHKeys),
  ...extractMessages(jaInvites),
  ...extractMessages(jaAgentSessions),
  ...extractMessages(jaCapabilities),
  ...extractMessages(jaOnboarding),
  ...extractMessages(jaAgentEnvironment),
  ...extractMessages(jaAgentAutomations),
  ...extractMessages(jaSandboxGroups),
  ...extractMessages(jaLegal),
};

export default ja;
