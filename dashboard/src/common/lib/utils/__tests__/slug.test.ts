import { describe, it, expect } from "vitest";
import { toSlug, repoNameSlug, gitUrlSlug, imageSlug } from "../slug";

describe("toSlug", () => {
  it("lowercases and replaces non-alphanumeric chars with hyphens", () => {
    expect(toSlug("My Cool App")).toBe("my-cool-app");
    expect(toSlug("hello_world")).toBe("hello-world");
    expect(toSlug("foo.bar.baz")).toBe("foo-bar-baz");
  });

  it("collapses consecutive hyphens", () => {
    expect(toSlug("a--b---c")).toBe("a-b-c");
    expect(toSlug("hello   world")).toBe("hello-world");
  });

  it("strips leading and trailing hyphens", () => {
    expect(toSlug("-foo-")).toBe("foo");
    expect(toSlug("_bar_")).toBe("bar");
  });

  it("preserves valid DNS chars", () => {
    expect(toSlug("my-service-1")).toBe("my-service-1");
  });
});

describe("repoNameSlug", () => {
  it("extracts the repo name from owner/repo and slugifies", () => {
    expect(repoNameSlug("acme-corp/web-frontend")).toBe("web-frontend");
    expect(repoNameSlug("acme-corp/My_API")).toBe("my-api");
  });
});

describe("gitUrlSlug", () => {
  it("strips .git suffix and slugifies the last path segment", () => {
    expect(gitUrlSlug("https://github.com/acme/my-backend-api.git")).toBe(
      "my-backend-api",
    );
    expect(gitUrlSlug("https://github.com/acme/my-backend-api")).toBe(
      "my-backend-api",
    );
    expect(gitUrlSlug("git@github.com:acme/MyService.git")).toBe("myservice");
  });
});

describe("imageSlug", () => {
  it("extracts the image name without registry/org and without tag", () => {
    expect(imageSlug("docker.io/myorg/api-gateway:v2")).toBe("api-gateway");
    expect(imageSlug("nginx:latest")).toBe("nginx");
    expect(imageSlug("ghcr.io/acme/my-app")).toBe("my-app");
  });
});
