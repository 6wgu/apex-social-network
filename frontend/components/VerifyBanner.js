"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useAuth } from "@/components/AuthProvider";

export default function VerifyBanner() {
  const { user } = useAuth();
  const pathname = usePathname();

  if (!user || user.is_verified || pathname === "/verify") return null;

  return (
    <div className="verify-banner" role="status">
      <span>Please verify your email address ({user.email}).</span>
      <Link href="/verify" className="btn-primary">
        Verify now
      </Link>
    </div>
  );
}
