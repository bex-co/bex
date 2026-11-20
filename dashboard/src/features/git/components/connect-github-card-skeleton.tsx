import { GithubIcon } from "@/common/components/github-icon";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import {
  ReservedButton,
  ReservedText,
} from "@/common/components/reserved-skeleton";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * The connections read's loading frame (w4/686): DisconnectedState's body and
 * its one connect action (ADR078 §3b) at their ready geometry, the copy
 * reserved but not drawn. A connected workspace settles to its account list
 * over the same single action, so only its list rows are a residual.
 */
export function GitConnectionsSkeleton() {
  const { t } = useTranslations();
  return (
    <div
      aria-hidden="true"
      className="flex flex-col items-start gap-3"
      data-testid="git-connections-skeleton"
    >
      <p className="relative text-sm">
        <ReservedText>{t("git.disconnectedBody")}</ReservedText>
      </p>
      <ReservedButton variant="default">
        <GithubIcon className="size-4" />
        {t("git.connectButton")}
      </ReservedButton>
    </div>
  );
}

/**
 * ConnectGithubCard's frame before it mounts (w3/067): its static title and
 * description, then the connections read's own loading frame. The Connected
 * badge waits for the read, as on the card.
 */
export function ConnectGithubCardSkeleton() {
  const { t } = useTranslations();
  return (
    <Card aria-hidden="true" data-testid="connect-github-card-skeleton">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <GithubIcon className="size-4" />
          {t("git.title")}
        </CardTitle>
        <CardDescription>{t("git.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <GitConnectionsSkeleton />
      </CardContent>
    </Card>
  );
}
