"use client";

import { createContext, useCallback, useContext, useEffect, useRef, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { api, PUBLIC_PAGES } from "@/lib/api";
import { Loading } from "@/components/RacingLights";
import LaunchOverlay, { MORPH_MS } from "@/components/LaunchOverlay";

const ARRIVE_MS = 650;
const BYE_MS = 700;
const FADE_MS = 350;
const pause = (ms) => new Promise((done) => setTimeout(done, ms));

const AuthContext = createContext({ user: null, loading: true, setUser() {}, logout() {}, leave() {} });

export function useAuth() {
  return useContext(AuthContext);
}

export default function AuthProvider({ children }) {
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(true);
  const [exitTo, setExitTo] = useState(null);
  const pathname = usePathname();

  const updateUser = useCallback((nextUser) => {
    setUser(nextUser);
    if (typeof window !== "undefined" && nextUser?.id) {
      window.dispatchEvent(
        new CustomEvent("user-avatar-changed", {
          detail: { id: nextUser.id, avatarPath: nextUser.avatar_path || "" },
        })
      );
    }
  }, []);

  useEffect(() => {
    api("/api/me", { redirectOn401: false })
      .then(updateUser)
      .catch(() => setUser(null))
      .finally(() => setLoading(false));
  }, [updateUser]);

  const leave = useCallback(
    (path) => {
      setExitTo(path);
      updateUser(null);
    },
    [updateUser]
  );

  useEffect(() => {
    if (exitTo === pathname) setExitTo(null);
  }, [exitTo, pathname]);

  const [farewell, setFarewell] = useState(null);
  const leaving = useRef(false);

  const logout = useCallback(
    async (options) => {
      if (leaving.current) return;
      leaving.current = true;
      const animate = !options?.quiet && !window.matchMedia("(prefers-reduced-motion: reduce)").matches;
      if (animate) {
        setFarewell("arrive");
        await pause(ARRIVE_MS);
        setFarewell("park");
        await pause(MORPH_MS);
        setFarewell("bye");
        await pause(BYE_MS);
      }
      await api("/api/logout", { method: "POST", redirectOn401: false }).catch(() => {
      });
      leave("/");
      if (animate) {
        await pause(450);
        setFarewell("fade");
        await pause(FADE_MS);
        setFarewell(null);
      }
      leaving.current = false;
    },
    [leave]
  );

  return (
    <AuthContext.Provider value={{ user, loading, setUser: updateUser, logout, leave, exitTo }}>
      {children}
      <LaunchOverlay phase={farewell} word="See you soon" label="Signing out…" />
    </AuthContext.Provider>
  );
}

export function AuthGate({ children }) {
  const { user, loading, exitTo } = useAuth();
  const pathname = usePathname();
  const router = useRouter();
  const isLandingPage = pathname === "/";
  const isPublicPage = PUBLIC_PAGES.includes(pathname) && !isLandingPage;
  const mustVerify = Boolean(user) && !user.is_verified;
  const allowed =
    !loading && (mustVerify ? pathname === "/verify" : isLandingPage || (isPublicPage ? !user : Boolean(user)));

  useEffect(() => {
    if (loading) return;
    if (mustVerify && pathname !== "/verify") router.replace("/verify");
    else if (!user && !isPublicPage && !isLandingPage) router.replace(exitTo || "/login");
    else if (user && isPublicPage) router.replace("/");
  }, [loading, user, mustVerify, isPublicPage, isLandingPage, pathname, router, exitTo]);

  if (!allowed) return <Loading className="muted center loading-screen" />;
  return children;
}
