"use client";

import { useEffect, useState } from "react";

// A user's picture, or their initials on a coloured circle when they have none.
// `user` is a User or UserSummary; `src` overrides the image (file preview).
export default function Avatar({ user, src, size = 40 }) {
  const [avatarOverride, setAvatarOverride] = useState(null);

  useEffect(() => {
    setAvatarOverride(null);
    const onAvatarChanged = (event) => {
      if (String(event.detail?.id) === String(user?.id)) {
        setAvatarOverride(event.detail.avatarPath);
      }
    };
    window.addEventListener("user-avatar-changed", onAvatarChanged);
    return () => window.removeEventListener("user-avatar-changed", onAvatarChanged);
  }, [user?.id]);

  const image = src || (avatarOverride !== null ? avatarOverride : user?.avatar_path);
  const style = { width: size, height: size, fontSize: Math.round(size * 0.4) };
  if (image) {
    // eslint-disable-next-line @next/next/no-img-element
    return <img className="avatar" src={image} alt="" style={style} />;
  }
  const initials = `${user?.first_name?.[0] || ""}${user?.last_name?.[0] || ""}`.toUpperCase() || "?";
  return (
    <span className="avatar avatar-initials" style={style} aria-hidden="true">
      {initials}
    </span>
  );
}
