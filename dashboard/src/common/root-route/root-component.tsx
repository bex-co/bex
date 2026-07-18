import { TanStackRouterDevtoolsPanel } from "@tanstack/react-router-devtools";
import { RootProvider } from "@/common/providers/root-provider";
import { TanStackDevtools } from "@tanstack/react-devtools";
import { Outlet, useRouter } from "@tanstack/react-router";
import { useLanguageHydrationSync } from "@/i18n/use-language-hydration-sync";
import { useRootContext } from "@/common/hooks/use-root-context";
import { useCallback, useEffect } from "react";
import i18n from "@/i18n/init";

export const RootComponent = () => {
  useLanguageHydrationSync();
  const { workspaceId } = useRootContext();
  const router = useRouter();
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
      <Outlet />
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
