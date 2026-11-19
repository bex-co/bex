import { Skeleton } from "@/common/components/ui/skeleton";
import {
  ReservedOryButton,
  ReservedText,
} from "@/common/components/reserved-skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils";

/**
 * The Kratos settings flow's frame while it loads (w3/067): the six cards Ory
 * Elements' `<Settings>` draws for an account with no second factor yet, at
 * their ready geometry under the dashboard's overrides (`style.css`,
 * `.account-settings-flow`). Ory's own copy is reserved invisibly from the
 * mirrored catalog (`ory-copy/keys.ts`), in Ory's font, so every title,
 * description and footer wraps as the ready card does.
 *
 * The page draws it until the flow arrives, and the route's pending frame draws
 * it too, so the two cannot drift apart. Residuals are the account's own state:
 * a linked authenticator app or recovery codes, registered security keys, and
 * how many social providers are listed and whether each is linked (one linked
 * provider's row is drawn; an unlinked one's button is 4 px taller).
 */
export function AccountSettingsFlowSkeleton() {
  const { t } = useTranslations();
  return (
    <div
      aria-hidden="true"
      data-testid="account-settings-flow-skeleton"
      // Ory's font and type scale, and its card rhythm as `style.css` sets it:
      // 16 px between cards, plus the empty messages section Ory draws after
      // the last one.
      className="flex flex-col gap-4 pb-4 text-base [font-family:var(--font-sans-default)]"
    >
      <FlowCard
        group="profile"
        title={t("orySettings.profileTitle")}
        description={t("orySettings.profileDescription")}
        footerButton={t("orySettings.save")}
        lastChildFooter
      >
        <FlowField label={t("auth.profileEmailLabel")} />
        <FlowField label={t("auth.profileNameLabel")} />
      </FlowCard>
      <FlowCard
        group="password"
        title={t("orySettings.passwordTitle")}
        description={t("orySettings.passwordDescription")}
        footerButton={t("orySettings.save")}
        lastChildFooter
      >
        {/* The password input and its show/hide toggle share one bordered box. */}
        <FlowField
          label={t("orySettings.passwordLabel")}
          inputClassName="h-[38px] max-w-none"
        />
      </FlowCard>
      <FlowCard
        group="oidc"
        title={t("orySettings.oidcTitle")}
        description={t("orySettings.oidcDescription")}
        footerText={t("orySettings.oidcInfo")}
      >
        {/* A provider's row: its 32 px logo and name, then the unlink icon. */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-6">
            <Skeleton className="size-8" />
            <Skeleton className="h-5 w-12" />
          </div>
          <Skeleton className="size-6" />
        </div>
      </FlowCard>
      <FlowCard
        group="lookup_secret"
        title={t("orySettings.lookupSecretTitle")}
        description={t("orySettings.lookupSecretDescription")}
        footerButton={t("orySettings.lookupSecretEnable")}
      >
        {/* The codes row, empty until codes exist; it still takes a gap. */}
        <div />
      </FlowCard>
      <FlowCard
        group="webauthn"
        title={t("orySettings.webauthnTitle")}
        description={t("orySettings.webauthnDescription")}
        footerText={t("orySettings.webauthnInfo")}
      >
        <div className="flex flex-col gap-3 sm:flex-row sm:items-end md:max-w-96">
          <div className="flex-1">
            <FlowField label={t("orySettings.webauthnNameLabel")} />
          </div>
          <ReservedOryButton>{t("orySettings.webauthnAdd")}</ReservedOryButton>
        </div>
      </FlowCard>
      <FlowCard
        group="totp"
        title={t("orySettings.totpTitle")}
        description={t("orySettings.totpDescription")}
        footerText={t("orySettings.totpInfoNotLinked")}
        footerButton={t("orySettings.save")}
      >
        <div className="grid grid-cols-1 gap-8 md:grid-cols-2">
          <div className="col-span-full border-t" />
          <div className="flex justify-center rounded-xl bg-muted p-8">
            <Skeleton className="size-44 bg-card" />
          </div>
          <div className="flex flex-col gap-6">
            <FlowField label={t("orySettings.totpSecretLabel")} />
            {/* The six-digit code input is 48 px tall. */}
            <FlowField
              label={t("orySettings.totpCodeLabel")}
              inputClassName="h-12"
            />
          </div>
        </div>
      </FlowCard>
    </div>
  );
}

/**
 * One of Ory's settings cards: a bordered body (its padding and gap as
 * `style.css` sets them) over a muted footer. Ory's footer is at least 72 px
 * tall; the dashboard's 64 px override reaches only a footer that is its form's
 * last child, which the profile and password forms' footers are.
 */
function FlowCard({
  group,
  title,
  description,
  footerText,
  footerButton,
  lastChildFooter = false,
  children,
}: {
  group: string;
  title: string;
  description: string;
  footerText?: string;
  footerButton?: string;
  lastChildFooter?: boolean;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col" data-ory-group={group}>
      <div className="flex flex-col gap-6 rounded-t-xl border border-b-0 bg-card p-6">
        <div className="flex flex-col gap-2">
          <div className="relative w-fit max-w-full font-medium">
            <ReservedText>{title}</ReservedText>
          </div>
          <div className="relative w-fit max-w-full">
            <ReservedText>{description}</ReservedText>
          </div>
        </div>
        {children}
      </div>
      <div
        className={cn(
          // Bars in the muted footer take the card's colour to stay visible.
          "flex items-center justify-between gap-2 rounded-b-xl border bg-muted px-6 py-4 [&_[data-slot=skeleton]]:bg-card",
          lastChildFooter ? "min-h-16" : "min-h-[72px]",
        )}
      >
        {/* Ory keeps the text's span even when it is empty. */}
        <span className="relative">
          {footerText ? <ReservedText>{footerText}</ReservedText> : null}
        </span>
        {footerButton ? (
          <ReservedOryButton>{footerButton}</ReservedOryButton>
        ) : null}
      </div>
    </div>
  );
}

/** A labelled input: the label's line, then the h-9 input Ory caps at 488 px. */
function FlowField({
  label,
  inputClassName,
}: {
  label: string;
  inputClassName?: string;
}) {
  return (
    <div className="flex flex-col gap-1">
      <span className="relative w-fit max-w-full">
        <ReservedText>{label}</ReservedText>
      </span>
      <Skeleton className={cn("h-9 w-full max-w-[488px]", inputClassName)} />
    </div>
  );
}
