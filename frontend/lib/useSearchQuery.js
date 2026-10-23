"use client";

import { useEffect, useState } from "react";
import { SEARCH_DELAY_MS, searchQuery } from "@/lib/search";

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
