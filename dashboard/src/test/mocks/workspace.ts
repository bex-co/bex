/** Full workspace context stub for unit tests that render dashboard chrome. */
export function mockWorkspaceContext(workspaceId = "tea-test") {
  const workspace = {
    id: workspaceId,
    name: "Test",
    plan: "hobby" as const,
    role: "ADMIN",
    createdAt: null as string | null,
  };
  return {
    useWorkspace: () => ({
      currentWorkspaceId: workspaceId,
      workspaces: [workspace],
      currentWorkspace: workspace,
      setCurrentWorkspaceId: () => undefined,
      loading: false,
      error: undefined,
      refetch: async () => undefined,
    }),
  };
}
