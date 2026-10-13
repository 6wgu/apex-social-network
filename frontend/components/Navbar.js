"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useAuth } from "@/components/AuthProvider";
import Avatar from "@/components/Avatar";
import NotificationBell from "@/components/NotificationBell";

const links = [
  { href: "/", label: "Home" },
  { href: "/groups", label: "Groups" },
  { href: "/chats", label: "Chats" },
];

// Shown on every page. Logged out, it only offers Log in / Register.
// The notification bell (red, round badge) is live; the chat badge (blue,
// square) is a slot filled in Phase 4, so the two never look alike.
export default function Navbar() {
  const pathname = usePathname();
  const { user, loading, logout } = useAuth();

  if (!user) {
    return (
      <header className="navbar">
        <Link href="/login" className="brand">
          social<span>network</span>
        </Link>
        <span className="nav-links" />
        {!loading && (
          <div className="nav-actions">
            <Link href="/login" className="btn-secondary">
              Log in
            </Link>
            <Link href="/register" className="btn-primary">
              Register
            </Link>
          </div>
        )}
      </header>
    );
  }

  return (
    <header className="navbar">
      <Link href="/" className="brand">
        social<span>network</span>
      </Link>

      <nav className="nav-links">
        {links.map((l) => (
          <Link key={l.href} href={l.href} className={pathname === l.href ? "active" : ""}>
            {l.label}
            {l.href === "/chats" && <span className="badge badge-message" data-slot="unread-messages" hidden />}
          </Link>
        ))}
      </nav>

      <div className="nav-actions">
        <NotificationBell />
        <Link href={`/profile/${user.id}`} className="nav-profile" aria-label="My profile">
          <Avatar user={user} size={32} />
          <span>{user.first_name}</span>
        </Link>
        <button className="btn-secondary" onClick={logout}>
          Logout
        </button>
      </div>
    </header>
  );
}
