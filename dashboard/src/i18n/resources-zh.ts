import zhCommon from "@/common/locales/zh";
import zhAuth from "@/features/auth/locales/zh";
import zhLogs from "@/features/logs/locales/zh";
import zhMetrics from "@/features/metrics/locales/zh";
import zhServices from "@/features/services/locales/zh";
import zhDatabases from "@/features/databases/locales/zh";
import zhKeyValue from "@/features/keyvalue/locales/zh";
import zhApiKeys from "@/features/api-keys/locales/zh";
import zhWorkspaces from "@/features/workspaces/locales/zh";
import zhTeam from "@/features/team/locales/zh";
import zhUsage from "@/features/usage/locales/zh";
import zhAudit from "@/features/audit/locales/zh";
import zhGit from "@/features/git/locales/zh";
import zhNotifications from "@/features/notifications/locales/zh";
import zhProjects from "@/features/projects/locales/zh";
import zhEnvironments from "@/features/environments/locales/zh";
import zhRegistryCredentials from "@/features/registry-credentials/locales/zh";
import zhConnectedAgents from "@/features/connected-agents/locales/zh";
import zhSessions from "@/features/sessions/locales/zh";
import zhBlueprints from "@/features/blueprints/locales/zh";
import zhEnvGroups from "@/features/env-groups/locales/zh";
import zhWebhooks from "@/features/webhooks/locales/zh";
import zhDeploys from "@/features/deploys/locales/zh";
import zhSSHKeys from "@/features/ssh-keys/locales/zh";
import zhInvites from "@/features/invites/locales/zh";
import zhAgentSessions from "@/features/agent-sessions/locales/zh";
import zhCapabilities from "@/features/capabilities/locales/zh";
import zhOnboarding from "@/features/onboarding/locales/zh";
import { extractMessages } from "./index";

/**
 * The flattened `zh` `{ key: message }` catalog, in its **own module** so it is
 * lazy-loaded (`import("./resources-zh")`) rather than bundled into the
 * always-mounted entry chunk (w9/m60 t003). An English session never downloads
 * these strings; a zh session pulls this one async chunk before
 * `changeLanguage("zh")` resolves (see `ensureLanguage` in `init.ts`). On the
 * server the import resolves synchronously inside the root `beforeLoad`, so a
 * zh-preference session still SSRs fully translated with no hydration mismatch.
 */
const zh: Record<string, string> = {
  ...extractMessages(zhCommon),
  ...extractMessages(zhAuth),
  ...extractMessages(zhLogs),
  ...extractMessages(zhMetrics),
  ...extractMessages(zhServices),
  ...extractMessages(zhDatabases),
  ...extractMessages(zhKeyValue),
  ...extractMessages(zhApiKeys),
  ...extractMessages(zhWorkspaces),
  ...extractMessages(zhTeam),
  ...extractMessages(zhUsage),
  ...extractMessages(zhAudit),
  ...extractMessages(zhGit),
  ...extractMessages(zhNotifications),
  ...extractMessages(zhProjects),
  ...extractMessages(zhEnvironments),
  ...extractMessages(zhRegistryCredentials),
  ...extractMessages(zhConnectedAgents),
  ...extractMessages(zhSessions),
  ...extractMessages(zhBlueprints),
  ...extractMessages(zhEnvGroups),
  ...extractMessages(zhWebhooks),
  ...extractMessages(zhDeploys),
  ...extractMessages(zhSSHKeys),
  ...extractMessages(zhInvites),
  ...extractMessages(zhAgentSessions),
  ...extractMessages(zhCapabilities),
  ...extractMessages(zhOnboarding),
};

export default zh;
