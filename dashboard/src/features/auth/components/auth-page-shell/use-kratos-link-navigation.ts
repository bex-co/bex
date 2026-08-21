import { useCallback } from "react";
import { useNavigate } from "@tanstack/react-router";
import { kratosLinkTarget } from "@/features/auth/lib/kratos-link-target";

/**
 * Turns Ory Elements' hardcoded cross-origin self-service links ("Sign up",
 * "Sign in", "Forgot Password?", "Recover Account") into client-side
 * navigations.
 *
 * Ory builds those anchors as `${sdk.url}/self-service/{flow}/browser` and
 * ignores `project.*_ui_url`, so following one leaves the app entirely: a
 * cross-origin request to Kratos, a 303 back, and a second full page load — for
 * a hop between two routes this app already serves. The links are not ours to
 * re-render (the recovery pair lives inside `Node.Label`, whose default the
 * library does not export), so they are intercepted at the click instead.
 *
 * Deliberately conservative: anything that is not a plain left-click on a
 * recognized Kratos self-service link is left completely alone, so the
 * fallback is always today's behavior rather than a broken one.
 */
export function useKratosLinkNavigation() {
  const navigate = useNavigate();

  return useCallback(
    (event: React.MouseEvent<HTMLElement>) => {
      if (event.defaultPrevented) return; // already handled by something nearer
      if (event.button !== 0) return; // middle/right — never ours to swallow
      // Modified clicks mean "open elsewhere" (new tab/window, download).
      if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) {
        return;
      }

      // The click can land on a <span> inside the anchor.
      const anchor = (event.target as Element | null)?.closest?.("a[href]");
      if (!(anchor instanceof HTMLAnchorElement)) return;
      if (anchor.target && anchor.target !== "_self") return;
      if (anchor.hasAttribute("download")) return;
      // A flow *restart* has to reach Kratos to mint a new flow; resuming ours
      // from sessionStorage would make it a no-op. Unreachable today (no
      // identifier_first login), guarded so enabling it stays correct.
      if (anchor.dataset.testid?.endsWith("/action/restart")) return;

      const target = kratosLinkTarget(anchor.href, window.location.origin);
      if (!target) return;

      event.preventDefault();
      // Switched rather than spread so each branch keeps its own typed search.
      switch (target.to) {
        case "/auth/login":
          void navigate({ to: target.to, search: target.search });
          return;
        case "/auth/sign-up":
          void navigate({ to: target.to, search: target.search });
          return;
        case "/auth/forgot-password":
          void navigate({ to: target.to, search: target.search });
          return;
        case "/auth/verification":
          void navigate({ to: target.to, search: target.search });
          return;
      }
    },
    [navigate],
  );
}
