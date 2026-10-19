"use client";

import { useEffect, useRef, useState } from "react";

// A miniature formula car that replaces the mouse pointer on desktop. It is
// decoration only: pointer-events are off, the hotspot is the tip of the nose
// (exactly where the real pointer is), and text fields get the native I-beam
// back. It is skipped on touch devices, with reduced motion, and when the
// user switches it off (CursorToggle, stored in localStorage).
//
// Movement runs in one requestAnimationFrame loop that writes styles
// directly, so moving the mouse never re-renders React. The loop stops itself
// when the car is still and starts again on the next mouse move.

const PREF_KEY = "apex-cursor";
const PREF_EVENT = "apex-cursor-pref";

const VB_W = 60;
const VB_H = 140;
const HEIGHT = 46; // px on screen
const SCALE = HEIGHT / VB_H;
const WIDTH = VB_W * SCALE;
const NOSE_X = 30 * SCALE; // the hotspot, in px from the car's top-left corner
const NOSE_Y = 2 * SCALE;
const TAIL = 134 * SCALE; // nose to rear wing
const WING_HALF = 20 * SCALE; // rear wing endplates, from the centre line
const REST_ANGLE = -32; // nose up and to the left, like a normal arrow pointer
const TRAIL_MS = 260;
const TRAIL_SPEED = 7; // px per frame before the slipstream shows

const TEXT_FIELDS =
  'input:not([type="checkbox"]):not([type="radio"]):not([type="button"]):not([type="submit"]):not([type="reset"]):not([type="file"]):not([type="range"]):not([type="color"]), textarea, select, [contenteditable=""], [contenteditable="true"]';
const CLICKABLE = 'a, button, label, summary, [role="button"], [role="tab"], input[type="checkbox"], input[type="radio"], input[type="file"]';

function readPref() {
  try {
    return localStorage.getItem(PREF_KEY) !== "off";
  } catch {
    return true;
  }
}

function writePref(on) {
  try {
    localStorage.setItem(PREF_KEY, on ? "on" : "off");
  } catch {
    // Storage blocked: the choice lasts for this page only.
  }
  window.dispatchEvent(new CustomEvent(PREF_EVENT, { detail: on }));
}

function finePointer() {
  return window.matchMedia("(hover: hover) and (pointer: fine)").matches;
}

export default function RacingCursor() {
  const car = useRef(null);
  const canvas = useRef(null);
  const [enabled, setEnabled] = useState(false);

  // Decide whether the car is shown, and follow later changes.
  useEffect(() => {
    const fine = window.matchMedia("(hover: hover) and (pointer: fine)");
    const calm = window.matchMedia("(prefers-reduced-motion: reduce)");
    let pref = readPref();
    const update = () => setEnabled(fine.matches && !calm.matches && pref);
    const onPref = (e) => {
      pref = typeof e.detail === "boolean" ? e.detail : readPref();
      update();
    };
    const onStorage = (e) => e.key === PREF_KEY && onPref({});
    update();
    fine.addEventListener("change", update);
    calm.addEventListener("change", update);
    window.addEventListener(PREF_EVENT, onPref);
    window.addEventListener("storage", onStorage);
    return () => {
      fine.removeEventListener("change", update);
      calm.removeEventListener("change", update);
      window.removeEventListener(PREF_EVENT, onPref);
      window.removeEventListener("storage", onStorage);
    };
  }, []);

  useEffect(() => {
    if (!enabled) return;
    const el = car.current;
    const cv = canvas.current;
    const ctx = cv.getContext("2d");
    const wheelL = el.querySelector('[data-wheel="l"]');
    const wheelR = el.querySelector('[data-wheel="r"]');
    const root = document.documentElement;
    root.classList.add("apex-cursor");

    let x = 0;
    let y = 0;
    let lastX = 0;
    let lastY = 0;
    let vx = 0;
    let vy = 0;
    let angle = REST_ANGLE;
    let heading = REST_ANGLE;
    let steer = 0;
    let visible = false;
    let raf = 0;
    const trail = []; // { lx, ly, rx, ry, t } rear wing endplates over time

    function resize() {
      const dpr = Math.min(window.devicePixelRatio || 1, 2);
      cv.width = Math.round(window.innerWidth * dpr);
      cv.height = Math.round(window.innerHeight * dpr);
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    }

    function start() {
      if (!raf) raf = requestAnimationFrame(frame);
    }

    function frame(now) {
      raf = 0;
      const dx = x - lastX;
      const dy = y - lastY;
      lastX = x;
      lastY = y;
      vx = vx * 0.7 + dx * 0.3;
      vy = vy * 0.7 + dy * 0.3;
      const speed = Math.hypot(vx, vy);

      // Point the nose where the mouse is going; turn by the shortest way.
      if (speed > 1.2) heading = (Math.atan2(vy, vx) * 180) / Math.PI + 90;
      const diff = ((heading - angle + 540) % 360) - 180;
      angle += diff * 0.2;
      steer = steer * 0.75 + Math.max(-26, Math.min(26, diff * 0.6)) * 0.25;

      el.style.transform = `translate3d(${x - NOSE_X}px, ${y - NOSE_Y}px, 0) rotate(${angle.toFixed(2)}deg)`;
      wheelL.setAttribute("transform", `rotate(${steer.toFixed(1)} 10 34)`);
      wheelR.setAttribute("transform", `rotate(${steer.toFixed(1)} 50 34)`);

      // Slipstream from the rear wing endplates, only while moving fast.
      if (visible && speed > TRAIL_SPEED) {
        const rad = ((angle - 90) * Math.PI) / 180;
        const fx = Math.cos(rad);
        const fy = Math.sin(rad);
        const tx = x - fx * TAIL;
        const ty = y - fy * TAIL;
        trail.push({ lx: tx + fy * WING_HALF, ly: ty - fx * WING_HALF, rx: tx - fy * WING_HALF, ry: ty + fx * WING_HALF, t: now });
      }
      while (trail.length && now - trail[0].t > TRAIL_MS) trail.shift();
      ctx.clearRect(0, 0, window.innerWidth, window.innerHeight);
      if (trail.length > 1) {
        ctx.lineCap = "round";
        ctx.lineWidth = 1.6;
        for (let i = 1; i < trail.length; i++) {
          const a = trail[i - 1];
          const b = trail[i];
          ctx.strokeStyle = `rgba(255, 48, 69, ${(0.5 * (1 - (now - b.t) / TRAIL_MS)).toFixed(3)})`;
          ctx.beginPath();
          ctx.moveTo(a.lx, a.ly);
          ctx.lineTo(b.lx, b.ly);
          ctx.moveTo(a.rx, a.ry);
          ctx.lineTo(b.rx, b.ry);
          ctx.stroke();
        }
      }

      const settling = Math.abs(diff) > 0.3 || Math.abs(steer) > 0.2 || speed > 0.05 || trail.length > 0;
      if (settling) start();
    }

    function onMove(e) {
      if (e.pointerType && e.pointerType !== "mouse") return;
      x = e.clientX;
      y = e.clientY;
      if (!visible) {
        visible = true;
        lastX = x;
        lastY = y;
        el.classList.add("is-visible");
      }
      start();
    }

    function onOver(e) {
      const target = e.target instanceof Element ? e.target : null;
      const text = Boolean(target?.closest(TEXT_FIELDS));
      el.classList.toggle("is-text", text);
      el.classList.toggle("is-hover", !text && Boolean(target?.closest(CLICKABLE)));
    }

    function onOut(e) {
      if (e.relatedTarget) return; // still inside the page
      visible = false;
      el.classList.remove("is-visible");
    }

    const onDown = () => el.classList.add("is-down");
    const onUp = () => el.classList.remove("is-down");

    resize();
    window.addEventListener("resize", resize);
    window.addEventListener("pointermove", onMove, { passive: true });
    window.addEventListener("dragover", onMove, { passive: true });
    document.addEventListener("mouseover", onOver, { passive: true });
    document.addEventListener("mouseout", onOut, { passive: true });
    window.addEventListener("pointerdown", onDown, { passive: true });
    window.addEventListener("pointerup", onUp, { passive: true });

    return () => {
      cancelAnimationFrame(raf);
      root.classList.remove("apex-cursor");
      window.removeEventListener("resize", resize);
      window.removeEventListener("pointermove", onMove);
      window.removeEventListener("dragover", onMove);
      document.removeEventListener("mouseover", onOver);
      document.removeEventListener("mouseout", onOut);
      window.removeEventListener("pointerdown", onDown);
      window.removeEventListener("pointerup", onUp);
    };
  }, [enabled]);

  if (!enabled) return null;

  return (
    <>
      <canvas ref={canvas} className="apex-trail" aria-hidden="true" />
      <div
        ref={car}
        className="apex-car"
        aria-hidden="true"
        style={{ width: WIDTH, height: HEIGHT, transformOrigin: `${NOSE_X}px ${NOSE_Y}px` }}
      >
        <div className="apex-car-body" style={{ transformOrigin: `${NOSE_X}px ${NOSE_Y}px` }}>
          <CarSvg />
        </div>
      </div>
    </>
  );
}

// The Settings row for the car pointer. Where there is no mouse or trackpad
// the car never appears, so the row says so instead of offering a switch.
export function CursorSetting() {
  const [on, setOn] = useState(null);
  const [supported, setSupported] = useState(true);

  useEffect(() => {
    if (finePointer()) setOn(readPref());
    else setSupported(false);
  }, []);

  return (
    <div className="setting-row">
      <span className="setting-row-text">
        <strong>Racing cursor</strong>
        <span className="muted">
          {supported
            ? "Replace the mouse pointer with the APEX car. It is always off when your system asks for reduced motion."
            : "Available on devices with a mouse or trackpad."}
        </span>
      </span>
      {on !== null && (
        <label className="switch">
          <input
            type="checkbox"
            checked={on}
            aria-label="Racing cursor"
            onChange={(e) => {
              setOn(e.target.checked);
              writePref(e.target.checked);
            }}
          />
          <span className="switch-track" aria-hidden="true" />
        </label>
      )}
    </div>
  );
}

// Top-down car, nose up. viewBox 0 0 60 140; the nose tip is at (30, 2).
export function CarSvg({ p = "apx", width = WIDTH, height = HEIGHT }) {
  return (
    <svg viewBox={`0 0 ${VB_W} ${VB_H}`} width={width} height={height} focusable="false" aria-hidden="true">
      <defs>
        <linearGradient id={`${p}-body`} x1="0" x2="1">
          <stop offset="0" stopColor="#9E0E22" />
          <stop offset=".32" stopColor="#FF3045" />
          <stop offset=".5" stopColor="#FF6B79" />
          <stop offset=".68" stopColor="#FF3045" />
          <stop offset="1" stopColor="#9E0E22" />
        </linearGradient>
        <linearGradient id={`${p}-carbon`} x1="0" x2="1">
          <stop offset="0" stopColor="#05070B" />
          <stop offset=".5" stopColor="#222B39" />
          <stop offset="1" stopColor="#05070B" />
        </linearGradient>
        <linearGradient id={`${p}-tire`} x1="0" x2="1">
          <stop offset="0" stopColor="#050506" />
          <stop offset=".42" stopColor="#2B2F37" />
          <stop offset=".6" stopColor="#16181D" />
          <stop offset="1" stopColor="#050506" />
        </linearGradient>
        <radialGradient id={`${p}-helmet`} cx=".38" cy=".35" r=".7">
          <stop offset="0" stopColor="#FFFFFF" />
          <stop offset=".45" stopColor="#FFE38A" />
          <stop offset="1" stopColor="#E19A00" />
        </radialGradient>
        <linearGradient id={`${p}-halo`} x1="0" x2="1">
          <stop offset="0" stopColor="#6B7586" />
          <stop offset=".5" stopColor="#E6EBF2" />
          <stop offset="1" stopColor="#6B7586" />
        </linearGradient>
      </defs>

      {/* suspension, under the bodywork */}
      <g stroke="#AEB7C6" strokeWidth="1.3" strokeLinecap="round">
        <path d="M15 29 25 36M15 39 25 43M45 29 35 36M45 39 35 43" />
        <path d="M16 104 24 99M16 114 24 110M44 104 36 99M44 114 36 110" />
      </g>

      {/* front tyres, steered by the cursor loop */}
      <g data-wheel="l">
        <Tire x={4} y={23} w={12} h={22} p={p} />
      </g>
      <g data-wheel="r">
        <Tire x={44} y={23} w={12} h={22} p={p} />
      </g>
      {/* rear tyres */}
      <Tire x={2} y={96} w={15} h={26} p={p} />
      <Tire x={43} y={96} w={15} h={26} p={p} />

      {/* floor */}
      <path d="M18 48h24l5.5 48-2.5 22H15l-2.5-22Z" fill="#07090D" />

      {/* sidepods with inlets and undercuts */}
      <path d="M20 56c-6 2-8 10-8 18l1 18c1 6 7 10 11 12h12c4-2 10-6 11-12l1-18c0-8-2-16-8-18Z" fill={`url(#${p}-body)`} />
      <path d="M13.6 65c0-5 3-8 7.4-8.4L22 65ZM46.4 65c0-5-3-8-7.4-8.4L38 65Z" fill="#05070B" />
      <path d="M12.6 78h2.6l1 16-2.8-2.6ZM47.4 78h-2.6l-1 16 2.8-2.6Z" fill="#0B0E14" />
      <path d="M15.4 67c.2-3.4 1.7-6 3.6-7.2V88l-3.4-3.6Z" fill="#fff" opacity=".12" />

      {/* front wing: endplates, main plane, red flap with a white edge */}
      <rect x="2" y="6" width="3" height="13" rx="1" fill="#0B0E14" stroke="#3A4456" strokeWidth=".5" />
      <rect x="55" y="6" width="3" height="13" rx="1" fill="#0B0E14" stroke="#3A4456" strokeWidth=".5" />
      <path d="M4 12h52v5c-17 2.6-35 2.6-52 0Z" fill={`url(#${p}-carbon)`} />
      <path d="M5 8.6h50v2.8c-16.6 2-33.4 2-50 0Z" fill={`url(#${p}-body)`} />
      <path d="M7 9.6h15M38 9.6h15" stroke="#F5F7FA" strokeWidth=".6" opacity=".8" />
      <rect x="26.6" y="10" width="1.1" height="8" fill="#0B0E14" />
      <rect x="32.3" y="10" width="1.1" height="8" fill="#0B0E14" />

      {/* nose and monocoque */}
      <path d="M30 3c2.4 0 3.4 3 3.6 7L35 40l1.5 14h-13L25 40l1.4-30c.2-4 1.2-7 3.6-7Z" fill={`url(#${p}-body)`} />
      <path d="M29.2 7h1.6l.6 33h-2.8Z" fill="#F5F7FA" opacity=".92" />
      <path d="M27.4 4.2Q30 1.6 32.6 4.2l.2 2.8h-5.6Z" fill="#0B0E14" />
      <path d="M27.2 11h.8l.5 29h-2Z" fill="#fff" opacity=".2" />

      {/* engine cover with the white spine and airbox */}
      <path d="M23.5 54c-1.5 8-1.5 16 .5 26l3 30h6l3-30c2-10 2-18 .5-26Z" fill={`url(#${p}-body)`} />
      <path d="M29.3 72h1.4l.5 36h-2.4Z" fill="#F5F7FA" opacity=".85" />
      <ellipse cx="30" cy="70.6" rx="3" ry="2" fill="#05070B" stroke="#FF6B79" strokeWidth=".5" />

      {/* mirrors */}
      <path d="M24 50.5h-4.5M36 50.5h4.5" stroke="#0B0E14" strokeWidth=".9" />
      <rect x="18" y="49.4" width="3.4" height="2" rx=".6" fill="#0B0E14" stroke="#3A4456" strokeWidth=".4" />
      <rect x="38.6" y="49.4" width="3.4" height="2" rx=".6" fill="#0B0E14" stroke="#3A4456" strokeWidth=".4" />

      {/* cockpit, driver, halo */}
      <path d="M25 52c0-5 2-7 5-7s5 2 5 7l.4 14c-1.4 2-9.4 2-10.8 0Z" fill="#05070B" stroke="#fff" strokeOpacity=".14" strokeWidth=".5" />
      <circle cx="30" cy="58.4" r="4.2" fill={`url(#${p}-helmet)`} />
      <path d="M26.7 56.6Q30 53.8 33.3 56.6l-.4 1.4Q30 56 27.1 58Z" fill="#0B0E14" />
      <rect x="29.45" y="58.6" width="1.1" height="3.8" rx=".5" fill="#FF3045" />
      <path d="M24.4 62c-.4-9.6 1.6-13.4 5.6-13.4s6 3.8 5.6 13.4M30 48.6V43" fill="none" stroke={`url(#${p}-halo)`} strokeWidth="1.6" strokeLinecap="round" />

      {/* diffuser with strakes */}
      <path d="M16 116h28l2 10H14Z" fill="#05070B" />
      <path d="M20 117v8M25 117v8.6M30 117v8.6M35 117v8.6M40 117v8" stroke="#303847" strokeWidth=".8" />

      {/* rear wing: pylon, endplates, main plane, DRS flap, rain light */}
      <rect x="29" y="108" width="2" height="16" fill="#0B0E14" />
      <rect x="8" y="120" width="2.6" height="17" rx=".8" fill="#0B0E14" stroke="#3A4456" strokeWidth=".5" />
      <rect x="49.4" y="120" width="2.6" height="17" rx=".8" fill="#0B0E14" stroke="#3A4456" strokeWidth=".5" />
      <rect x="10" y="124" width="40" height="6" fill={`url(#${p}-carbon)`} />
      <rect x="10" y="131" width="40" height="4" fill={`url(#${p}-body)`} />
      <rect x="10" y="130.2" width="40" height=".6" fill="#F5F7FA" opacity=".55" />
      <rect x="28.4" y="126" width="3.2" height="2" rx=".4" fill="#FF3045" className="apex-car-rainlight" />
    </svg>
  );
}

function Tire({ x, y, w, h, p }) {
  return (
    <>
      <rect x={x} y={y} width={w} height={h} rx="3" fill={`url(#${p}-tire)`} />
      <rect x={x + w / 2 - 0.4} y={y + 1} width=".8" height={h - 2} fill="#3A3F48" />
      <rect x={x + 0.6} y={y + 1.2} width="1.1" height={h - 2.4} rx=".5" fill="#fff" opacity=".12" />
      <path d={`M${x + 1.5} ${y + 1.2}h${w - 3}M${x + 1.5} ${y + h - 1.2}h${w - 3}`} stroke="#FFB800" strokeWidth=".6" opacity=".7" />
    </>
  );
}
