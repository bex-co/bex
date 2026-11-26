import { useEffect, useState, type ReactNode } from "react";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils";
import {
  AVATAR_PIXELS,
  AVATAR_SIZE_CLASS,
  emailMonogram,
  forgetGravatar,
  gravatarHash,
  gravatarUrl,
  hasGravatar,
  isRetinaScreen,
  monogramColor,
  normalizeEmail,
  resolveAvatarUrl,
  workspaceMonogram,
  type AvatarSize,
  type Monogram,
} from "@/features/avatars/lib/avatar-utils";

/**
 * Avatars as Render draws them (w1/m648, docs/render-artifacts/avatars.md):
 * an uploaded image when there is one; for a person, else their Gravatar;
 * else a monogram, one letter on a palette color. Round at every size.
 */

const ROUND = "shrink-0 select-none rounded-full";

/** One letter on one of Render's eight colors. Decorative: the name or email
 *  beside it (or its button's label) says whose it is. */
export function MonogramAvatar({
  monogram,
  size = "medium",
  className,
}: {
  monogram: Monogram;
  size?: AvatarSize;
  className?: string;
}) {
  const color = monogramColor(monogram.colorIndex);
  return (
    <span
      aria-hidden="true"
      data-slot="monogram"
      className={cn(
        "flex shrink-0 select-none items-center justify-center rounded-full leading-none font-medium",
        AVATAR_SIZE_CLASS[size],
        className,
      )}
      style={{ backgroundColor: color.background, color: color.foreground }}
    >
      {monogram.letter}
    </span>
  );
}

/** An owner's uploaded image, or `fallback` while there is none or when it
 *  cannot load. */
function UploadedOr({
  imageURL,
  alt,
  size,
  className,
  fallback,
}: {
  imageURL: string | null | undefined;
  alt: string;
  size: AvatarSize;
  className?: string;
  fallback: ReactNode;
}) {
  const src = resolveAvatarUrl(imageURL);
  const [failed, setFailed] = useState<string | null>(null);
  if (!src || failed === src) return fallback;
  return (
    <img
      src={src}
      alt={alt}
      data-slot="avatar-upload"
      className={cn(
        ROUND,
        "border object-cover",
        AVATAR_SIZE_CLASS[size],
        className,
      )}
      onError={() => setFailed(src)}
    />
  );
}

/**
 * The Gravatar of an email when it has one, else the monogram. The hash and
 * the probe run in the browser only, so a server render is the monogram; the
 * probe asks for the size this screen shows, so the image it proves is the
 * one the browser then has cached.
 */
function GravatarOrMonogram({
  email,
  size,
  className,
}: {
  email: string;
  size: AvatarSize;
  className?: string;
}) {
  const { t } = useTranslations();
  const key = normalizeEmail(email);
  // What loaded, and for which email and size: a change of either shows the
  // monogram again until its own Gravatar answers.
  const [loaded, setLoaded] = useState<{
    for: string;
    hash: string;
    src: string;
  } | null>(null);
  const found = loaded?.for === `${key} ${size}` ? loaded : null;

  useEffect(() => {
    if (!key || typeof crypto === "undefined" || !crypto.subtle) return;
    const forKey = `${key} ${size}`;
    let live = true;
    void gravatarHash(key).then(async (hash) => {
      const src = gravatarUrl(hash, AVATAR_PIXELS[size], isRetinaScreen());
      if ((await hasGravatar(hash, src)) && live) {
        setLoaded({ for: forKey, hash, src });
      }
    });
    return () => {
      live = false;
    };
  }, [key, size]);

  if (found) {
    return (
      <img
        src={found.src}
        alt={t("avatars.gravatarAlt", { email: key })}
        data-slot="avatar-gravatar"
        className={cn(ROUND, "border", AVATAR_SIZE_CLASS[size], className)}
        onError={() => {
          forgetGravatar(found.hash);
          setLoaded(null);
        }}
      />
    );
  }
  return (
    <MonogramAvatar
      monogram={emailMonogram(key)}
      size={size}
      className={className}
    />
  );
}

/**
 * A person's avatar: their upload, else their Gravatar, else the monogram of
 * their email's first letter.
 */
export function UserAvatar({
  email,
  imageURL,
  size = "medium",
  className,
}: {
  email: string | null | undefined;
  imageURL?: string | null;
  size?: AvatarSize;
  className?: string;
}) {
  const { t } = useTranslations();
  return (
    <UploadedOr
      imageURL={imageURL}
      alt={t("avatars.uploadedAlt", { email: normalizeEmail(email) })}
      size={size}
      className={className}
      fallback={
        <GravatarOrMonogram
          email={email ?? ""}
          size={size}
          className={className}
        />
      }
    />
  );
}

/**
 * A workspace's avatar: its upload, else the monogram of its name's first
 * letter, colored by its id. Workspaces have no Gravatar.
 */
export function WorkspaceAvatar({
  id,
  name,
  imageURL,
  size = "medium",
  className,
}: {
  id: string;
  name: string | null | undefined;
  imageURL?: string | null;
  size?: AvatarSize;
  className?: string;
}) {
  const { t } = useTranslations();
  return (
    <UploadedOr
      imageURL={imageURL}
      alt={t("avatars.workspaceUploadedAlt")}
      size={size}
      className={className}
      fallback={
        <MonogramAvatar
          monogram={workspaceMonogram(id, name)}
          size={size}
          className={className}
        />
      }
    />
  );
}
