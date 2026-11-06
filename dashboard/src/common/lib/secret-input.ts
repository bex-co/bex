/**
 * Props for a masked input that takes a secret (a provider key, an agent
 * secret value). A secret is not a login password: these keep browsers and
 * password managers from offering, saving or filling it. Spread onto an
 * `<Input type="password">`.
 */
export const SECRET_INPUT_PROPS = {
  autoComplete: "new-password",
  "data-1p-ignore": true,
  "data-lpignore": "true",
  "data-bwignore": true,
  spellCheck: false,
} as const;
