"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

const links = [
  { href: "/", label: "Home" },
  { href: "/groups", label: "Groups" },
  { href: "/chats", label: "Chats" },
];

// Shown on every page. The bell and chat badge are slots filled in Phase 2/4;
// Logout and the profile link are wired up in Phase 1.
export default function Navbar() {
  const pathname = usePathname();

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
        <button className="icon-btn" aria-label="Notifications" data-slot="notifications">
          🔔
          <span className="badge badge-notification" hidden />
        </button>
        <Link href="/profile/me" className="icon-btn" aria-label="My profile">
          👤
        </Link>
        <button className="btn-secondary" data-slot="logout">
          Logout
        </button>
      </div>
    </header>
  );
}
