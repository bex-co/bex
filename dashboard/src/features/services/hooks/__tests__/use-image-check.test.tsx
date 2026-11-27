import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { Observable } from "rxjs";
import { apolloCacheConfig } from "@/common/apollo/cache";
import { apolloDefaultOptions } from "@/common/apollo/default-options";
import { UndeployableImageFailureCode } from "@/graphql/schema-types";
import {
  IMAGE_CHECK_DEBOUNCE_MS,
  useImageCheck,
} from "@/features/services/hooks/use-image-check";

/**
 * w1/m673: Render's isDeployableImage as the image field is edited. Real
 * Apollo with the app's defaults; the network answers when the test says so,
 * and the clock is faked.
 */
type Request = {
  imageUrl: string;
  registryCredentialId: string | null;
  answer: (result: {
    deployable: boolean;
    failureCode?: UndeployableImageFailureCode | null;
  }) => void;
  fail: () => void;
};

function fakeClient() {
  const requests: Request[] = [];
  const link = new ApolloLink(
    (operation) =>
      new Observable((sub) => {
        const { externalImageReference } = operation.variables as {
          externalImageReference: {
            imageUrl: string;
            registryCredentialId: string | null;
          };
        };
        requests.push({
          ...externalImageReference,
          answer: ({ deployable, failureCode = null }) => {
            sub.next({
              data: {
                isDeployableImage: {
                  __typename: "IsDeployableImageResult",
                  deployable,
                  failureCode,
                  failureReason: deployable ? null : "refused",
                },
              },
            });
            sub.complete();
          },
          fail: () => sub.error(new Error("network down")),
        });
      }),
  );
  const client = new ApolloClient({
    link,
    cache: new InMemoryCache(apolloCacheConfig),
    defaultOptions: apolloDefaultOptions,
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <ApolloProvider client={client}>{children}</ApolloProvider>
  );
  return { wrapper, requests };
}

type Props = { image: string; credentialId: string; enabled: boolean };

function setup(initial: Props) {
  const { wrapper, requests } = fakeClient();
  const hook = renderHook((props: Props) => useImageCheck(props), {
    wrapper,
    initialProps: initial,
  });
  return { ...hook, requests };
}

async function flush() {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(0);
  });
}

beforeEach(() => {
  vi.useFakeTimers();
});
afterEach(() => {
  vi.useRealTimers();
});

describe("useImageCheck", () => {
  it("checks a typed image 800 ms after typing pauses, once", async () => {
    const { result, rerender, requests } = setup({
      image: "",
      credentialId: "",
      enabled: true,
    });
    expect(result.current.status).toBe("idle");

    for (const typed of ["n", "ng", "ngin", "nginx"]) {
      rerender({ image: typed, credentialId: "", enabled: true });
      await act(async () => {
        await vi.advanceTimersByTimeAsync(300);
      });
    }
    expect(requests).toHaveLength(0);
    // Waiting for the pause is already a check under way.
    expect(result.current.status).toBe("checking");

    await act(async () => {
      await vi.advanceTimersByTimeAsync(IMAGE_CHECK_DEBOUNCE_MS - 300 - 1);
    });
    expect(requests).toHaveLength(0);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
    });
    expect(requests.map((r) => r.imageUrl)).toEqual(["nginx"]);
    expect(requests[0].registryCredentialId).toBeNull();
    expect(result.current.status).toBe("checking");

    act(() => requests[0].answer({ deployable: true }));
    await flush();
    expect(result.current.status).toBe("deployable");
    expect(result.current.failureCode).toBeNull();
  });

  it("checks a pasted image at once", async () => {
    const { result, rerender, requests } = setup({
      image: "",
      credentialId: "",
      enabled: true,
    });
    act(() => result.current.notePaste());
    rerender({
      image: "ghcr.io/acme/app:1",
      credentialId: "",
      enabled: true,
    });
    await flush();
    expect(requests.map((r) => r.imageUrl)).toEqual(["ghcr.io/acme/app:1"]);
  });

  it("checks the value the field opens with at once, as Render's Update Image does", async () => {
    const { requests } = setup({
      image: "docker.io/library/nginx:latest",
      credentialId: "rgc-a",
      enabled: true,
    });
    await flush();
    expect(requests).toHaveLength(1);
    expect(requests[0]).toMatchObject({
      imageUrl: "docker.io/library/nginx:latest",
      registryCredentialId: "rgc-a",
    });
  });

  it("checks the same image again at once when the credential changes", async () => {
    const { result, rerender, requests } = setup({
      image: "ghcr.io/acme/private:1",
      credentialId: "",
      enabled: true,
    });
    await flush();
    act(() =>
      requests[0].answer({
        deployable: false,
        failureCode: UndeployableImageFailureCode.DoesNotExist,
      }),
    );
    await flush();
    expect(result.current).toMatchObject({
      status: "failed",
      failureCode: UndeployableImageFailureCode.DoesNotExist,
      withCredential: false,
    });

    rerender({
      image: "ghcr.io/acme/private:1",
      credentialId: "rgc-ghcr",
      enabled: true,
    });
    await flush();
    expect(requests).toHaveLength(2);
    expect(requests[1].registryCredentialId).toBe("rgc-ghcr");
    // The old answer was for no credential: the field is checking again.
    expect(result.current.status).toBe("checking");
    act(() => requests[1].answer({ deployable: true }));
    await flush();
    expect(result.current).toMatchObject({
      status: "deployable",
      withCredential: true,
    });
  });

  it("drops an answer for a value that has since changed", async () => {
    const { result, rerender, requests } = setup({
      image: "nginx:1",
      credentialId: "",
      enabled: true,
    });
    await flush();
    rerender({ image: "nginx:2", credentialId: "", enabled: true });
    // The first image's answer arrives while the second waits for its pause.
    act(() => requests[0].answer({ deployable: true }));
    await flush();
    expect(result.current.status).toBe("checking");

    await act(async () => {
      await vi.advanceTimersByTimeAsync(IMAGE_CHECK_DEBOUNCE_MS);
    });
    expect(requests.map((r) => r.imageUrl)).toEqual(["nginx:1", "nginx:2"]);
    act(() =>
      requests[1].answer({
        deployable: false,
        failureCode: UndeployableImageFailureCode.UnsupportedPlatform,
      }),
    );
    await flush();
    expect(result.current).toMatchObject({
      status: "failed",
      failureCode: UndeployableImageFailureCode.UnsupportedPlatform,
    });
  });

  it("keeps the newer answer when an older check answers last", async () => {
    const { result, rerender, requests } = setup({
      image: "nginx:1",
      credentialId: "",
      enabled: true,
    });
    await flush();
    rerender({ image: "nginx:2", credentialId: "", enabled: true });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(IMAGE_CHECK_DEBOUNCE_MS);
    });
    expect(requests.map((r) => r.imageUrl)).toEqual(["nginx:1", "nginx:2"]);
    act(() => requests[1].answer({ deployable: true }));
    await flush();
    expect(result.current.status).toBe("deployable");
    // The first check's answer arrives late: it is for a value the field no
    // longer holds, and it must not replace the answer the field shows.
    act(() =>
      requests[0].answer({
        deployable: false,
        failureCode: UndeployableImageFailureCode.DoesNotExist,
      }),
    );
    await flush();
    expect(result.current.status).toBe("deployable");
  });

  it("checks with the credential chosen while typing paused", async () => {
    const { rerender, requests } = setup({
      image: "",
      credentialId: "",
      enabled: true,
    });
    rerender({ image: "ghcr.io/acme/app:1", credentialId: "", enabled: true });
    rerender({
      image: "ghcr.io/acme/app:1",
      credentialId: "rgc-ghcr",
      enabled: true,
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(IMAGE_CHECK_DEBOUNCE_MS);
    });
    expect(requests).toHaveLength(1);
    expect(requests[0].registryCredentialId).toBe("rgc-ghcr");
  });

  it("resets to idle on an empty field and sends nothing for a malformed one", async () => {
    const { result, rerender, requests } = setup({
      image: "nginx",
      credentialId: "",
      enabled: true,
    });
    await flush();
    rerender({ image: "  ", credentialId: "", enabled: true });
    expect(result.current.status).toBe("idle");
    rerender({ image: "Not An Image", credentialId: "", enabled: true });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(IMAGE_CHECK_DEBOUNCE_MS * 2);
    });
    expect(result.current.status).toBe("idle");
    expect(requests).toHaveLength(1);
  });

  it("answers a failed request as a failure with no code", async () => {
    const { result, requests } = setup({
      image: "nginx",
      credentialId: "",
      enabled: true,
    });
    await flush();
    act(() => requests[0].fail());
    await flush();
    expect(result.current).toMatchObject({
      status: "failed",
      failureCode: null,
    });
  });

  it("does nothing while disabled, and checks at once when enabled", async () => {
    const { result, rerender, requests } = setup({
      image: "nginx",
      credentialId: "",
      enabled: false,
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(IMAGE_CHECK_DEBOUNCE_MS * 2);
    });
    expect(requests).toHaveLength(0);
    expect(result.current.status).toBe("idle");
    rerender({ image: "nginx", credentialId: "", enabled: true });
    await flush();
    expect(requests).toHaveLength(1);
  });
});
