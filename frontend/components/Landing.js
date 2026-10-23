"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import Icon from "@/components/Icon";
import RacingLights from "@/components/RacingLights";
import { CarSvg } from "@/components/RacingCursor";
import SamplePost from "@/components/SamplePost";

// Each timing sector stands for one part of the app.
const SECTORS = [
  { key: "s1", label: "S1", name: "Profiles", text: "Choose who sees your profile, posts, and conversations." },
  { key: "s2", label: "S2", name: "Groups", text: "Follow friends, discover groups, and plan events together." },
  { key: "s3", label: "S3", name: "Chat", text: "Message people live, and get notifications that are worth it." },
];

// The finish-line start: all five lights red, yellow for 400 ms, then green,
// and a reaction timer runs below and stops at 0.695 s. It plays while the
// section is on screen and repeats; with reduced motion it just shows the end.
const RED_MS = 1400;
const YELLOW_MS = 400;
const REACTION_MS = 695;
const HOLD_MS = 2600;

function StartSequence() {
  const ref = useRef(null);
  const [phase, setPhase] = useState("red");
  const [ms, setMs] = useState(0);

  useEffect(() => {
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
      setPhase("green");
      setMs(REACTION_MS);
      return;
    }
    let timers = [];
    let frame = 0;
    const clear = () => {
      timers.forEach(clearTimeout);
      timers = [];
      cancelAnimationFrame(frame);
    };
    const run = () => {
      clear();
      setPhase("red");
      setMs(0);
      timers.push(setTimeout(() => setPhase("yellow"), RED_MS));
      timers.push(
        setTimeout(() => {
          setPhase("green");
          const start = performance.now();
          const tick = (now) => {
            const t = Math.min(now - start, REACTION_MS);
            setMs(t);
            if (t < REACTION_MS) frame = requestAnimationFrame(tick);
            else timers.push(setTimeout(run, HOLD_MS));
          };
          frame = requestAnimationFrame(tick);
        }, RED_MS + YELLOW_MS)
      );
    };
    const seen = new IntersectionObserver(([entry]) => (entry.isIntersecting ? run() : clear()), { threshold: 0.4 });
    seen.observe(ref.current);
    return () => {
      seen.disconnect();
      clear();
    };
  }, []);

  return (
    <div className="start-seq" ref={ref}>
      <span className={`racing-lights racing-lights-static is-${phase}`} aria-hidden="true">
        <i />
        <i />
        <i />
        <i />
        <i />
      </span>
      <span className={`reaction-timer${phase === "green" && ms >= REACTION_MS ? " is-stopped" : ""}`} aria-hidden="true">
        {(ms / 1000).toFixed(3)}
        <small>s</small>
      </span>
    </div>
  );
}

// The hero car: launches in when the start lights go out, then floats,
// with speed streaks behind it and the three sectors tagged around it.
function HeroCar() {
  return (
    <div className="hero-car" aria-hidden="true">
      <div className="hero-car-glow" />
      <div className="hero-car-streaks">
        {Array.from({ length: 12 }, (_, i) => (
          <span key={i} style={{ "--i": i }} />
        ))}
      </div>
      <div className="hero-car-shadow" />
      <div className="hero-car-float">
        <div className="hero-car-pose">
          <CarSvg p="apx-hero" width={220} height={513} />
        </div>
      </div>
      {SECTORS.map((s) => (
        <span key={s.key} className={`hero-tag hero-tag-${s.key}`}>
          <i />
          {s.label} · {s.name}
        </span>
      ))}
    </div>
  );
}

// The logged-out home page.
export default function Landing() {
  return (
    <div className="landing">
      <section className="landing-hero">
        <div className="landing-copy">
          <div className="landing-lights" aria-hidden="true">
            <RacingLights mode="once" />
            <span>Lights out</span>
          </div>
          <p className="eyebrow">Posts · Groups · Chat</p>
          <h1>Connect at Full Speed.</h1>
          <p className="landing-lede">
            A precise, people-first social space for updates, conversations, groups, and the people you actually want to hear from.
          </p>
          <div className="landing-actions">
            <Link href="/register" className="btn-primary btn-large">
              Create your account
            </Link>
            <Link href="/login" className="text-link">
              Already a member? Log in
            </Link>
          </div>
          <ul className="landing-facts">
            <li>
              <Icon name="lock" size={16} /> Private by default
            </li>
            <li>
              <Icon name="chat" size={16} /> Live chat
            </li>
            <li>
              <Icon name="calendar" size={16} /> Group events
            </li>
          </ul>
        </div>

        <HeroCar />
      </section>

      <div className="kerb-strip" aria-hidden="true" />

      <section className="sectors" aria-label="What APEX does">
        {SECTORS.map((s, i) => (
          <article key={s.key} className={`sector-card sector-${s.key}`}>
            <span className="sector-number">0{i + 1}</span>
            <span className="sector-tag">Sector {i + 1}</span>
            <h2>{s.name}</h2>
            <p>{s.text}</p>
          </article>
        ))}
      </section>

      <section className="paddock">
        <div className="paddock-copy">
          <p className="eyebrow">The paddock</p>
          <h2>Your circle, in one feed.</h2>
          <p>
            Posts from the people you follow, newest first. Pick who sees each post: everyone, only your followers, or a few people
            you choose.
          </p>
          <Link href="/register" className="btn-secondary">
            Join APEX
          </Link>
        </div>
        <div className="landing-preview" aria-label="Social network preview">
          <div className="preview-topline">
            <span className="preview-dot" />
            <span>APEX / your circle</span>
            <span className="preview-status">live · private by default</span>
          </div>
          <SamplePost />
        </div>
      </section>

      <section className="finish">
        <StartSequence />
        <h2>Your grid slot is ready.</h2>
        <p>It takes a minute to sign up.</p>
        <div className="landing-actions">
          <Link href="/register" className="btn-primary btn-large">
            Create your account
          </Link>
          <Link href="/login" className="text-link">
            Log in
          </Link>
        </div>
      </section>
    </div>
  );
}
