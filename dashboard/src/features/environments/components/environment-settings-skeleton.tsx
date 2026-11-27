import { Layers, Pencil, Trash2 } from "lucide-react";
import { RuleRowSkeleton } from "@/common/components/ip-allow-list-editor";
import {
  ReservedButton,
  ReservedText,
} from "@/common/components/reserved-skeleton";
import { SECTION_NAVIGATION_TARGET_CLASS } from "@/common/components/section-navigation";
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { buttonVariants } from "@/common/components/ui/button";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { TOUCH_TARGET_HEIGHT } from "@/common/lib/touch-target";
import { cn } from "@/common/lib/utils/utils";
import type { EnvironmentSettingsSection } from "@/features/environments/lib/settings";
import {
  CHOICE_RADIO_CLASS,
  ChoiceCardBody,
  ChoiceOptionRow,
  PlanGateRow,
} from "@/features/environments/components/environment-settings-layout";
import {
  CONNECTION_OPTIONS,
  PROTECTION_OPTIONS,
  type ChoiceOptionCopy,
} from "@/features/environments/lib/settings";

const INPUT_BAR = cn("h-9 w-full", TOUCH_TARGET_HEIGHT);

/**
 * The Environment Groups card's table while its env groups load: the ready
 * table's 36px header and one 53px row (an env group's name over its id),
 * both in the card and in the page's pending frame.
 */
export function EnvGroupsTableSkeleton() {
  return (
    <div aria-hidden="true" data-skeleton-region="env-groups-table">
      <div className="flex h-9 items-center gap-4 border-b px-2">
        <Skeleton className="h-3 w-16" />
        <Skeleton className="ml-auto h-3 w-24" />
      </div>
      <div className="flex h-[53px] items-center gap-4 border-b px-2">
        <Skeleton className="h-4 w-32" />
        <Skeleton className="ml-auto h-4 w-16" />
      </div>
    </div>
  );
}

function TitleBar({ children }: { children: string }) {
  return (
    <CardTitle className="relative w-fit">
      <ReservedText>{children}</ReservedText>
    </CardTitle>
  );
}

function DescriptionBar({ children }: { children: string }) {
  return (
    <CardDescription className="relative">
      <ReservedText>{children}</ReservedText>
    </CardDescription>
  );
}

function EditBar({ variant }: { variant: "outline" | "ghost" }) {
  const { t } = useTranslations();
  return (
    <ReservedButton variant={variant} size="sm" className={TOUCH_TARGET_HEIGHT}>
      <Pencil />
      {t("environments.settingsEdit")}
    </ReservedButton>
  );
}

/** A Permissions or Connections card in read mode: both options, unchosen. */
function ChoiceCardSkeleton({
  title,
  paragraphs,
  options,
  gate,
}: {
  title: string;
  paragraphs: string[];
  options: Array<ChoiceOptionCopy<string>>;
  /** The plan gate's sentence and button, below Pro. */
  gate?: { text: string; button: string };
}) {
  const { t } = useTranslations();
  return (
    <Card>
      <CardHeader>
        <TitleBar>{title}</TitleBar>
      </CardHeader>
      <CardContent className="space-y-4">
        <ChoiceCardBody
          explanation={paragraphs.map((text) => (
            <p key={text} className="relative">
              <ReservedText>{text}</ReservedText>
            </p>
          ))}
          choice={
            <div className="space-y-2">
              {options.map((option) => (
                <ChoiceOptionRow
                  key={option.value}
                  radio={
                    <Skeleton
                      className={cn(CHOICE_RADIO_CLASS, "rounded-full")}
                    />
                  }
                  title={<ReservedText>{t(option.titleKey)}</ReservedText>}
                  icon={option.icon}
                  hint={<ReservedText>{t(option.hintKey)}</ReservedText>}
                />
              ))}
            </div>
          }
          actions={gate ? null : <EditBar variant="ghost" />}
        />
        {gate ? (
          <PlanGateRow
            text={<ReservedText>{gate.text}</ReservedText>}
            action={
              <ReservedButton
                variant="default"
                size="sm"
                className={TOUCH_TARGET_HEIGHT}
              >
                {gate.button}
              </ReservedButton>
            }
          />
        ) : null}
      </CardContent>
    </Card>
  );
}

/**
 * The environment settings page's cards while the environment loads
 * (w1/m683), each at its ready card's geometry in read mode: the Name card's
 * field and Edit, the two choice cards with both options and Edit (the
 * Connections card's plan gate below Pro), the Environment Groups search over
 * two rows, the Networking editor over the seeded allow-all pair (its reset
 * below Scale), and Delete Environment. Static copy is reserved with its own
 * translated text; nothing the environment decides is shown.
 */
export function EnvironmentSettingsCardsSkeleton({
  sections,
  connectionsGated,
  belowScale,
}: {
  sections: EnvironmentSettingsSection[];
  connectionsGated: boolean;
  belowScale: boolean;
}) {
  const { t } = useTranslations();
  const cards: Record<EnvironmentSettingsSection, React.ReactNode> = {
    name: (
      <Card>
        <CardHeader>
          <TitleBar>{t("environments.nameCardTitle")}</TitleBar>
          <DescriptionBar>
            {t("environments.nameCardDescription")}
          </DescriptionBar>
          <CardAction>
            <EditBar variant="outline" />
          </CardAction>
        </CardHeader>
        <CardContent className="space-y-2">
          <Skeleton className={cn(INPUT_BAR, "max-w-[35rem]")} />
        </CardContent>
      </Card>
    ),
    permissions: (
      <ChoiceCardSkeleton
        title={t("environments.permissionsCardTitle")}
        paragraphs={[
          t("environments.permissionsDefaultBody"),
          t("environments.permissionsProtectedBody", {
            protectedEnvironment: t("environments.permissionsProtectedLink"),
          }),
        ]}
        options={PROTECTION_OPTIONS}
      />
    ),
    "cross-environment-connections": (
      <ChoiceCardSkeleton
        title={t("environments.connectionsCardTitle")}
        paragraphs={[
          t("environments.connectionsBody"),
          t("environments.connectionsBodyOutside"),
        ]}
        options={CONNECTION_OPTIONS}
        gate={
          connectionsGated
            ? {
                text: t("environments.connectionsPaywall", {
                  plan: t("workspaces.planProName"),
                }),
                button: t("environments.upgrade"),
              }
            : undefined
        }
      />
    ),
    "environment-groups": (
      <Card>
        <CardHeader>
          <TitleBar>{t("environments.envGroupsCardTitle")}</TitleBar>
          <CardAction>
            <ReservedButton
              variant="outline"
              size="sm"
              className={TOUCH_TARGET_HEIGHT}
            >
              <Layers />
              {t("environments.envGroupsNew")}
            </ReservedButton>
          </CardAction>
        </CardHeader>
        <CardContent className="space-y-4">
          <Skeleton className={INPUT_BAR} />
          <EnvGroupsTableSkeleton />
        </CardContent>
      </Card>
    ),
    networking: (
      <Card>
        <CardHeader>
          <TitleBar>{t("environments.networkingCardTitle")}</TitleBar>
          <DescriptionBar>{t("environments.ipAllowListLabel")}</DescriptionBar>
        </CardHeader>
        <CardContent className="space-y-4">
          {belowScale ? (
            <div className="space-y-2">
              <p className="relative text-xs">
                <ReservedText>
                  {t("environments.ipAllowListScalePlanOnly")}
                </ReservedText>{" "}
                {/* The ready Upgrade link is a button: 44px tall on a touch
                    layout, which sets its line's height. */}
                <span
                  data-touch-spacer=""
                  className={cn(
                    buttonVariants({ variant: "link" }),
                    "invisible h-auto p-0 text-xs",
                  )}
                >
                  {t("environments.upgrade")}
                </span>
              </p>
              {/* The ready line is a bare span, so it sits in a line box at
                  the card's own 24px line height. */}
              <div>
                <span className="relative text-sm">
                  <ReservedText>
                    {t("environments.ipAllowListAllowsAll")}
                  </ReservedText>
                </span>
              </div>
            </div>
          ) : (
            <div className="space-y-2">
              <p className="relative text-xs">
                <ReservedText>{t("environments.ipAllowListHint")}</ReservedText>
              </p>
              <div className="space-y-2">
                <RuleRowSkeleton touch />
                <RuleRowSkeleton touch />
              </div>
              <div className="flex flex-wrap gap-2">
                <Skeleton className={cn(INPUT_BAR, "max-w-xs min-w-0")} />
                <Skeleton className={cn(INPUT_BAR, "max-w-xs min-w-0")} />
                <ReservedButton
                  variant="outline"
                  size="sm"
                  className={TOUCH_TARGET_HEIGHT}
                >
                  <Layers className="invisible" />
                  {t("environments.ipAllowListAdd")}
                </ReservedButton>
                <ReservedButton
                  variant="outline"
                  size="sm"
                  className={TOUCH_TARGET_HEIGHT}
                >
                  {t("environments.cancel")}
                </ReservedButton>
                <ReservedButton
                  variant="default"
                  size="sm"
                  className={TOUCH_TARGET_HEIGHT}
                >
                  {t("environments.settingsCardSave")}
                </ReservedButton>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    ),
    "delete-environment": (
      <Card className="border-destructive/50">
        <CardHeader>
          <TitleBar>{t("environments.deleteCardTitle")}</TitleBar>
          <DescriptionBar>
            {t("environments.deleteCardDescription")}
          </DescriptionBar>
        </CardHeader>
        <CardContent className="space-y-6">
          <ReservedButton variant="destructive" className={TOUCH_TARGET_HEIGHT}>
            <Trash2 />
            {t("environments.deleteButton")}
          </ReservedButton>
        </CardContent>
      </Card>
    ),
  };
  return (
    <>
      {sections.map((section) => (
        <section
          key={section}
          data-skeleton-region={section}
          className={SECTION_NAVIGATION_TARGET_CLASS}
        >
          {cards[section]}
        </section>
      ))}
    </>
  );
}
