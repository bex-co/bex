import { createFileRoute } from "@tanstack/react-router";
import { SandboxGroupSettingsSkeleton } from "@/features/sandbox-group-pages/components/group-page-skeletons";
import { SandboxGroupSettingsPage } from "@/features/sandbox-group-pages/components/settings-page";
import { sandboxGroupPageHead } from "@/features/sandbox-group-pages/lib/page-head";

/** A sandbox group's Settings page (w1/m658). */
export const Route = createFileRoute("/sandbox-groups_/$sandboxGroupId/settings")({
  head: sandboxGroupPageHead("sandboxGroupPages.navSettings"),
  component: SandboxGroupSettingsPage,
  pendingComponent: SandboxGroupSettingsSkeleton,
});
