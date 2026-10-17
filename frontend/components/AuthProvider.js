"use client";

import { createContext, useCallback, useContext, useEffect, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { api, PUBLIC_PAGES } from "@/lib/api";

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

  const logout = useCallback(async () => {
    try {
      await api("/api/logout", { method: "POST", redirectOn401: false });
    } catch {
      // Already logged out on the server: nothing left to undo.
    }
    updateUser(null);
    router.replace("/login");
  }, [router, updateUser]);

  return <AuthContext.Provider value={{ user, loading, setUser: updateUser, logout }}>{children}</AuthContext.Provider>;
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
  const allowed = !loading && (isLandingPage || (isPublicPage ? !user : Boolean(user)));

  useEffect(() => {
    if (loading) return;
    if (!user && !isPublicPage && !isLandingPage) router.replace("/login");
    if (user && isPublicPage) router.replace("/");
  }, [loading, user, isPublicPage, isLandingPage, router]);

  if (!allowed) return <p className="muted center">Loading…</p>;
  return children;
}
