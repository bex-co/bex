import { useCallback, useEffect, useState } from "react";
import { useMutation } from "@apollo/client/react";
import {
  RegisterNotificationWebPushSubscriptionDocument,
  UnregisterNotificationWebPushSubscriptionDocument,
} from "@/graphql/definitions";

const browserIdKey = "bex.webpush.browserId";

export type WebPushStatus =
  | "unsupported"
  | "unconfigured"
  | "denied"
  | "prompt"
  | "subscribed"
  | "busy"
  | "error";

function browserSupportsPush(): boolean {
  return (
    typeof window !== "undefined" &&
    "serviceWorker" in navigator &&
    "PushManager" in window &&
    "Notification" in window
  );
}

function readBrowserId(): string {
  let id = window.localStorage.getItem(browserIdKey);
  if (id && /^[A-Za-z0-9][A-Za-z0-9._:-]{0,199}$/.test(id)) {
    return id;
  }
  id = `wp-${crypto.randomUUID()}`;
  window.localStorage.setItem(browserIdKey, id);
  return id;
}

function urlBase64ToUint8Array(value: string): Uint8Array {
  const padding = "=".repeat((4 - (value.length % 4)) % 4);
  const raw = atob((value + padding).replace(/-/g, "+").replace(/_/g, "/"));
  const out = new Uint8Array(raw.length);
  for (let i = 0; i < raw.length; i++) out[i] = raw.charCodeAt(i);
  return out;
}

async function probeWebPushStatus(
  vapidPublicKey: string,
  serverAvailable: boolean,
): Promise<WebPushStatus> {
  if (!browserSupportsPush()) return "unsupported";
  if (!serverAvailable || !vapidPublicKey) return "unconfigured";
  if (Notification.permission === "denied") return "denied";
  const registration = await navigator.serviceWorker.getRegistration("/push-sw.js");
  const sub = await registration?.pushManager.getSubscription();
  return sub ? "subscribed" : "prompt";
}

export function useWebPushSubscription(vapidPublicKey: string, serverAvailable: boolean) {
  const [status, setStatus] = useState<WebPushStatus>("busy");
  const [register] = useMutation(RegisterNotificationWebPushSubscriptionDocument);
  const [unregister] = useMutation(UnregisterNotificationWebPushSubscriptionDocument);

  useEffect(() => {
    let cancelled = false;
    void probeWebPushStatus(vapidPublicKey, serverAvailable).then((next) => {
      if (!cancelled) setStatus(next);
    });
    return () => {
      cancelled = true;
    };
  }, [serverAvailable, vapidPublicKey]);

  const subscribe = useCallback(async () => {
    if (!browserSupportsPush() || !vapidPublicKey) return false;
    setStatus("busy");
    try {
      const permission = await Notification.requestPermission();
      if (permission !== "granted") {
        setStatus(permission === "denied" ? "denied" : "prompt");
        return false;
      }
      const registration = await navigator.serviceWorker.register("/push-sw.js");
      const key = urlBase64ToUint8Array(vapidPublicKey);
      const applicationServerKey = new ArrayBuffer(key.byteLength);
      new Uint8Array(applicationServerKey).set(key);
      const sub = await registration.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey,
      });
      const json = sub.toJSON();
      if (!json.endpoint || !json.keys?.p256dh || !json.keys.auth) {
        setStatus("error");
        return false;
      }
      await register({
        variables: {
          browserId: readBrowserId(),
          endpoint: json.endpoint,
          p256dh: json.keys.p256dh,
          auth: json.keys.auth,
        },
      });
      setStatus("subscribed");
      return true;
    } catch {
      setStatus("error");
      return false;
    }
  }, [register, vapidPublicKey]);

  const unsubscribe = useCallback(async () => {
    setStatus("busy");
    try {
      const registration = await navigator.serviceWorker.getRegistration("/push-sw.js");
      const sub = await registration?.pushManager.getSubscription();
      await sub?.unsubscribe();
      await unregister({ variables: { browserId: readBrowserId() } });
      setStatus("prompt");
      return true;
    } catch {
      setStatus("error");
      return false;
    }
  }, [unregister]);

  return { status, subscribe, unsubscribe };
}
