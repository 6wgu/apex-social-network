// A user's picture, or their initials on a coloured circle when they have none.
// `user` is a User or UserSummary; `src` overrides the image (file preview).
export default function Avatar({ user, src, size = 40 }) {
  const image = src || user?.avatar_path;
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
