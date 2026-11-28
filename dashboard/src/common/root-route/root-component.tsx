import { TanStackRouterDevtoolsPanel } from "@tanstack/react-router-devtools";
import { RootProvider } from "@/common/providers/root-provider";
import { UnsavedChangesProvider } from "@/common/providers/unsaved-changes-provider";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { PaymentSetupGate } from "@/features/onboarding/components/payment-setup-gate";
import { EmailVerificationGate } from "@/features/onboarding/components/email-verification-gate";
import { WorkspaceLoginGate } from "@/features/workspaces/components/workspace-login-gate";
import { AccountThemeSync } from "@/features/appearance/components/account-theme-sync";
import { TanStackDevtools } from "@tanstack/react-devtools";
import { Outlet, useRouter, useMatches } from "@tanstack/react-router";
import { useLanguageHydrationSync } from "@/i18n/use-language-hydration-sync";
import { useRootContext } from "@/common/hooks/use-root-context";
import { useCallback, useEffect } from "react";
import i18n from "@/i18n/init";

export const RootComponent = () => {
  useLanguageHydrationSync();
  const { workspaceId } = useRootContext();
  const router = useRouter();
  // The ONE persistent dashboard shell. Mounting `DashboardLayout` here — above
  // the `<Outlet/>`, once — means the sidebar + header survive every
  // navigation; only the routed content inside swaps. Gated on `staticData.chrome`
  // so auth / health / redirect-shim / 404 routes stay bare. Between two chrome
  // routes the `<DashboardLayout>` element holds its slot, so React reconciles
  // it (no remount) and the router's pending fallback paints inside the shell
  // rather than blanking the viewport. `PaymentSetupGate` sits inside the
  // shell, around the routed content only: a workspace the payment gate still
  // refuses is sent to the sign-up wall from any app route (ADR075 D7), while
  // bare routes (auth, the wall itself, consent/device, 404) are never gated.
  // `EmailVerificationGate` runs first and wraps the shell itself: an
  // unverified session (ADR075 D8 revision, w2/m168) goes to
  // /auth/verification before any chrome or app content renders. It shares
  // the same chrome-only mount, so the bare routes stay ungated.
  // `WorkspaceLoginGate` is the outermost gate around the routed content,
  // inside the shell: a workspace whose login requirement the member does
  // not meet (two-factor authentication, or a Google sign-in) is replaced by
  // one explanatory page there, while the sidebar's workspace switcher stays
  // usable. `AccountThemeSync` binds the signed-in account's theme to the
  // theme provider (w3/m531), so the account's stored theme wins over the
  // browser's copy and every change is written to the account.
  // `UnsavedChangesProvider` is Render's root unsaved-changes warning (w3/118):
  // a navigation away from an edited, unsaved settings row asks first.
  const chrome = useMatches({
    select: (matches) => matches.some((m) => m.staticData?.chrome),
  });
  const invalidate = useCallback(() => {
    void router.invalidate();
  }, [router]);

  // Route heads read the same i18n singleton as visible page copy. Re-running
  // the active matches on a language change updates static and dynamic titles
  // without a client-only document.title side channel.
  useEffect(() => {
    i18n.on("languageChanged", invalidate);
    return () => {
      i18n.off("languageChanged", invalidate);
    };
  }, [invalidate]);

  return (
    <RootProvider
      initialWorkspaceId={workspaceId}
      onWorkspaceChange={invalidate}
    >
      <UnsavedChangesProvider>
        {chrome ? (
          <EmailVerificationGate>
            <AccountThemeSync />
            <DashboardLayout>
              <WorkspaceLoginGate>
                <PaymentSetupGate>
                  <Outlet />
                </PaymentSetupGate>
              </WorkspaceLoginGate>
            </DashboardLayout>
          </EmailVerificationGate>
        ) : (
          <Outlet />
        )}
      </UnsavedChangesProvider>
      {import.meta.env.DEV && (
        <TanStackDevtools
          config={{
            position: "bottom-right",
          }}
          plugins={[
            {
              name: "Tanstack Router",
              render: <TanStackRouterDevtoolsPanel />,
            },
          ]}
        />
      )}
    </RootProvider>
  );
};

export default RootComponent;
