export default function RacingLights({ mode = "start", small = false }) {
  return (
    <span className={`racing-lights racing-lights-${mode}${small ? " racing-lights-sm" : ""}`} aria-hidden="true">
      <i />
      <i />
      <i />
      <i />
      <i />
    </span>
  );
}

export function Loading({ children = "Loading…", className = "muted center" }) {
  return (
    <p className={`loading-state ${className}`} role="status">
      <RacingLights />
      <span>{children}</span>
    </p>
  );
}

export function TypingIndicator({ name, plural = false }) {
  return (
    <p className="typing-indicator" role="status">
      <RacingLights mode="cycle" small />
      <span>{name} {plural ? "are" : "is"} typing…</span>
    </p>
  );
}
