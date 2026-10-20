"use client";

import { useEffect, useState } from "react";
import { SEARCH_DELAY_MS, searchQuery } from "@/lib/search";

// The query a search box stands for (see searchQuery), once typing pauses for
// SEARCH_DELAY_MS, so a search does not run on every keystroke. An empty box
// (or only spaces) counts at once: there is nothing to search for, and the
// full list comes back without a wait.
export default function useSearchQuery(text) {
  const query = searchQuery(text);
  const [settled, setSettled] = useState(query);

  useEffect(() => {
    if (!query) {
      setSettled("");
      return;
    }
    const timer = setTimeout(() => setSettled(query), SEARCH_DELAY_MS);
    return () => clearTimeout(timer);
  }, [query]);

  return query ? settled : "";
}
