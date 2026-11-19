/**
 * Ory Elements' own copy for the loading frames of the cards it draws.
 *
 * Ory words its cards from its bundled catalog (`OryLocales`), which lives in
 * the flow page's chunk. The loading frames paint before that chunk arrives,
 * and they reserve this copy invisibly so each card wraps, and so stands as
 * tall, as the ready one does at the same width and in the same language:
 *
 * - the account settings flow's cards (`AccountSettingsFlowSkeleton`, w3/067);
 * - the login, sign-up, forgot-password and verification cards' headers and
 *   footers (`OryCardSkeleton` in `route-skeletons.tsx`, w7/092), whose
 *   subtitle Ory composes from parts (`useOryCardCopy` there), and their
 *   forms' labels and submit buttons (w7/093).
 *
 * So each language's entries are mirrored here and merged into that language's
 * i18n resources: `en` eagerly, the rest with their lazy catalogs. They sit
 * outside `locales/` because they are Ory's words, not the dashboard's (Ory's
 * zh says 您), and `ory-copy.test.ts` keeps every mirror equal to
 * `OryLocales`, so an `@ory/elements-react` upgrade that rewords a card fails
 * the suite until the mirrors are regenerated from it.
 *
 * Maps each mirrored key to the Ory catalog key it copies.
 */
export const ORY_COPY_SOURCE = {
  "orySettings.profileTitle": "settings.profile.title",
  "orySettings.profileDescription": "settings.profile.description",
  "orySettings.passwordTitle": "settings.password.title",
  "orySettings.passwordDescription": "settings.password.description",
  "orySettings.passwordLabel": "identities.messages.1070001",
  "orySettings.oidcTitle": "settings.oidc.title",
  "orySettings.oidcDescription": "settings.oidc.description",
  "orySettings.oidcInfo": "settings.oidc.info",
  "orySettings.lookupSecretTitle": "settings.lookup_secret.title",
  "orySettings.lookupSecretDescription": "settings.lookup_secret.description",
  "orySettings.lookupSecretEnable": "identities.messages.1050008",
  "orySettings.webauthnTitle": "settings.webauthn.title",
  "orySettings.webauthnDescription": "settings.webauthn.description",
  "orySettings.webauthnNameLabel": "identities.messages.1050013",
  "orySettings.webauthnAdd": "identities.messages.1050012",
  "orySettings.webauthnInfo": "settings.webauthn.info",
  "orySettings.totpTitle": "settings.totp.title",
  "orySettings.totpDescription": "settings.totp.description",
  "orySettings.totpSecretLabel": "identities.messages.1050017",
  "orySettings.totpCodeLabel": "identities.messages.1070006",
  "orySettings.totpInfoNotLinked": "settings.totp.info.not-linked",
  "orySettings.save": "identities.messages.1070003",
  // The auth cards' headers (Ory's `useCardHeaderText`) and footers.
  "oryCard.loginTitle": "login.title",
  "oryCard.loginSubtitle": "login.subtitle",
  "oryCard.registrationTitle": "registration.title",
  "oryCard.registrationSubtitle": "registration.subtitle",
  "oryCard.passwordPart": "card.header.parts.password.login",
  "oryCard.oidcPart": "card.header.parts.oidc",
  "oryCard.identifierPart": "card.header.parts.identifier-first",
  "oryCard.or": "misc.or",
  "oryCard.recoveryTitle": "recovery.title",
  "oryCard.recoverySubtitle": "recovery.subtitle",
  "oryCard.verificationTitle": "verification.title",
  "oryCard.verificationSubtitle": "verification.subtitle",
  "oryCard.loginFooterLabel": "login.registration-label",
  "oryCard.loginFooterLink": "login.registration-button",
  "oryCard.registrationFooterLabel": "registration.login-label",
  "oryCard.registrationFooterLink": "registration.login-button",
  // Their forms' words (w7/093): login's link beside the e-mail label, the
  // submit buttons, and forgot-password's and verification's e-mail label.
  // Login's password label is `orySettings.passwordLabel`, and the sign-in and
  // sign-up e-mail and name labels are the dashboard's (`useOryConfig`).
  "oryCard.recoverAccount": "forms.label.recover-account",
  "oryCard.loginSubmit": "identities.messages.1010022",
  "oryCard.registrationSubmit": "identities.messages.1040001",
  "oryCard.emailLabel": "identities.messages.1070007",
  "oryCard.continue": "identities.messages.1070009",
} as const;

export type OryCopy = Record<keyof typeof ORY_COPY_SOURCE, string>;
