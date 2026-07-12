export function toSlug(s: string): string {
  return s
    .toLowerCase()
    .replace(/[^a-z0-9-]/g, "-")
    .replace(/-+/g, "-")
    .replace(/^-+|-+$/g, "");
}

export function repoNameSlug(fullName: string): string {
  return toSlug(fullName.split("/").pop() ?? "");
}

export function gitUrlSlug(url: string): string {
  return toSlug(
    url
      .split("/")
      .pop()
      ?.replace(/\.git$/i, "") ?? "",
  );
}

export function imageSlug(image: string): string {
  return toSlug(
    image
      .split("/")
      .pop()
      ?.split(":")[0] ?? "",
  );
}
