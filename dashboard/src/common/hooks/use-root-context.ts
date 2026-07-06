import { useRouteContext } from "@tanstack/react-router";

export function useRootContext() {
  return useRouteContext({ from: "__root__" });
}
