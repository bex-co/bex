import { useEffect } from "react";
import type { LoginFlow, RegistrationFlow } from "@ory/client-fetch";
import {
  rememberSsoProviders,
  ssoProvidersOfNodes,
} from "@/common/lib/ory/sso-providers";

/**
 * Keep the providers the page's login or sign-up flow offers, the one place
 * the browser learns them: the next auth route's loader and pending frame draw
 * them without asking Kratos. A re-authentication or second-factor login flow
 * lists only the methods it asks for, so it is not read.
 */
export function useRememberSsoProviders(
  flow: LoginFlow | RegistrationFlow | null,
): void {
  useEffect(() => {
    if (!flow) return;
    const login = flow as Partial<LoginFlow>;
    if (login.refresh || login.requested_aal === "aal2") return;
    rememberSsoProviders(ssoProvidersOfNodes(flow.ui.nodes));
  }, [flow]);
}
