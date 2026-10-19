"use client";

import { createContext, useCallback, useContext, useEffect, useRef, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { api, PUBLIC_PAGES } from "@/lib/api";
import { Loading } from "@/components/RacingLights";
import LaunchOverlay, { MORPH_MS } from "@/components/LaunchOverlay";

// The sign-out animation: the car drives in and parks, turns back into the
// logo, says goodbye, and the overlay fades once the home page is showing.
const ARRIVE_MS = 650;
const BYE_MS = 700;
const FADE_MS = 350;
const pause = (ms) => new Promise((done) => setTimeout(done, ms));

const AuthContext = createContext({ user: null, loading: true, setUser() {}, logout() {} });

/** useAuth() → { user, loading, setUser, logout }. user is null when logged out. */
export function useAuth() {
  return useContext(AuthContext);
}

// Loads the current user once (GET /api/me) and keeps it for every page.
// The backend is still what protects the data: this only decides which page
// the browser shows.
export default function AuthProvider({ children }) {
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(true);
  const router = useRouter();

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

  const [farewell, setFarewell] = useState(null); // sign-out animation phase
  const leaving = useRef(false);

  // logout() plays the sign-out animation; logout({ quiet: true }) skips it
  // (used when the session ended on its own).
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
      // Sign out after the animation, so the page underneath never meets a
      // closed session halfway through it.
      await api("/api/logout", { method: "POST", redirectOn401: false }).catch(() => {
        // Already logged out on the server: nothing left to undo.
      });
      // Go home first, then forget the user: cleared on a private page, the
      // user would be sent to /login by AuthGate instead.
      router.replace("/");
      await pause(200);
      updateUser(null);
      if (animate) {
        await pause(250);
        setFarewell("fade");
        await pause(FADE_MS);
        setFarewell(null);
      }
      leaving.current = false;
    },
    [router, updateUser]
  );

  return (
    <AuthContext.Provider value={{ user, loading, setUser: updateUser, logout }}>
      {children}
      <LaunchOverlay phase={farewell} word="See you soon" label="Signing out…" />
    </AuthContext.Provider>
  );
}

// Wraps the page content: logged-out visitors are sent to /login, and
// logged-in users who open /login or /register are sent home. Nothing is
// rendered until we know which case applies, so a page never flashes.
export function AuthGate({ children }) {
  const { user, loading } = useAuth();
  const pathname = usePathname();
  const router = useRouter();
  const isLandingPage = pathname === "/";
  const isPublicPage = PUBLIC_PAGES.includes(pathname) && !isLandingPage;
  // Signed in but not verified yet: the code page is the only page there is
  // (the backend answers 403 everywhere else too).
  const mustVerify = Boolean(user) && !user.is_verified;
  const allowed =
    !loading && (mustVerify ? pathname === "/verify" : isLandingPage || (isPublicPage ? !user : Boolean(user)));

  useEffect(() => {
    if (loading) return;
    if (mustVerify && pathname !== "/verify") router.replace("/verify");
    else if (!user && !isPublicPage && !isLandingPage) router.replace("/login");
    else if (user && isPublicPage) router.replace("/");
  }, [loading, user, mustVerify, isPublicPage, isLandingPage, pathname, router]);

  if (!allowed) return <Loading className="muted center loading-screen" />;
  return children;
}
