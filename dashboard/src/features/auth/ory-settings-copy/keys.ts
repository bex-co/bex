/**
 * Ory Elements' own copy for the account settings flow's cards (w3/067).
 *
 * `<Settings>` words its cards from Ory's bundled catalog (`OryLocales`), which
 * lives in the settings page's chunk. The flow's loading frame
 * (`AccountSettingsFlowSkeleton`) paints before that chunk arrives, and it
 * reserves this copy invisibly so each card wraps, and so stands as tall, as
 * the ready one does at the same width and in the same language.
 *
 * So each language's entries are mirrored here and merged into that language's
 * i18n resources: `en` eagerly, the rest with their lazy catalogs. They sit
 * outside `locales/` because they are Ory's words, not the dashboard's (Ory's
 * zh says 您), and `ory-settings-copy.test.ts` keeps every mirror equal to
 * `OryLocales`, so an `@ory/elements-react` upgrade that rewords a card fails
 * the suite until the mirrors are regenerated from it.
 *
 * Maps each mirrored key to the Ory catalog key it copies.
 */
export const ORY_SETTINGS_COPY_SOURCE = {
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
} as const;

export type OrySettingsCopy = Record<
  keyof typeof ORY_SETTINGS_COPY_SOURCE,
  string
>;
