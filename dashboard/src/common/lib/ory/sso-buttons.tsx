import {
  Children,
  isValidElement,
  useEffect,
  useRef,
  useState,
  type ReactNode,
  type Ref,
} from "react";
import { useIntl, type IntlShape } from "react-intl";
import {
  isUiNodeInputAttributes,
  isUiNodeScriptAttributes,
  type UiNode,
} from "@ory/client-fetch";
import {
  OryFormSsoForm,
  uiTextToFormattedMessage,
  useOryFlow,
  type OryCardContentProps,
  type OryFormSsoRootProps,
  type OryNodeSettingsButton,
  type OryNodeSsoButtonProps,
  type OrySettingsSsoProps,
} from "@ory/elements-react";
import { SsoProviderLogo } from "@/common/components/sso-provider-logos";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  arrangeSsoProviders,
  isSsoNode,
  ssoProviderOf,
  ssoRowColumns,
} from "@/common/lib/ory/sso-providers";
import { cn } from "@/common/lib/utils/utils";

function nodeValue(node: UiNode): string {
  return ssoProviderOf(node) ?? "";
}

/**
 * A provider button's words: Kratos's label in the page's language ("Sign in
 * with GitHub", "Bitbucket entknüpfen"), and the provider's display name from
 * the label's context (its configured `label`: "GitHub", "Bitbucket"), or its
 * id when the label has none.
 */
function providerText(
  node: UiNode,
  intl: IntlShape,
): { label: string; name: string } {
  const { label } = node.meta;
  const provider = (label?.context as { provider?: unknown } | undefined)
    ?.provider;
  const name =
    typeof provider === "string" && provider !== ""
      ? provider
      : nodeValue(node);
  return {
    label: label ? uiTextToFormattedMessage(label, intl) : name,
    name,
  };
}

/** Ory's spinner (`theme/default/components/form/spinner.tsx`). */
function Spinner({ className }: { className?: string }) {
  return (
    <svg
      aria-hidden="true"
      className={cn(
        "pointer-events-none absolute inset-0 m-auto size-8 animate-spin",
        className,
      )}
      viewBox="0 0 24 24"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
    >
      <path
        d="M12 3C10.22 3 8.47991 3.52784 6.99987 4.51677C5.51983 5.50571 4.36628 6.91131 3.68509 8.55585C3.0039 10.2004 2.82567 12.01 3.17294 13.7558C3.5202 15.5016 4.37737 17.1053 5.63604 18.364C6.89472 19.6226 8.49836 20.4798 10.2442 20.8271C11.99 21.1743 13.7996 20.9961 15.4442 20.3149C17.0887 19.6337 18.4943 18.4802 19.4832 17.0001C20.4722 15.5201 21 13.78 21 12"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

/**
 * Ory's social button chrome (`buttonStyles`, intent `social`) at the
 * dashboard's h-9 scale, with shadcn's soft focus ring around Ory's 1 px ring.
 * Padding and type use the classes Ory's own sheet compiles: its reset sits in
 * a later layer than the dashboard's utilities and would zero theirs.
 */
const socialButtonClassName = cn(
  "group relative flex cursor-pointer items-center justify-center gap-3 overflow-hidden rounded-buttons font-medium ring-1 ring-inset",
  "disabled:cursor-not-allowed loading:pointer-events-none loading:cursor-wait",
  "transition-colors duration-100 ease-linear",
  "min-h-9 max-w-[488px] px-4 py-2 text-sm",
  "focus-visible:shadow-[0_0_0_3px_color-mix(in_oklab,var(--ring)_50%,transparent)]",
  "bg-button-social-background-default text-button-social-foreground-default ring-button-social-border-default",
  "hover:bg-button-social-background-hover hover:text-button-social-foreground-hover hover:ring-button-social-border-hover",
  "disabled:bg-button-social-background-disabled disabled:text-button-social-foreground-disabled disabled:ring-button-social-border-disabled",
  "loading:bg-button-social-background-default loading:text-button-social-foreground-default loading:ring-button-social-border-default",
);

/**
 * A provider button: its logo, then its text, always shown. Ory's own
 * (`DefaultButtonSocial`) drops the text once a flow offers three or four
 * providers and reads its logo from a map with no Bitbucket.
 */
function SocialButton({
  node,
  buttonProps,
  isSubmitting,
  showName,
}: OryNodeSsoButtonProps & { showName: boolean }) {
  const { label, name } = providerText(node, useIntl());
  return (
    <button
      className={socialButtonClassName}
      data-testid={`ory/form/node/input/${node.attributes.name}`}
      data-loading={isSubmitting}
      aria-label={label}
      {...buttonProps}
    >
      <span className="relative flex size-4 shrink-0 items-center justify-center group-disabled:opacity-20 group-loading:opacity-20">
        <SsoProviderLogo provider={nodeValue(node)} label={name} size={16} />
      </span>
      {isSubmitting ? (
        <Spinner className="size-6 stroke-button-social-foreground-default" />
      ) : null}
      <span className="truncate group-disabled:opacity-20 group-loading:opacity-20">
        {showName ? name : label}
      </span>
    </button>
  );
}

/**
 * Login and sign-up: the button reads the provider's name ("GitHub",
 * "Bitbucket"), as Render's do; its accessible name is Kratos's whole label
 * in the page's language ("Sign in with GitHub", "Mit GitHub anmelden").
 */
export function SsoSignInButton(props: OryNodeSsoButtonProps) {
  return <SocialButton {...props} showName />;
}

/** Account settings: the button reads Kratos's label ("Link Bitbucket"). */
export function SsoLinkButton(props: OryNodeSsoButtonProps) {
  return <SocialButton {...props} showName={false} />;
}

function SsoRow({
  items,
  ref,
}: {
  items: ReadonlyArray<{ child: ReactNode; provider: string }>;
  ref?: Ref<HTMLDivElement>;
}) {
  return (
    <div ref={ref} className={cn("grid gap-3", ssoRowColumns(items.length))}>
      {items.map(({ child }) => child)}
    </div>
  );
}

/**
 * Login and sign-up's provider buttons (`Form.SsoRoot`), laid out as Render's:
 * GitHub and Google side by side, and GitLab and Bitbucket on a second row
 * that "More options" reveals. Ory hands the root its nodes and one rendered
 * child per node, in the same order; each child keeps Ory's submit wiring.
 */
export function SsoButtonRows({ nodes, children }: OryFormSsoRootProps) {
  const { t } = useTranslations();
  const [expanded, setExpanded] = useState(false);
  const moreRow = useRef<HTMLDivElement>(null);
  useEffect(() => {
    // Only "More options" expands, and it is gone now: focus the first
    // button it revealed.
    if (expanded) moreRow.current?.querySelector("button")?.focus();
  }, [expanded]);

  const buttons: { child: ReactNode; provider: string }[] = [];
  const rest: ReactNode[] = [];
  Children.toArray(children).forEach((child, index) => {
    const provider = ssoProviderOf(nodes[index]);
    if (provider) buttons.push({ child, provider });
    else rest.push(child);
  });
  const { primary, more } = arrangeSsoProviders(
    buttons,
    ({ provider }) => provider,
  );

  return (
    <div className="grid gap-3" data-testid="sso-buttons">
      {rest}
      {primary.length > 0 ? <SsoRow items={primary} /> : null}
      {more.length === 0 ? null : expanded ? (
        <SsoRow items={more} ref={moreRow} />
      ) : (
        <button
          type="button"
          className="cursor-pointer justify-self-center text-sm text-button-link-default-secondary hover:text-button-link-default-secondary-hover focus-visible:underline focus-visible:outline-none"
          onClick={() => setExpanded(true)}
        >
          {t("auth.ssoMoreOptions")}
        </button>
      )}
    </div>
  );
}

/** Ory's `isNodeVisible`: every node but scripts and hidden inputs. */
function isVisibleNode(node: UiNode): boolean {
  if (isUiNodeScriptAttributes(node.attributes)) return false;
  return !(
    isUiNodeInputAttributes(node.attributes) &&
    node.attributes.type === "hidden"
  );
}

/** Render's "Or continue with": a rule each side of the words. */
function ContinueWithDivider() {
  const { t } = useTranslations();
  return (
    <div
      className="flex items-center gap-3 text-sm text-interface-foreground-default-tertiary"
      data-testid="sso-continue-with"
    >
      <hr
        aria-hidden="true"
        className="flex-1 border-interface-border-default-primary"
      />
      <span>{t("auth.ssoContinueWith")}</span>
      <hr
        aria-hidden="true"
        className="flex-1 border-interface-border-default-primary"
      />
    </div>
  );
}

function isSsoForm(part: ReactNode): boolean {
  return isValidElement(part) && part.type === OryFormSsoForm;
}

/**
 * Login and sign-up's card body (`Card.Content`) in Render's order: the
 * e-mail form and its submit button first, then "Or continue with" and the
 * provider buttons. Ory's identifier and method-choice cards hand it their
 * provider form (`OryFormSsoForm`, its own `<form>`) before the e-mail form;
 * this moves it last, so the tab order follows what the page shows. A card
 * with nothing above the providers draws no divider, as Ory's does not.
 */
export function AuthCardContent({ children }: OryCardContentProps) {
  const {
    flow: { ui },
  } = useOryFlow();
  const parts = Children.toArray(children);
  const sso = parts.filter(isSsoForm);
  const visible = ui.nodes.filter(isVisibleNode);
  const divider =
    sso.length > 0 &&
    visible.some(isSsoNode) &&
    visible.some((node) => !isSsoNode(node));
  return (
    <>
      {parts.filter((part) => !isSsoForm(part))}
      {divider ? <ContinueWithDivider key="continue-with" /> : null}
      {sso}
    </>
  );
}

/**
 * Ory's divider (`Card.Divider`), except on the identifier card, where Ory
 * draws it only between its provider buttons and the e-mail form:
 * `AuthCardContent` moves the buttons below the form under "Or continue
 * with", so there it would only rule off the top of the form.
 */
export function AuthCardDivider() {
  const { formState } = useOryFlow();
  if (formState.current === "provide_identifier") return null;
  return <hr className="border-interface-border-default-primary" />;
}

/** Ory's trash icon (`theme/default/assets/icons/trash.svg`). */
function TrashIcon({ className }: { className?: string }) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      fill="none"
      viewBox="0 0 24 24"
      width={24}
      height={24}
      aria-hidden="true"
      className={className}
    >
      <path
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M4 7h16m-10 4v6m4-6v6M5 7l1 12a2 2 0 0 0 2 2h8a2 2 0 0 0 2-2l1-12M9 7V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v3"
      />
    </svg>
  );
}

/**
 * A linked provider's row: its logo and name, then the unlink button. Ory's
 * (`UnlinkRow`) reads its logo only from its own map, so a linked Bitbucket
 * account showed a grey "B", and names its button in English.
 */
function UnlinkRow({
  button,
  isSubmitting,
}: {
  button: OryNodeSettingsButton;
  isSubmitting: boolean;
}) {
  const intl = useIntl();
  // This row's button started the running submit: it spins until the submit
  // ends, as Ory's does.
  const [clicked, setClicked] = useState(false);
  const [wasSubmitting, setWasSubmitting] = useState(isSubmitting);
  if (wasSubmitting !== isSubmitting) {
    setWasSubmitting(isSubmitting);
    if (!isSubmitting) setClicked(false);
  }
  const value = nodeValue(button);
  const { label, name: provider } = providerText(button, intl);
  return (
    <div className="flex justify-between" data-testid={`sso-linked/${value}`}>
      <div className="flex items-center gap-6">
        <SsoProviderLogo provider={value} label={provider} size={32} />
        <p className="text-sm font-medium text-interface-foreground-default-secondary">
          {provider}
        </p>
      </div>
      <button
        type="submit"
        name={button.buttonProps.name}
        value={button.buttonProps.value}
        onClick={() => {
          button.onClick();
          setClicked(true);
        }}
        disabled={isSubmitting}
        className="relative"
        title={label}
        aria-label={label}
      >
        {clicked ? (
          <Spinner className="relative" />
        ) : (
          <TrashIcon className="text-button-link-default-secondary hover:text-button-link-default-secondary-hover" />
        )}
      </button>
    </div>
  );
}

function inProviderOrder<T extends UiNode>(buttons: readonly T[]): T[] {
  const { primary, more } = arrangeSsoProviders(buttons, nodeValue);
  return [...primary, ...more];
}

/**
 * Account settings' connected accounts (`Form.SsoSettings`): Ory's layout
 * (`DefaultSettingsOidc`), with the dashboard's logos, Render's provider
 * order, and the unlink button named in the page's language.
 */
export function SettingsSsoAccounts({
  linkButtons,
  unlinkButtons,
  isSubmitting,
}: OrySettingsSsoProps) {
  const link = inProviderOrder(linkButtons);
  const unlink = inProviderOrder(unlinkButtons).filter(
    (button) => button.attributes.node_type === "input",
  );
  return (
    <div className="flex flex-col gap-8">
      {link.length > 0 ? (
        <div className="grid grid-cols-1 items-start gap-3 sm:grid-cols-2 md:grid-cols-3">
          {link.map((button) => (
            <SsoLinkButton
              key={nodeValue(button)}
              node={button}
              attributes={button.attributes}
              buttonProps={button.buttonProps}
              isSubmitting={isSubmitting}
              provider={nodeValue(button)}
            />
          ))}
        </div>
      ) : null}
      {link.length > 0 && unlink.length > 0 ? (
        <hr className="border-interface-border-default-primary" />
      ) : null}
      {unlink.map((button) => (
        <UnlinkRow
          key={nodeValue(button)}
          button={button}
          isSubmitting={isSubmitting}
        />
      ))}
    </div>
  );
}
