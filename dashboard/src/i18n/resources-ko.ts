import routerMessages from "@/features/router/locales/ko";
import koCommon from "@/common/locales/ko";
import koAuth from "@/features/auth/locales/ko";
import koLogs from "@/features/logs/locales/ko";
import koMetrics from "@/features/metrics/locales/ko";
import koServices from "@/features/services/locales/ko";
import koDatabases from "@/features/databases/locales/ko";
import koKeyValue from "@/features/keyvalue/locales/ko";
import koApiKeys from "@/features/api-keys/locales/ko";
import koWorkspaces from "@/features/workspaces/locales/ko";
import koAvatars from "@/features/avatars/locales/ko";
import koTeam from "@/features/team/locales/ko";
import koUsage from "@/features/usage/locales/ko";
import koAudit from "@/features/audit/locales/ko";
import koGit from "@/features/git/locales/ko";
import koNotifications from "@/features/notifications/locales/ko";
import koProjects from "@/features/projects/locales/ko";
import koEnvironments from "@/features/environments/locales/ko";
import koRegistryCredentials from "@/features/registry-credentials/locales/ko";
import koConnectedAgents from "@/features/connected-agents/locales/ko";
import koSessions from "@/features/sessions/locales/ko";
import koBlueprints from "@/features/blueprints/locales/ko";
import koEnvGroups from "@/features/env-groups/locales/ko";
import koWebhooks from "@/features/webhooks/locales/ko";
import koDeploys from "@/features/deploys/locales/ko";
import koSSHKeys from "@/features/ssh-keys/locales/ko";
import koInvites from "@/features/invites/locales/ko";
import koAgentSessions from "@/features/agent-sessions/locales/ko";
import koCapabilities from "@/features/capabilities/locales/ko";
import koOnboarding from "@/features/onboarding/locales/ko";
import koAgentEnvironment from "@/features/agent-environment/locales/ko";
import koAgentAutomations from "@/features/agent-automations/locales/ko";
import koSandboxGroups from "@/features/sandbox-groups/locales/ko";
import koBuildSources from "@/features/build-sources/locales/ko";
import koLegal from "@/features/legal/locales/ko";
import koOrganizations from "@/features/organizations/locales/ko";
import koOryCopy from "@/features/auth/ory-copy/ko";
import { extractMessages } from "./index";

/**
 * The flattened `ko` `{ key: message }` catalog, in its **own module** so it is
 * lazy-loaded (`import("./resources-ko")`) rather than bundled into the
 * always-mounted entry chunk (w9/m60 t003). An English session never downloads
 * these strings; a ko session pulls this one async chunk before
 * `changeLanguage("ko")` resolves (see `ensureLanguage` in `init.ts`). On the
 * server the import resolves synchronously inside the root `beforeLoad`, so a
 * ko-preference session still SSRs fully translated with no hydration mismatch.
 */
const ko: Record<string, string> = {
  ...extractMessages(koCommon),
  ...extractMessages(routerMessages),
  ...extractMessages(koAuth),
  ...extractMessages(koLogs),
  ...extractMessages(koMetrics),
  ...extractMessages(koServices),
  ...extractMessages(koDatabases),
  ...extractMessages(koKeyValue),
  ...extractMessages(koApiKeys),
  ...extractMessages(koWorkspaces),
  ...extractMessages(koAvatars),
  ...extractMessages(koTeam),
  ...extractMessages(koUsage),
  ...extractMessages(koAudit),
  ...extractMessages(koGit),
  ...extractMessages(koNotifications),
  ...extractMessages(koProjects),
  ...extractMessages(koEnvironments),
  ...extractMessages(koRegistryCredentials),
  ...extractMessages(koConnectedAgents),
  ...extractMessages(koSessions),
  ...extractMessages(koBlueprints),
  ...extractMessages(koEnvGroups),
  ...extractMessages(koWebhooks),
  ...extractMessages(koDeploys),
  ...extractMessages(koSSHKeys),
  ...extractMessages(koInvites),
  ...extractMessages(koAgentSessions),
  ...extractMessages(koCapabilities),
  ...extractMessages(koOnboarding),
  ...extractMessages(koAgentEnvironment),
  ...extractMessages(koAgentAutomations),
  ...extractMessages(koSandboxGroups),
  ...extractMessages(koBuildSources),
  ...extractMessages(koLegal),
  ...extractMessages(koOrganizations),
  ...koOryCopy,
};

export default ko;
