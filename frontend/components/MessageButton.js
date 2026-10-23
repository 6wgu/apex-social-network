"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";

export default function MessageButton({ user, reloadKey }) {
  const [allowed, setAllowed] = useState(null);

  useEffect(() => {
    api("/api/chat-contacts")
      .then((contacts) => setAllowed(contacts.some((c) => c.id === user.id)))
      .catch(() => setAllowed(false));
  }, [user.id, reloadKey]);

  if (allowed === null) return null;
  if (allowed) {
    return (
      <Link href={`/chats?user=${user.id}`} className="btn-secondary message-btn">
        💬 Message
      </Link>
    );
  }
  return (
    <p className="hint message-hint">
      💬 You can message {user.first_name} once one of you follows the other.
    </p>
  );
}
