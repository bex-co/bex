import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemberRow } from "@/features/team/components/member-row";
import { memberActionReasonKeys } from "@/features/team/lib/member-action-reasons";
import type { MemberView } from "@/features/team/types";

function member(overrides: Partial<MemberView> = {}): MemberView {
  return {
    subject: "subj-1",
    userId: "own-abc",
    email: "a@example.com",
    role: "DEVELOPER",
    createdAt: null,
    mfaEnabled: false,
    identityResolved: true,
    isOwner: false,
    isSelf: false,
    ...overrides,
  };
}

describe("MemberRow — unresolved identity (w4/070)", () => {
  it("keeps the never-blank fallback and badges an unresolved lookup miss", () => {
    render(
      <table>
        <tbody>
          <MemberRow
            member={member({
              email: "",
              identityResolved: false,
            })}
            canManage={false}
            changing={false}
            removing={false}
            onChangeRole={vi.fn()}
            onRemove={vi.fn()}
          />
        </tbody>
      </table>,
    );

    expect(screen.getByText("own-abc")).toBeInTheDocument();
    expect(screen.getByText("Unresolved")).toBeInTheDocument();
  });

  it("does not badge a resolved member", () => {
    render(
      <table>
        <tbody>
          <MemberRow
            member={member()}
            canManage={false}
            changing={false}
            removing={false}
            onChangeRole={vi.fn()}
            onRemove={vi.fn()}
          />
        </tbody>
      </table>,
    );

    expect(screen.getByText("a@example.com")).toBeInTheDocument();
    expect(screen.queryByText("Unresolved")).toBeNull();
  });
});

describe("MemberRow — membership invariants (w5/m101)", () => {
  function renderRow(overrides: Partial<MemberView>) {
    const onRemove = vi.fn();
    const onChangeRole = vi.fn();
    render(
      <table>
        <tbody>
          <MemberRow
            member={member(overrides)}
            canManage
            changing={false}
            removing={false}
            onChangeRole={onChangeRole}
            onRemove={onRemove}
          />
        </tbody>
      </table>,
    );
    return { onRemove, onChangeRole };
  }

  // A disabled control swallows pointer events, so PermissionTooltip wraps it
  // in a focusable trigger span that carries the reason. Radix renders the
  // content into a portal jsdom never mounts, so the row test asserts the
  // trigger is there (the permission-tooltip test's own convention) and the
  // reason mapping is asserted directly against memberActionReasonKeys below.
  function expectExplainedTriggers(count: number) {
    expect(
      document.querySelectorAll('[data-slot="tooltip-trigger"]'),
    ).toHaveLength(count);
  }

  it("disables Remove and the role picker on the caller's own row, and says why", () => {
    renderRow({ isSelf: true });

    expect(screen.getByText("You")).toBeInTheDocument();
    // The control is present but refused — not hidden (the m50 pattern).
    expect(screen.getByRole("button", { name: "Remove" })).toBeDisabled();
    expect(screen.getByRole("combobox")).toBeDisabled();
    expectExplainedTriggers(2); // the role picker and the Remove action
  });

  it("disables Remove and demotion on the workspace owner's row, and says why", () => {
    renderRow({ isOwner: true });

    expect(screen.getByText("Owner")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Remove" })).toBeDisabled();
    expect(screen.getByRole("combobox")).toBeDisabled();
    expectExplainedTriggers(2);
  });

  it("leaves an ordinary teammate fully actionable", () => {
    renderRow({});

    expect(screen.queryByText("You")).toBeNull();
    expect(screen.queryByText("Owner")).toBeNull();
    expect(screen.getByRole("button", { name: "Remove" })).toBeEnabled();
    expect(screen.getByRole("combobox")).toBeEnabled();
    expectExplainedTriggers(0); // nothing to explain — nothing is refused
  });

  it("names the refusal each row would hit, self before owner", () => {
    expect(memberActionReasonKeys(member())).toEqual({
      removeReason: null,
      roleReason: null,
    });
    expect(memberActionReasonKeys(member({ isOwner: true }))).toEqual({
      removeReason: "team.removeOwnerReason",
      roleReason: "team.changeOwnerRoleReason",
    });
    // The owner looking at their own row is told about themselves — that
    // refusal (CANNOT_REMOVE_SELF) is the one the server answers first.
    expect(
      memberActionReasonKeys(member({ isOwner: true, isSelf: true })),
    ).toEqual({
      removeReason: "team.removeSelfReason",
      roleReason: "team.changeOwnRoleReason",
    });
  });
});
