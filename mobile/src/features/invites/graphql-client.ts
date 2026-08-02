import type { ApolloClient } from "@apollo/client";
import {
  MobileAcceptWorkspaceInviteDocument,
  MobileWorkspacesDocument,
} from "@/generated-graphql";
import type {
  AcceptedWorkspace,
  InviteAcceptanceClient,
} from "./invite-controller";

const workspaceIdPattern = /^tea-[a-z0-9]+$/;

export class ApolloInviteAcceptanceClient implements InviteAcceptanceClient {
  constructor(private readonly client: ApolloClient) {}

  async accept(token: string, signal: AbortSignal): Promise<AcceptedWorkspace> {
    const result = await this.client.mutate({
      mutation: MobileAcceptWorkspaceInviteDocument,
      variables: { token },
      fetchPolicy: "no-cache",
      context: { fetchOptions: { signal } },
    });
    const accepted = result.data?.acceptWorkspaceInvite;
    const id = accepted?.workspaceId?.trim();
    if (!id || !workspaceIdPattern.test(id)) {
      throw new Error("invite acceptance returned no workspace identifier");
    }
    return {
      id,
      name: accepted?.workspaceName?.trim() || null,
      role: accepted?.role?.trim() || null,
    };
  }
}

export async function refreshInviteWorkspaces(
  client: ApolloClient,
): Promise<void> {
  await client.query({
    query: MobileWorkspacesDocument,
    fetchPolicy: "network-only",
  });
}
