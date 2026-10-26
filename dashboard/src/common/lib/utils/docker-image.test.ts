import { describe, expect, it } from "vitest";
import { dockerImageProblem, isValidDockerImageRef } from "./docker-image";

// w4/406: the dashboard mirrors bex-api's ValidateImage row-local rules.
describe("dockerImageProblem", () => {
  it.each([
    "not a valid ref!!",
    "Nginx:latest",
    "ghcr.io/Traefik/whoami:v1.10",
    "nginx@sha256:abc",
    "nginx:",
    "/nginx",
    "nginx//x",
  ])("marks %s malformed", (ref) => {
    expect(dockerImageProblem(ref)).toBe("malformed");
    expect(isValidDockerImageRef(ref)).toBe(false);
  });

  it.each([
    "localhost:5000/x:1",
    "localhost/x:1",
    "169.254.169.254/x:1",
    "10.0.0.5/x:1",
    "192.168.1.20:5000/x:1",
    "127.0.0.1/x:1",
    "[::1]:5000/x:1",
    "[fe80::1]/x:1",
  ])("marks %s as a private host", (ref) => {
    expect(dockerImageProblem(ref)).toBe("privateHost");
  });

  it.each([
    "ghcr.io/traefik/whoami:v1.10",
    "nginx",
    "nginx:latest",
    "docker.io/library/nginx:1.27",
    "myregistry.example.com:5000/team/app:v2",
    `nginx@sha256:${"a".repeat(64)}`,
    "8.8.8.8/x:1",
  ])("accepts %s", (ref) => {
    expect(dockerImageProblem(ref)).toBeNull();
    expect(isValidDockerImageRef(ref)).toBe(true);
  });

  it("marks an over-long reference", () => {
    expect(dockerImageProblem(`ghcr.io/${"a".repeat(520)}/x:1`)).toBe(
      "tooLong",
    );
  });

  it("leaves emptiness to the required check", () => {
    expect(dockerImageProblem("")).toBeNull();
    expect(dockerImageProblem("   ")).toBeNull();
    expect(isValidDockerImageRef("")).toBe(false);
  });
});
