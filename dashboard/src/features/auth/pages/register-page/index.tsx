import { useNavigate, useRouter, useSearch } from "@tanstack/react-router";
import { Registration } from "@ory/elements-react/theme";
import { useOryFlow, clearStoredOryFlow } from "@/common/hooks/use-ory-flow";
import { oryConfig, oryHideCardLogo } from "@/common/lib/ory/config";
import { Skeleton } from "@/common/components/ui/skeleton";
import { AuthPageShell } from "@/features/auth/components/auth-page-shell";
import { AUTH_FEATURES } from "@/features/auth/components/auth-page-shell/auth-features";

export default function RegisterPage() {
  const navigate = useNavigate();
  const router = useRouter();
  const search = useSearch({ from: "/auth/sign-up" });
  const flow = useOryFlow("registration", search.flow);

  return (
    <AuthPageShell
      title="Create your account"
      subtitle="Enter your details to get started"
      features={AUTH_FEATURES}
    >
      {flow ? (
        <Registration
          flow={flow}
          config={oryConfig}
          components={oryHideCardLogo}
          onSuccess={async () => {
            clearStoredOryFlow("registration");
            // The root route's beforeLoad cached the (unauthenticated)
            // session on first load — force it to refetch before navigating
            // to an authenticated route, or requireAuth bounces us right
            // back to /auth/login on stale context.
            await router.invalidate();
            void navigate({ to: "/" });
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
