import { createHash } from "node:crypto";
import { afterEach, describe, expect, it, vi } from "vitest";
import { cropArea, cropDiameter } from "@/features/avatars/lib/avatar-image";
import { sha256Hex } from "@/common/lib/utils/sha256";
import {
  GRAVATAR_MISS_TTL_MS,
  MONOGRAM_COLORS,
  emailMonogram,
  forgetGravatars,
  gravatarHash,
  gravatarUrl,
  hasGravatar,
  isSavableMonogram,
  monogramColorIndex,
  workspaceMonogram,
} from "@/features/avatars/lib/avatar-utils";

// Render's avatar rules (w1/m648, docs/render-artifacts/avatars.md). The
// expected colors were worked out from Render's hash by a second, independent
// implementation (Python's 32-bit arithmetic), not by this one.

const colorOf = (index: number) => MONOGRAM_COLORS[index]?.id;

describe("default monograms", () => {
  it("is a person's email's first letter, colored by the email", () => {
    expect(emailMonogram("ada@example.com")).toEqual({
      letter: "A",
      colorIndex: 3,
    });
    expect(colorOf(emailMonogram("ada@example.com").colorIndex)).toBe("orange");
    expect(colorOf(emailMonogram("bob@example.com").colorIndex)).toBe(
      "lime-green",
    );
    expect(colorOf(emailMonogram("zoe@example.org").colorIndex)).toBe("blue");
  });

  it("reads the email trimmed and lower-cased, as Render keys it", () => {
    expect(emailMonogram("  ADA@Example.COM ")).toEqual(
      emailMonogram("ada@example.com"),
    );
  });

  it("is a workspace's name's first letter, colored by its id", () => {
    expect(workspaceMonogram("tea-acme", "acme")).toEqual({
      letter: "A",
      colorIndex: 1,
    });
    // The id, not the name, picks the color: a rename keeps it.
    expect(workspaceMonogram("tea-acme", "zeta").colorIndex).toBe(1);
  });

  it("hashes an empty seed to the first color", () => {
    expect(monogramColorIndex("")).toBe(0);
    expect(emailMonogram(null)).toEqual({ letter: "", colorIndex: 0 });
  });

  it("saves exactly one character on a palette color", () => {
    expect(isSavableMonogram({ letter: "B", colorIndex: 7 })).toBe(true);
    expect(isSavableMonogram({ letter: "", colorIndex: 0 })).toBe(false);
    expect(isSavableMonogram({ letter: "AB", colorIndex: 0 })).toBe(false);
    expect(isSavableMonogram({ letter: "A", colorIndex: 8 })).toBe(false);
    // One code point outside the BMP is one character.
    expect(isSavableMonogram({ letter: "😀", colorIndex: 0 })).toBe(true);
  });
});

describe("Gravatar", () => {
  afterEach(() => forgetGravatars());

  it("asks Render's URL: the email's SHA-256, d=404, doubled on retina", async () => {
    const hash = await sha256Hex("ada@example.com");
    expect(hash).toBe(
      createHash("sha256").update("ada@example.com").digest("hex"),
    );
    expect(gravatarUrl(hash, 24)).toBe(
      `https://en.gravatar.com/avatar/${hash}?s=24&r=x&d=404`,
    );
    expect(gravatarUrl(hash, 24, true)).toContain("?s=48&");
  });

  it("keys an email trimmed and lower-cased", async () => {
    expect(await gravatarHash(" Ada@Example.com ")).toBe(
      await sha256Hex("ada@example.com"),
    );
  });

  it("asks once per email, remembers a miss for five minutes and a hit for good", async () => {
    const probe = vi.fn((url: string) => Promise.resolve(url.includes("hit")));
    const now = Date.now();
    // Two avatars of one email at once share one probe.
    const [a, b] = await Promise.all([
      hasGravatar("miss", "/miss?s=24", now, probe),
      hasGravatar("miss", "/miss?s=64", now, probe),
    ]);
    expect([a, b]).toEqual([false, false]);
    expect(probe).toHaveBeenCalledTimes(1);
    // Within five minutes the miss is answered from memory, at any size.
    expect(
      await hasGravatar(
        "miss",
        "/miss?s=32",
        now + GRAVATAR_MISS_TTL_MS - 1,
        probe,
      ),
    ).toBe(false);
    expect(probe).toHaveBeenCalledTimes(1);
    // After them it is asked again.
    await hasGravatar(
      "miss",
      "/miss?s=32",
      Date.now() + GRAVATAR_MISS_TTL_MS,
      probe,
    );
    expect(probe).toHaveBeenCalledTimes(2);

    expect(await hasGravatar("hit", "/hit", now, probe)).toBe(true);
    expect(
      await hasGravatar("hit", "/hit", now + 100 * GRAVATAR_MISS_TTL_MS, probe),
    ).toBe(true);
    expect(probe).toHaveBeenCalledTimes(3);
  });
});

describe("the crop", () => {
  it("covers a square image whole at zoom 1", () => {
    const view = {
      imageWidth: 400,
      imageHeight: 400,
      viewport: 400,
      zoom: 1,
      offsetX: 0,
      offsetY: 0,
    };
    expect(cropDiameter(view)).toBe(400);
    expect(cropArea(view)).toEqual({ x: 0, y: 0, size: 400 });
  });

  it("is the shorter side of a fitted image, centred, zoomed and dragged", () => {
    const base = {
      imageWidth: 800,
      imageHeight: 400,
      viewport: 400,
      zoom: 1,
      offsetX: 0,
      offsetY: 0,
    };
    expect(cropDiameter(base)).toBe(200);
    expect(cropArea(base)).toEqual({ x: 200, y: 0, size: 400 });
    expect(cropArea({ ...base, zoom: 2 })).toEqual({
      x: 300,
      y: 100,
      size: 200,
    });
    // Dragging the image 100 px right puts the circle 200 image px left.
    expect(cropArea({ ...base, offsetX: 100 })).toEqual({
      x: 0,
      y: 0,
      size: 400,
    });
  });
});
