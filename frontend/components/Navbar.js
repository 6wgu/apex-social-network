"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useState } from "react";
import { useAuth } from "@/components/AuthProvider";
import Avatar from "@/components/Avatar";
import Icon from "@/components/Icon";
import NotificationBell from "@/components/NotificationBell";
import { useUnreadMessages } from "@/components/UnreadMessages";

const COLLAPSE_KEY = "apex-rail-collapsed";

const links = [
  { href: "/", label: "Feed", icon: "home" },
  { href: "/search", label: "Search", icon: "search" },
  { href: "/groups", label: "Groups", icon: "groups" },
  { href: "/chats", label: "Chat", icon: "chat" },
];

// The APEX wordmark: a forward-leaning A with speed lines.
function Brand({ tagline = false }) {
  return (
    <Link href="/" className="brand" aria-label="APEX home">
      <svg className="brand-mark" viewBox="0 0 44 28" aria-hidden="true" focusable="false">
        <path className="brand-lines" d="M1 11h11M3.5 16h8M6 21h4" />
        <path className="brand-a" fillRule="evenodd" d="M13 26 25 2h7.5L38 26h-6.6l-1.2-5.6h-7.6L19.8 26ZM24 15.2h5.2l-1.2-6Z" />
      </svg>
      <span className="brand-text">
        <span className="brand-word">APEX</span>
        {tagline && <span className="brand-tag">Connect at full speed.</span>}
      </span>
    </Link>
  );
}

function isActive(pathname, href) {
  return pathname === href || (href !== "/" && pathname.startsWith(`${href}/`));
}

// Logged out: a top bar that points to the other auth page.
// Logged in: the side rail (a bottom bar on phones). Notifications and
// messages never look alike: the bell's badge is red and round, unread
// private messages show as a red, square badge on Chat.
export default function Navbar() {
  const pathname = usePathname();
  const { user, loading, logout } = useAuth();
  const { unread } = useUnreadMessages();
  const [collapsed, setCollapsed] = useState(false);

  useEffect(() => {
    try {
      setCollapsed(localStorage.getItem(COLLAPSE_KEY) === "1");
    } catch {
      // Storage blocked: the rail starts open.
    }
  }, []);

  function toggleCollapsed() {
    setCollapsed((c) => {
      try {
        localStorage.setItem(COLLAPSE_KEY, c ? "0" : "1");
      } catch {
        // Not remembered, still toggled.
      }
      return !c;
    });
  }

  // Signed in but not verified: no menu yet, only a way out.
  if (user && !user.is_verified) {
    return (
      <header className="navbar navbar-guest">
        <Brand tagline />
        <span className="nav-links" />
        <div className="nav-actions">
          <span className="nav-prompt">Verify your email to continue</span>
          <button type="button" className="btn-secondary btn-small" onClick={logout}>
            Log out
          </button>
        </div>
      </header>
    );
  }

  if (!user) {
    const onLogin = pathname === "/login";
    const onRegister = pathname === "/register";
    return (
      <header className="navbar navbar-guest">
        <Brand tagline />
        <span className="nav-links" />
        {!loading && (
          <div className="nav-actions">
            {onLogin && (
              <>
                <span className="nav-prompt">New here?</span>
                <Link href="/register" className="btn-primary btn-small">
                  Sign up
                </Link>
              </>
            )}
            {onRegister && (
              <>
                <span className="nav-prompt">Already a member?</span>
                <Link href="/login" className="btn-primary btn-small">
                  Log in
                </Link>
              </>
            )}
            {!onLogin && !onRegister && (
              <>
                <Link href="/login" className="btn-secondary btn-small">
                  Log in
                </Link>
                <Link href="/register" className="btn-primary btn-small">
                  Sign up
                </Link>
              </>
            )}
          </div>
        )}
      </header>
    );
  }

  return (
    <header className={`navbar navbar-app${collapsed ? " is-collapsed" : ""}`}>
      <div className="rail-head">
        <Brand />
        <button
          type="button"
          className="rail-collapse"
          onClick={toggleCollapsed}
          aria-label={collapsed ? "Expand the menu" : "Collapse the menu"}
          aria-expanded={!collapsed}
        >
          <Icon name="chevronLeft" size={18} />
        </button>
      </div>

      <nav className="nav-links" aria-label="Main">
        {links.map((l) => (
          <Link key={l.href} href={l.href} className={`nav-item${isActive(pathname, l.href) ? " active" : ""}`} title={l.label} aria-label={l.href === "/chats" && unread ? `${l.label}, ${unread} unread messages` : l.label}>
            <Icon name={l.icon} className="nav-icon" />
            <span className="nav-label">{l.label}</span>
            {l.href === "/chats" && (
              <span className="badge badge-message" hidden={!unread} aria-label={`${unread} unread messages`}>
                {unread > 99 ? "99+" : unread}
              </span>
            )}
          </Link>
        ))}
        <NotificationBell />
        <Link href="/settings" className={`nav-item${isActive(pathname, "/settings") ? " active" : ""}`} title="Settings" aria-label="Settings">
          <Icon name="settings" className="nav-icon" />
          <span className="nav-label">Settings</span>
        </Link>
      </nav>

      <div className="rail-foot">
        <Link
          href={`/profile/${user.id}`}
          className={`rail-user${pathname === `/profile/${user.id}` || pathname === "/profile/me" ? " active" : ""}`}
          title="My profile"
        >
          <Avatar user={user} size={40} />
          <span className="rail-user-text">
            <strong>
              {user.first_name} {user.last_name}
            </strong>
            {user.nickname && <span className="rail-user-handle">@{user.nickname}</span>}
            <span className="rail-user-link">Profile</span>
          </span>
        </Link>
        <button type="button" className="nav-item rail-logout" onClick={logout} title="Logout" aria-label="Logout">
          <Icon name="logout" className="nav-icon" />
          <span className="nav-label">Logout</span>
        </button>
        <p className="rail-love">
          Made with <span className="love" role="img" aria-label="love">♥</span>
        </p>
      </div>
    </header>
  );
}
