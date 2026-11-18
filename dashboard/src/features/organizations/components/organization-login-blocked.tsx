import { useTranslations } from "@/common/hooks/use-translations";
import { LoginBlockedCard } from "@/features/workspaces/components/workspace-blocked-frame";
import { GoogleSignInSection } from "@/features/workspaces/components/workspace-google-login-blocked-page";
import { TwoFactorEnrollSection } from "@/features/workspaces/components/workspace-two-factor-blocked-page";
import { loginBlockedCopy } from "@/features/workspaces/lib/login-requirements";
import {
  clearLoginBlocks,
  type OrganizationLoginBlock,
} from "@/features/workspaces/lib/workspace-login-block";

/**
 * What an org page shows a person the org's own login setting refuses
 * (w7/m466, docs/ADR012-auth.md 9c): the workspace blocked pages' card, naming
 * the org and what it requires in all of its workspaces, with the
 * requirement's way back in. It sits under the org page's header, so the org
 * navigation and the sidebar's workspace switcher stay usable; there is no
 * workspace here to leave. The org's name is the one the refusal carried.
 *
 * "Check again" (after enrolling a second factor) drops the recorded block;
 * the page's own check then asks the server again
 * (`useOrganizationLoginCheck`), and the card comes straight back if it
 * still refuses.
 */
export function OrganizationLoginBlocked({
  block,
}: {
  block: OrganizationLoginBlock;
}) {
  const { t } = useTranslations();
  return (
    <div className="w-full max-w-xl" data-testid="organization-login-blocked">
      <LoginBlockedCard
        {...loginBlockedCopy(t, block.requirement, block.organization)}
        heading="h2"
      >
        {block.requirement === "googleLogin" ? (
          <GoogleSignInSection />
        ) : (
          <TwoFactorEnrollSection onRecheck={clearLoginBlocks} />
        )}
      </LoginBlockedCard>
    </div>
  );
}
