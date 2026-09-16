import { useQuery, useMutation } from "@apollo/client/react";
import { useWorkspace } from "@/features/workspaces/context";
import {
  RouterOverviewDocument,
  CreateRouterKeyDocument as CreateDocument,
  UpdateRouterKeyDocument as UpdateDocument,
  DeleteRouterKeyDocument as DeleteDocument,
  type RouterOverviewQuery,
} from "@/graphql/definitions";
export type RouterOverview = NonNullable<RouterOverviewQuery["routerOverview"]>;
export type RouterKey = RouterOverview["keys"][number];
export type RouterKeyOptions = Omit<
  NonNullable<RouterKey["options"]>,
  "__typename"
>;

export function useRouterOverview() {
  const { currentWorkspaceId } = useWorkspace();
  const ownerId = currentWorkspaceId ?? "";
  const query = useQuery(RouterOverviewDocument, {
    variables: { ownerId },
    skip: !ownerId,
    fetchPolicy: "no-cache",
    pollInterval: 30_000,
  });
  const [create] = useMutation(CreateDocument);
  const [update] = useMutation(UpdateDocument);
  const [remove] = useMutation(DeleteDocument);
  return {
    ...query,
    overview: query.error
      ? undefined
      : (query.data?.routerOverview ?? undefined),
    ownerId,
    create: async (name: string) => {
      const result = await create({ variables: { ownerId, name } });
      if (!result.data?.createRouterKey)
        throw new Error("Router mutation failed");
      // The mutation already succeeded; a refresh error belongs to the page retry.
      await query.refetch().catch(() => undefined);
    },
    update: async (id: string, name: string, options: RouterKeyOptions) => {
      const result = await update({
        variables: { ownerId, id, name, ...options },
      });
      if (!result.data?.updateRouterKey)
        throw new Error("Router mutation failed");
      // The mutation already succeeded; a refresh error belongs to the page retry.
      await query.refetch().catch(() => undefined);
    },
    remove: async (id: string) => {
      const result = await remove({ variables: { ownerId, id } });
      if (!result.data?.deleteRouterKey)
        throw new Error("Router mutation failed");
      // The mutation already succeeded; a refresh error belongs to the page retry.
      await query.refetch().catch(() => undefined);
    },
  };
}
