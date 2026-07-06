import { useNavigate, useRouter, useSearch } from "@tanstack/react-router";
import { Login } from "@ory/elements-react/theme";
import { useOryFlow } from "@/common/hooks/use-ory-flow";
import { oryConfig } from "@/common/lib/ory/config";
import { Skeleton } from "@/common/components/ui/skeleton";
import { AuthPageShell } from "@/features/auth/components/auth-page-shell";
import { AUTH_FEATURES } from "@/features/auth/components/auth-page-shell/auth-features";

export default function LoginPage() {
  const navigate = useNavigate();
  const router = useRouter();
  const search = useSearch({ from: "/auth/login" });
  const flow = useOryFlow("login", search.flow, search.next || "/");

  return (
    <AuthPageShell
      title="Welcome back"
      subtitle="Sign in to your account"
      features={AUTH_FEATURES}
    >
      {flow ? (
        <Login
          flow={flow}
          config={oryConfig}
          onSuccess={async () => {
            // See register-page: root's beforeLoad cached the (unauthenticated)
            // session on first load — refetch it before navigating.
            await router.invalidate();
            void navigate({ to: search.next ? search.next : "/" });
          }}
        />
      ) : (
        <div className="space-y-4">
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-10 w-full" />
        </div>
      )}
    </AuthPageShell>
  );
}
