import { afterEach, describe, it, expect } from "vitest";
import { act, renderHook } from "@testing-library/react";
import i18n from "@/i18n/init";
import type { FunctionComponent } from "react";
import {
  oryConfig,
  oryHideCardLogo,
  oryHideSettingsPageHeader,
  useOryConfig,
} from "../config";

describe("oryConfig", () => {
  it("hides Ory's own branding badge", () => {
    expect(oryConfig.project.hide_ory_branding).toBe(true);
  });
});

describe("oryHideCardLogo", () => {
  it("suppresses the Ory card's text-logo header", () => {
    const Logo = oryHideCardLogo.Card?.Logo as
      FunctionComponent<Record<string, never>> | undefined;
    expect(Logo?.({})).toBeNull();
  });
});

describe("oryHideSettingsPageHeader", () => {
  it("suppresses the Settings page's own header", () => {
    const Header = oryHideSettingsPageHeader.Page?.Header as
      FunctionComponent<Record<never, never>> | undefined;
    expect(Header?.({})).toBeNull();
  });
});

// w4/587: the identity schema's "E-Mail" and "Name" titles reach the forms
// worded in the dashboard's language, through Ory's own message ids.
describe("useOryConfig", () => {
  afterEach(async () => {
    await i18n.changeLanguage("en");
  });

  it("words the profile traits in the current language", async () => {
    await act(() => i18n.changeLanguage("zh"));
    const { result } = renderHook(() => useOryConfig());
    expect(result.current.intl?.locale).toBe("zh");
    expect(result.current.intl?.customTranslations?.zh).toMatchObject({
      "forms.label.traits.email": "邮箱",
      "forms.input.placeholder.traits.email": "输入你的邮箱",
      "forms.label.traits.name": "名称",
      "forms.input.placeholder.traits.name": "输入你的名称",
      "input.placeholder": "输入你的{placeholder}",
    });
  });
});
