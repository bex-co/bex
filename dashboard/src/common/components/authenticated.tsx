import { useIsAuthenticated } from "@/common/hooks/use-is-authenticated";

type AuthenticatedProps = {
  children: React.ReactNode;
  fallback?: React.ReactNode;
};

/**
 * Authenticated component with hydration-safe rendering
 * Always renders fallback during SSR and initial client render to prevent hydration mismatches
 * Updates to show authenticated content after hydration if user is authenticated
 */
export const Authenticated = ({ children, fallback }: AuthenticatedProps) => {
  const isAuthenticated = useIsAuthenticated();
  if (!isAuthenticated) {
    return fallback ?? null;
  }
  return children;
};
