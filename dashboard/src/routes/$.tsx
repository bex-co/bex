import { createFileRoute } from "@tanstack/react-router";
import NotFoundPage from "@/common/root-route/not-found-page";

export const Route = createFileRoute("/$")({
  component: NotFoundPage,
  head: () => ({
    meta: [{ title: "Page not found · bex dashboard" }],
  }),
});
