import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemberRow } from "@/features/team/components/member-row";
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
