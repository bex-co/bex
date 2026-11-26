import { useCallback } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import {
  SendWorkspaceTwoFactorRemindersDocument,
  WorkspaceNonCompliantMembersDocument,
} from "@/graphql/definitions";
import type { WorkspaceLoginRequirement } from "@/common/apollo/auth-error-link";
import { graphQLErrorCodes } from "@/common/lib/graphql-error";
import { LOGIN_REQUIREMENT_ENUM } from "@/features/workspaces/hooks/use-login-requirement-impact";

/** A member who does not meet an enforced login requirement right now. */
export interface NonCompliantMemberView {
  userId: string;
  email: string;
  name: string;
}

/**
 * The read behind the notice under an enforced requirement (w1/m647): still
 * reading, could not read (shown as nothing, never as "All members"), or the
 * members, empty when every member meets it.
 */
type NonCompliantMembersState =
  | { status: "loading" }
  | { status: "error" }
  | { status: "ready"; members: readonly NonCompliantMemberView[] };

/**
 * Who in the workspace does not meet `requirement` right now, through
 * bex-api's admin-only `workspaceNonCompliantMembers`, read only while
 * `enabled` (an admin's card of an enforced requirement). A cached answer
 * shows at once and is refreshed, so a return visit draws no placeholder.
 */
export function useWorkspaceNonCompliantMembers(
  workspaceId: string,
  requirement: WorkspaceLoginRequirement,
  enabled: boolean,
): NonCompliantMembersState {
  const { data, error } = useQuery(WorkspaceNonCompliantMembersDocument, {
    variables: {
      workspaceId,
      requirement: LOGIN_REQUIREMENT_ENUM[requirement],
    },
    skip: !enabled,
    fetchPolicy: "cache-and-network",
  });
  const members = data?.workspaceNonCompliantMembers;
  if (members) return { status: "ready", members };
  if (error) return { status: "error" };
  return { status: "loading" };
}

/** The daily limit's code: reminders already went out today (UTC). */
const REMINDER_LIMIT_CODE = "WORKSPACE_2FA_REMINDER_LIMIT";

/** What one Send reminders came to. */
type ReminderOutcome = "sent" | "limit" | "error";

/**
 * Render's Send reminders (w1/m647): bex-api's
 * `sendWorkspaceTwoFactorReminders`, at most one batch per workspace per UTC
 * day, which answers `limit` once today's batch has gone.
 */
export function useSendTwoFactorReminders(workspaceId: string): {
  send: () => Promise<ReminderOutcome>;
  sending: boolean;
} {
  const [mutate, { loading }] = useMutation(
    SendWorkspaceTwoFactorRemindersDocument,
  );
  const send = useCallback(async (): Promise<ReminderOutcome> => {
    try {
      await mutate({ variables: { workspaceId } });
      return "sent";
    } catch (err) {
      return graphQLErrorCodes(err).includes(REMINDER_LIMIT_CODE)
        ? "limit"
        : "error";
    }
  }, [mutate, workspaceId]);
  return { send, sending: loading };
}
