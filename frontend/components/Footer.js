"use client";

import Link from "next/link";
import { useAuth } from "@/components/AuthProvider";

// The footer of the public pages (home, log in, register). Signed in, the
// side rail carries the "made with love" line instead, because the feed and
// chat fill the screen.
export default function Footer() {
  const { user, loading } = useAuth();
  if (loading || user) return null;

  return (
    <footer className="site-footer">
      <div className="site-footer-inner">
        <Link href="/" className="site-footer-brand" aria-label="APEX home">
          <svg viewBox="0 0 44 28" aria-hidden="true" focusable="false">
            <path className="brand-lines" d="M1 11h11M3.5 16h8M6 21h4" />
            <path className="brand-a" fillRule="evenodd" d="M13 26 25 2h7.5L38 26h-6.6l-1.2-5.6h-7.6L19.8 26ZM24 15.2h5.2l-1.2-6Z" />
          </svg>
          <span>APEX</span>
        </Link>
        <p className="made-with-love">
          Made with <span className="love" role="img" aria-label="love">♥</span> · © {new Date().getFullYear()} APEX
        </p>
      </div>
    </footer>
  );
}
