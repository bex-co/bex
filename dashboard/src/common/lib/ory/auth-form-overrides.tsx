import { useEffect, useRef, useState } from "react";
import type {
  OryFlowComponentOverrides,
  OryFormRootProps,
  OryNodeInputProps,
} from "@ory/elements-react";
import { Eye, EyeOff } from "lucide-react";
import { oryHideCardLogo } from "@/common/lib/ory/config";
import { cn } from "@/common/lib/utils/utils.ts";

const emailNodeNames = new Set(["identifier", "email", "traits.email"]);

/** Matches Ory Elements' default text-input chrome (theme DefaultInput). */
const textInputClassName = cn(
  "w-full rounded-forms border leading-tight antialiased transition-colors focus:ring-0 focus-visible:outline-none",
  "border-input-border-default bg-input-background-default text-input-foreground-primary",
  "focus-within:border-input-border-focus focus-visible:border-input-border-focus",
  "hover:bg-input-background-hover px-4 py-[13px] hover:border-input-border-hover",
  "placeholder:h-[20px] placeholder:text-input-foreground-tertiary",
  "disabled:border-input-border-disabled disabled:bg-input-background-disabled disabled:text-input-foreground-disabled",
);

/**
 * Password field that keeps a single `id` on the `<input>` and a distinct
 * id on the visibility toggle. Ory Elements' default uses Radix
 * PasswordToggleField, which stamps the same id on both (invalid HTML;
 * ambiguous `label[for]` / `getElementById`).
 */
export function UniqueIdPasswordInput({ inputProps }: OryNodeInputProps) {
  const [visible, setVisible] = useState(false);
  const { id, ...rest } = inputProps;
  const toggleId = id ? `${id}-visibility` : undefined;

  return (
    <div
      className={cn(
        "flex w-full justify-stretch rounded-forms border leading-tight antialiased transition-colors",
        "border-input-border-default bg-input-background-default text-input-foreground-primary",
        "focus-within:border-input-border-focus",
        "not-focus-within:hover:border-input-border-hover hover:bg-input-background-hover",
      )}
      data-disabled={rest.disabled}
    >
      <input
        {...rest}
        id={id}
        required
        type={visible ? "text" : "password"}
        data-testid={`ory/form/node/input/${rest.name}`}
        className={
          "w-full rounded-l-forms rounded-r-none bg-transparent px-4 py-[13px] text-input-foreground-primary placeholder:h-[20px] placeholder:text-input-foreground-tertiary focus:outline-none disabled:bg-input-background-disabled disabled:text-input-foreground-disabled"
        }
      />
      <button
        type="button"
        id={toggleId}
        aria-label={visible ? "Hide password" : "Show password"}
        aria-controls={id}
        className="cursor-pointer bg-transparent px-2 py-[13px] text-input-foreground-primary"
        onClick={() => setVisible((v) => !v)}
      >
        {visible ? (
          <EyeOff className="size-5" aria-hidden />
        ) : (
          <Eye className="size-5" aria-hidden />
        )}
      </button>
    </div>
  );
}

/**
 * Kratos's `code` method issues 6-digit codes (its unconfigured default —
 * revisit if `selfservice.methods.code.config` ever changes the length).
 */
const OTP_LENGTH = 6;

/**
 * Pull the one-time code out of arbitrary pasted text: the first run of
 * EXACTLY six digits. Users paste anything from the bare code to the whole
 * Kratos email ("…code: 513889 or clicking …?code=513889&flow=<uuid>…");
 * exact-run matching keeps UUID fragments ("47545") and durations ("60")
 * from ever qualifying.
 */
// eslint-disable-next-line react-refresh/only-export-components -- pure helper shared with tests, not a component
export function extractOtp(text: string): string | null {
  const runs = text.match(/\d+/g) ?? [];
  return runs.find((run) => run.length === OTP_LENGTH) ?? null;
}

/**
 * Set an input's value the way a user would, so React (and react-hook-form
 * behind it) observe a real change event: through the native value setter —
 * assigning `input.value` directly is swallowed by React's value tracking —
 * followed by a bubbling `input` event.
 */
function setNativeInputValue(input: HTMLInputElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(
    HTMLInputElement.prototype,
    "value",
  )?.set;
  if (setter) {
    setter.call(input, value);
  } else {
    input.value = value;
  }
  input.dispatchEvent(new Event("input", { bubbles: true }));
}

/** Is this element a text-entry surface other than the OTP input itself? */
function isOtherEditable(target: EventTarget | null, otp: HTMLInputElement) {
  return (
    target instanceof HTMLElement &&
    target !== otp &&
    (target instanceof HTMLInputElement ||
      target instanceof HTMLTextAreaElement ||
      target.isContentEditable)
  );
}

/**
 * One-time-code input for the verification/recovery `code` nodes: numeric
 * keypad on mobile, OS OTP autofill (`autocomplete="one-time-code"` also
 * covers a pasted or autofilled full code), and — the point — the flow
 * auto-submits the moment the last digit lands, so the user never has to
 * find the Continue button (w6/m42 onboarding follow-up).
 *
 * Submission clicks the flow's `method` submit button rather than calling
 * `form.requestSubmit()`: a real click carries the submitter's
 * `name=method value=code` pair exactly as a user click would, which the
 * Kratos flow body needs; "Resend code" renders under a different node
 * name, so the selector cannot hit it.
 */
export function OtpCodeInput({ inputProps }: OryNodeInputProps) {
  const lastAutoSubmitted = useRef<string | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);

  // Page-wide paste capture: the code entry is this page's single job, so a
  // paste ANYWHERE (input focused or not) that contains a six-digit code
  // lands in this field — sanitized to just the code, even when the user
  // copied the whole email around it — and flows through the ordinary
  // change path below, so auto-submit fires too. A paste aimed at a
  // different text-entry surface is never hijacked, and clipboard text with
  // no exact six-digit run falls through untouched.
  useEffect(() => {
    const onPaste = (event: ClipboardEvent) => {
      const input = inputRef.current;
      if (!input || input.disabled) return;
      if (isOtherEditable(event.target, input)) return;
      const code = extractOtp(event.clipboardData?.getData("text") ?? "");
      if (!code) return;
      event.preventDefault();
      setNativeInputValue(input, code);
      input.focus();
    };
    document.addEventListener("paste", onPaste);
    return () => document.removeEventListener("paste", onPaste);
  }, []);

  return (
    <input
      data-testid={`ory/form/node/input/${inputProps.name}`}
      {...inputProps}
      ref={(element: HTMLInputElement | null) => {
        inputRef.current = element;
        inputProps.ref?.(element);
      }}
      type="text"
      inputMode="numeric"
      autoComplete="one-time-code"
      pattern="[0-9]*"
      maxLength={OTP_LENGTH}
      // The code entry is this page's single job; focus it on arrival.
      autoFocus
      className={textInputClassName}
      onChange={(event) => {
        inputProps.onChange?.(event);
        const value = event.currentTarget.value.trim();
        if (value.length !== OTP_LENGTH || !/^\d+$/.test(value)) return;
        // Guard against re-fires for the same value (re-renders, a second
        // change event from autofill) — one auto-submit per typed code.
        if (lastAutoSubmitted.current === value) return;
        lastAutoSubmitted.current = value;
        const form = event.currentTarget.form;
        // A microtask lets react-hook-form's own onChange settle before the
        // submit reads the field.
        queueMicrotask(() => {
          const submit =
            form?.querySelector<HTMLButtonElement>(
              'button[data-testid="ory/form/node/button/method"]',
            ) ??
            form?.querySelector<HTMLButtonElement>(
              'button[type="submit"][name="method"]',
            );
          if (submit && !submit.disabled) {
            submit.click();
          } else {
            form?.requestSubmit();
          }
        });
      }}
    />
  );
}

function AuthNodeInput(props: OryNodeInputProps) {
  const { inputProps } = props;

  if (inputProps.type === "password") {
    return <UniqueIdPasswordInput {...props} />;
  }

  if (inputProps.type === "hidden") {
    return (
      <input
        data-testid={`ory/form/node/input/${inputProps.name}`}
        {...inputProps}
      />
    );
  }

  const isEmail = emailNodeNames.has(inputProps.name);
  return (
    <input
      data-testid={`ory/form/node/input/${inputProps.name}`}
      {...inputProps}
      type={isEmail ? "email" : inputProps.type}
      required={isEmail ? true : undefined}
      className={textInputClassName}
    />
  );
}

/**
 * Ory's DefaultFormContainer sets `noValidate`, so empty password login
 * submits silently. Drop that flag so `required` / `type="email"` from
 * AuthNodeInput can surface native validation feedback.
 */
function AuthFormRoot({
  children,
  onSubmit,
  action,
  method,
}: OryFormRootProps) {
  return (
    <form
      onSubmit={onSubmit}
      action={action}
      method={method}
      className="grid gap-8"
    >
      {children}
    </form>
  );
}

/**
 * Auth-flow component overrides: hide the redundant card logo, fix the
 * duplicate password id, and restore empty-submit browser validation.
 *
 * Intentionally avoids importing `@ory/elements-react/theme` (Vitest cannot
 * resolve its extensionless session-provider import; the app Vite config can).
 */
// eslint-disable-next-line react-refresh/only-export-components -- Ory override map, not a component
export const oryAuthFormOverrides: OryFlowComponentOverrides = {
  ...oryHideCardLogo,
  Form: {
    Root: AuthFormRoot,
  },
  Node: {
    Input: AuthNodeInput,
    // Elements routes `code`/`totp_code` nodes through Node.CodeInput, NOT
    // Node.Input (input-renderer's isPinCodeInput branch) — overriding Input
    // alone never sees the OTP field.
    CodeInput: OtpCodeInput,
  },
};
