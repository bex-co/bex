import { useState } from "react";
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
  const { id, type: _type, ...rest } = inputProps;
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
export const oryAuthFormOverrides: OryFlowComponentOverrides = {
  ...oryHideCardLogo,
  Form: {
    Root: AuthFormRoot,
  },
  Node: {
    Input: AuthNodeInput,
  },
};
