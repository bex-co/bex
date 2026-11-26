import { createContext, useContext } from "react";
import type { SandboxGroupQuery } from "@/graphql/definitions";

export type SandboxGroupView = NonNullable<SandboxGroupQuery["sandboxGroup"]>;

/**
 * What a sandbox group's layout route hands each of its pages (w1/m658): the
 * group it read and the re-read after a change. The layout mounts its
 * `<Outlet/>` only once the group is loaded and its workspace is the
 * selection, so a page never renders without one.
 */
export interface SandboxGroupDetail {
  group: SandboxGroupView;
  refetch: () => void;
}

const SandboxGroupDetailContext = createContext<SandboxGroupDetail | null>(
  null,
);

export const SandboxGroupDetailProvider = SandboxGroupDetailContext.Provider;

/** The open group, for a page under `/sandbox-groups/$sandboxGroupId`. */
export function useSandboxGroupDetail(): SandboxGroupDetail {
  const detail = useContext(SandboxGroupDetailContext);
  if (!detail) {
    throw new Error("useSandboxGroupDetail outside the sandbox group layout");
  }
  return detail;
}
