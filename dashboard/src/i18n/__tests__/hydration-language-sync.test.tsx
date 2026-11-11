import { Suspense, act, lazy } from "react";
import { hydrateRoot } from "react-dom/client";
import { renderToString } from "react-dom/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "@/i18n/init";
import { useLanguageHydrationSync } from "@/i18n/use-language-hydration-sync";
import { useTranslations } from "@/common/hooks/use-translations";
import { memoryStorage } from "@/test/mocks/storage";

// w21/054: the stored-language sync switches the language after hydration. The
// production report (#418 on a first load whose language cookie is missing while
// localStorage holds `zh`) is inferred to come from a Suspense boundary that is
// still dehydrated when the switch lands. This probe builds exactly that shape,
// an English server tree with a lazy boundary whose chunk resolves only after
// the switch, and PINS that this shape hydrates cleanly today: no recoverable
// error and a final zh render. So the mechanism the report infers is not the
// whole story; the boundary that throws in production is still to be named
// (the note stays blocked on a non-minified reproduction).

function Heading() {
  const { t } = useTranslations();
  return <h1>{t("common.navSettings")}</h1>;
}

function makeLazyHeading() {
  let release: () => void = () => {};
  const gate = new Promise<void>((resolve) => {
    release = resolve;
  });
  const Lazy = lazy(async () => {
    await gate;
    return { default: Heading };
  });
  return { Lazy, release };
}

function App({ Lazy }: { Lazy: React.ComponentType }) {
  useLanguageHydrationSync();
  const { t } = useTranslations();
  return (
    <main>
      <p>{t("common.navSettings")}</p>
      <Suspense fallback={<p>loading</p>}>
        <Lazy />
      </Suspense>
    </main>
  );
}

const zhSettings = i18n.t("common.navSettings", { lng: "zh" });

beforeEach(async () => {
  // setup.ts installs a no-op localStorage; persistence is the point here.
  const storage = memoryStorage();
  vi.stubGlobal("localStorage", storage);
  await i18n.changeLanguage("en");
  storage.setItem("i18nextLng", "zh");
});

afterEach(async () => {
  vi.unstubAllGlobals();
  await i18n.changeLanguage("en");
});

describe("the stored-language sync over a still-dehydrated boundary (w21/054)", () => {
  it("hydrates a late boundary cleanly and ends in the stored language", async () => {
    // Server: English, with the boundary's content already in the HTML (the
    // server renders it eagerly; only the client's chunk is late).
    const html = renderToString(<App Lazy={Heading} />);
    expect(html).toContain("<h1>");

    // Client: hydrate that HTML, with the boundary's chunk still pending.
    const container = document.createElement("div");
    container.innerHTML = html;
    document.body.appendChild(container);
    const client = makeLazyHeading();
    const recovered: unknown[] = [];
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    let root: ReturnType<typeof hydrateRoot> | undefined;
    await act(async () => {
      root = hydrateRoot(container, <App Lazy={client.Lazy} />, {
        onRecoverableError: (error) => recovered.push(error),
      });
    });

    // The post-hydration effect switches the language to the stored one while
    // the boundary is still dehydrated; only then does its chunk arrive.
    await vi.waitFor(() => expect(i18n.language).toBe("zh"));
    await act(async () => {
      client.release();
      await Promise.resolve();
    });
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 50));
    });

    expect(recovered).toEqual([]);
    expect(container.querySelector("h1")?.textContent).toBe(zhSettings);
    expect(container.querySelector("p")?.textContent).toBe(zhSettings);

    act(() => root?.unmount());
    container.remove();
    errorSpy.mockRestore();
  });
});
