import { useQuery } from "@apollo/client/react";
import {
  LoginRequirement,
  WorkspaceLoginRequirementImpactDocument,
} from "@/graphql/definitions";
import type { WorkspaceLoginRequirement } from "@/common/apollo/auth-error-link";

/** An API key that would stop working; an org's list names its workspace. */
export interface LoginRequirementImpactKey {
  id: string;
  name: string;
  workspaceName?: string;
}

export interface LoginRequirementImpactMember {
  userId: string;
  email: string;
  name: string;
  /** False: the member keeps access, and only their listed keys stop. */
  losesAccess: boolean;
  apiKeys: readonly LoginRequirementImpactKey[];
  sshKeys: readonly { id: string; name: string; fingerprint: string }[];
}

/**
 * The confirmation's answer: still checking, could not check (shown as such,
 * never as nobody), or the list, empty when nobody would be blocked.
 */
export type LoginRequirementImpactState =
  | { status: "loading" }
  | { status: "error" }
  | {
      status: "ready";
      members: readonly LoginRequirementImpactMember[];
      unattributedApiKeys: readonly LoginRequirementImpactKey[];
    };

/** The requirement as bex-api's `LoginRequirement` enum names it. */
export const LOGIN_REQUIREMENT_ENUM: Readonly<
  Record<WorkspaceLoginRequirement, LoginRequirement>
> = {
  twoFactor: LoginRequirement.TwoFactor,
  googleLogin: LoginRequirement.GoogleLogin,
};

/** A read's result as the confirmation shows it. */
export function loginRequirementImpactState(
  answer:
    | {
        members: readonly LoginRequirementImpactMember[];
        unattributedApiKeys: readonly LoginRequirementImpactKey[];
      }
    | undefined,
  error: unknown,
): LoginRequirementImpactState {
  if (error) return { status: "error" };
  if (!answer) return { status: "loading" };
  return {
    status: "ready",
    members: answer.members,
    unattributedApiKeys: answer.unattributedApiKeys,
  };
}

/**
 * Who turning `requirement` on in the workspace would block (w7/m471),
 * through bex-api's `workspaceLoginRequirementImpact`: read fresh each time
 * the confirmation opens (`open`), never from the cache, and not at all while
 * it is closed.
 */
export function useWorkspaceLoginRequirementImpact(
  workspaceId: string,
  requirement: WorkspaceLoginRequirement,
  open: boolean,
): LoginRequirementImpactState {
  const { data, error } = useQuery(WorkspaceLoginRequirementImpactDocument, {
    variables: {
      workspaceId,
      requirement: LOGIN_REQUIREMENT_ENUM[requirement],
    },
    skip: !open,
    fetchPolicy: "network-only",
  });
  return loginRequirementImpactState(
    data?.workspaceLoginRequirementImpact,
    error,
  );
}
