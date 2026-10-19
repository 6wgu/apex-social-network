// Five starting lights in small housings, drawn and animated in CSS
// (.racing-lights in apex.css).
//   mode "start"  the race-start sequence: on one by one, hold, all off.
//                 Used wherever the app is already waiting for something.
//   mode "cycle"  each light cycles red, yellow, cyan, green, purple, a step
//                 behind its neighbour. Meant for a typing indicator.
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

// A loading line: the start lights and the text that says what is loading.
export function Loading({ children = "Loading…", className = "muted center" }) {
  return (
    <p className={`loading-state ${className}`} role="status">
      <RacingLights />
      <span>{children}</span>
    </p>
  );
}

// "Name is typing…" with the colour-cycling lights. The backend does not
// send typing events yet, so nothing renders this today; show it only from a
// real typing state once one exists.
export function TypingIndicator({ name }) {
  return (
    <p className="typing-indicator" role="status">
      <RacingLights mode="cycle" small />
      <span>{name} is typing…</span>
    </p>
  );
}
