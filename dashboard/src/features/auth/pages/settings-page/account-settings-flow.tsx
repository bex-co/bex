import type { SettingsFlow } from "@ory/client-fetch";
import { Settings } from "@ory/elements-react/theme";
import { SessionProvider } from "@ory/elements-react/client";
import type { OryFlowComponentOverrides } from "@ory/elements-react";
import {
  KRATOS_PUBLIC_URL,
  useOryConfig,
  oryHideSettingsPageHeader,
} from "@/common/lib/ory/config";
import { orySettingsFormOverrides } from "@/common/lib/ory/auth-form-overrides";
import { settingsTransientPayload } from "@/features/auth/lib/settings-transient-payload";

const settingsComponents: OryFlowComponentOverrides = {
  ...oryHideSettingsPageHeader,
  ...orySettingsFormOverrides,
};

/**
 * Kratos's settings flow as Ory Elements renders it (profile, password,
 * second factors, connected accounts). An unlink names its provider in the
 * flow's transient payload, so bex-api's settings guard refuses only
 * disconnecting Google (w7/085).
 */
export function AccountSettingsFlow({ flow }: { flow: SettingsFlow }) {
  const oryConfig = useOryConfig();
  return (
    // Without baseUrl, SessionProvider guesses the Ory SDK URL as this app's
    // own origin — but Kratos lives on its own host, so its whoami would
    // 404/500 against the dashboard itself.
    <SessionProvider baseUrl={KRATOS_PUBLIC_URL}>
      <Settings
        className="account-settings-flow"
        flow={flow}
        config={oryConfig}
        components={settingsComponents}
        transientPayload={settingsTransientPayload}
      />
    </SessionProvider>
  );
}
