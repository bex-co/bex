import { Link } from "@tanstack/react-router";
import { Loader2 } from "lucide-react";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import type { WorkspaceLoginRequirement } from "@/common/apollo/auth-error-link";
import { useTranslations } from "@/common/hooks/use-translations";
import { useGoogleSignIn } from "@/features/auth/hooks/use-google-sign-in";
import type { LoginRequirementRefusalText } from "@/features/workspaces/hooks/use-login-requirement-setter";

/**
 * A login-requirement switch's refusal, beside the switch it came from (a
 * workspace's, w22/m402 and w7/m465, or an org's, w7/m466): the server's coded
 * 409 in words and, for the lock-out guard, the way to fix it on the acting
 * admin's or Owner's own account.
 */
export function LoginRequirementRefusalAlert({
  requirement,
  title,
  error,
}: {
  requirement: WorkspaceLoginRequirement;
  title: string;
  error: LoginRequirementRefusalText;
}) {
  return (
    <Alert variant="destructive">
      <AlertTitle>{title}</AlertTitle>
      <AlertDescription>
        <p>{error.text}</p>
        {error.accountFix ? <AccountFix requirement={requirement} /> : null}
      </AlertDescription>
    </Alert>
  );
}

/**
 * The lock-out guard's way out, on the actor's own account: enroll a second
 * factor in account security, or sign this session in with Google and come
 * back to this page.
 */
function AccountFix({
  requirement,
}: {
  requirement: WorkspaceLoginRequirement;
}) {
  const { t } = useTranslations();
  if (requirement === "googleLogin") return <SignInWithGoogleLink />;
  return (
    <Link to="/settings" hash="security" className="underline">
      {t("workspaces.twoFactorEnrollLink")}
    </Link>
  );
}

function SignInWithGoogleLink() {
  const { t } = useTranslations();
  const { start, busy, failed } = useGoogleSignIn();
  return (
    <>
      <Button
        type="button"
        variant="link"
        className="h-auto p-0 text-destructive underline"
        disabled={busy}
        onClick={() => void start()}
      >
        {busy ? <Loader2 className="animate-spin" /> : null}
        {t("workspaces.googleLoginSignIn")}
      </Button>
      {failed ? <p>{t("workspaces.googleLoginSignInError")}</p> : null}
    </>
  );
}
