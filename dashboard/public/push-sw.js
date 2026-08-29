/* Browser web push: decode the closed JSON payload and show an OS notification.
   Click navigation is restricted to same-origin relative routes. */
self.addEventListener("push", (event) => {
  let payload = { title: "bex", body: "", data: {} };
  try {
    payload = event.data ? event.data.json() : payload;
  } catch (_) {
    payload.body = event.data ? event.data.text() : "";
  }
  const title =
    typeof payload.title === "string" && payload.title ? payload.title : "bex";
  const body = typeof payload.body === "string" ? payload.body : "";
  const data =
    payload.data && typeof payload.data === "object" ? payload.data : {};
  event.waitUntil(
    self.registration.showNotification(title, {
      body,
      data,
      tag:
        typeof data.notificationId === "string"
          ? data.notificationId
          : undefined,
    }),
  );
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const route =
    event.notification.data && typeof event.notification.data.route === "string"
      ? event.notification.data.route
      : "";
  if (
    !route.startsWith("/") ||
    route.startsWith("//") ||
    route.startsWith("/\\")
  ) {
    return;
  }
  const resolved = new URL(route, self.location.origin);
  if (resolved.origin !== self.location.origin) {
    return;
  }
  // Keep the navigation relative after validation. This makes the same-origin
  // property explicit at the sink instead of carrying a resolved authority.
  const url = `${resolved.pathname}${resolved.search}${resolved.hash}`;
  event.waitUntil(
    self.clients
      .matchAll({ type: "window", includeUncontrolled: true })
      .then((clients) => {
        for (const client of clients) {
          if ("focus" in client) {
            client.navigate(url);
            return client.focus();
          }
        }
        if (self.clients.openWindow) {
          return self.clients.openWindow(url);
        }
        return undefined;
      }),
  );
});
