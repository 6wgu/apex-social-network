"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";

const PAGE_SIZE = 20; // the backend's page size for /api/feed and /api/users/{id}/posts

// Loads a newest-first list of posts page by page (?before={last id}).
// url is the list endpoint; pass null to load nothing (e.g. a private profile).
export default function usePagedPosts(url) {
  const [posts, setPosts] = useState(null);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const current = useRef(url); // ignores answers for a url we already left

  const fetchPage = useCallback(
    async (before) => {
      if (!url) return;
      setLoading(true);
      setError("");
      try {
        const sep = url.includes("?") ? "&" : "?";
        const page = await api(before ? `${url}${sep}before=${before}` : url);
        if (current.current !== url) return;
        setPosts((old) => (before && old ? [...old, ...page] : page));
        setHasMore(page.length === PAGE_SIZE);
      } catch (err) {
        if (current.current === url) setError(err.status === 403 ? "These posts are private." : err.message);
      } finally {
        if (current.current === url) setLoading(false);
      }
    },
    [url]
  );

  useEffect(() => {
    current.current = url;
    setPosts(null);
    setHasMore(false);
    fetchPage();
  }, [url, fetchPage]);

  // Fetches the newest page again and puts any posts newer than the ones
  // shown at the top. Resolves with how many were added.
  const postsRef = useRef(posts);
  postsRef.current = posts;
  const refreshTop = useCallback(async () => {
    if (!url || !postsRef.current) return 0;
    try {
      const page = await api(url);
      const shown = postsRef.current;
      if (current.current !== url || !shown) return 0;
      const newest = shown[0]?.id ?? 0;
      const fresh = page.filter((p) => p.id > newest);
      if (fresh.length) setPosts((old) => [...fresh, ...(old || [])]);
      return fresh.length;
    } catch {
      return 0; // offline for a moment: the next round tries again
    }
  }, [url]);

  const loadMore = () => posts?.length && fetchPage(posts[posts.length - 1].id);
  const prepend = (post) => setPosts((old) => [post, ...(old || [])]);
  const replace = (post) => setPosts((old) => old.map((p) => (p.id === post.id ? post : p)));

  return { posts, hasMore, loading, error, loadMore, prepend, replace, refreshTop, reload: () => fetchPage() };
}
