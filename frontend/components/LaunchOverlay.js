"use client";

import { CarSvg } from "@/components/RacingCursor";

export const MORPH_MS = 380;
export const GO_MS = 720;

export default function LaunchOverlay({ phase, word = "APEX", label }) {
  if (!phase) return null;
  return (
    <div className={`launch launch-${phase}`} role="status" aria-live="polite">
      <span className="visually-hidden">{label || (phase === "wait" ? "Signing in…" : "Signed in")}</span>
      <div className="launch-streaks" aria-hidden="true">
        {Array.from({ length: 14 }, (_, i) => (
          <span key={i} style={{ "--i": i }} />
        ))}
      </div>
      <div className="launch-stage" aria-hidden="true">
        <svg className="launch-logo" viewBox="0 0 44 28">
          <path className="brand-lines" d="M1 11h11M3.5 16h8M6 21h4" />
          <path className="brand-a" fillRule="evenodd" d="M13 26 25 2h7.5L38 26h-6.6l-1.2-5.6h-7.6L19.8 26ZM24 15.2h5.2l-1.2-6Z" />
        </svg>
        <div className="launch-car">
          <span className="launch-trail" />
          <span className="launch-car-body">
            <CarSvg p="apx-launch" width={112} height={262} />
          </span>
        </div>
      </div>
      <p className="launch-word" aria-hidden="true">
        {word}
      </p>
    </div>
  );
}
