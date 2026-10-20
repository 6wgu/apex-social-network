"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { nicknameProblem } from "@/lib/validation";

const DEBOUNCE_MS = 400;

// Asks GET /api/nicknames/available about a nickname once the user stops
// typing for 400 ms, or at once through checkNow() (when the field loses
// focus). Typing is never blocked. status is one of:
//   "idle"       nothing to say: empty, a bad format (the page shows that
//                itself), the user's own nickname (`own`, in any case), or the
//                check failed (the server checks again on save)
//   "checking"   a request is on its way
//   "available" / "taken"
// Only the answer for the value currently in the field counts.
export default function useNicknameCheck(nickname, own = "") {
  const value = nickname.trim();
  const needed = value !== "" && !nicknameProblem(value) && value.toLowerCase() !== own.trim().toLowerCase();
  const [result, setResult] = useState({ value: "", status: "idle" });
  const latest = useRef("");

  const check = useCallback(async (v) => {
    latest.current = v;
    setResult({ value: v, status: "checking" });
    try {
      const { available } = await api(`/api/nicknames/available?nickname=${encodeURIComponent(v)}`, { redirectOn401: false });
      if (latest.current === v) setResult({ value: v, status: available ? "available" : "taken" });
    } catch {
      if (latest.current === v) setResult({ value: v, status: "idle" });
    }
  }, []);

  useEffect(() => {
    if (!needed) {
      latest.current = "";
      return;
    }
    const timer = setTimeout(() => check(value), DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [value, needed, check]);

  const current = needed && result.value === value ? result.status : "idle";
  const checkNow = () => {
    if (needed && current === "idle") check(value);
  };
  return { status: current, checkNow };
}
