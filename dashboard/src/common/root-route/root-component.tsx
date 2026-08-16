import { TanStackRouterDevtoolsPanel } from "@tanstack/react-router-devtools";
import { RootProvider } from "@/common/providers/root-provider";
import { DashboardLayout } from "@/common/components/dashboard-layout";
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
  // rather than blanking the viewport.
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
      {chrome ? (
        <DashboardLayout>
          <Outlet />
        </DashboardLayout>
      ) : (
        <Outlet />
      )}
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
