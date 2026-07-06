import { TanStackRouterDevtoolsPanel } from "@tanstack/react-router-devtools";
import { RootProvider } from "@/common/providers/root-provider";
import { TanStackDevtools } from "@tanstack/react-devtools";
import { Outlet } from "@tanstack/react-router";

export const RootComponent = () => {
  return (
    <RootProvider>
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
