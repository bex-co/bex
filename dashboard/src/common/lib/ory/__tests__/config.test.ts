import { describe, it, expect } from "vitest";
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
    expect(oryHideCardLogo.Card?.Logo?.({})).toBeNull();
  });
});

describe("oryHideSettingsPageHeader", () => {
  it("suppresses the Settings page's own header", () => {
    expect(oryHideSettingsPageHeader.Page?.Header?.({})).toBeNull();
  });
});
