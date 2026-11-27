import { createFileRoute } from "@tanstack/react-router";

/** A project's own page; the `_overview` layout renders it. */
export const Route = createFileRoute("/project/$projectId/_overview/")({});
