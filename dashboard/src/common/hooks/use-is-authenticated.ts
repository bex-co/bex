import { useRootContext } from "@/common/hooks/use-root-context";

export function useIsAuthenticated(): boolean {
  const { session } = useRootContext();
  return session != null;
}
