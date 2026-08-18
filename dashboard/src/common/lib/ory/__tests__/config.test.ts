import { describe, it, expect } from "vitest";
import type { FunctionComponent } from "react";
import {
  oryConfig,
  oryHideCardLogo,
  oryHideSettingsPageHeader,
} from "../config";

describe("oryConfig", () => {
  it("hides Ory's own branding badge", () => {
    expect(oryConfig.project.hide_ory_branding).toBe(true);
  });
});

describe("oryHideCardLogo", () => {
  it("suppresses the Ory card's text-logo header", () => {
    const Logo = oryHideCardLogo.Card?.Logo as
      | FunctionComponent<Record<string, never>>
      | undefined;
    expect(Logo?.({})).toBeNull();
  });
});

describe("oryHideSettingsPageHeader", () => {
  it("suppresses the Settings page's own header", () => {
    const Header = oryHideSettingsPageHeader.Page?.Header as
      | FunctionComponent<Record<never, never>>
      | undefined;
    expect(Header?.({})).toBeNull();
  });
});
