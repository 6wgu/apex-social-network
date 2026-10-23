"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";

const PAGE_SIZE = 20;

export default function usePagedPosts(url) {
  const [posts, setPosts] = useState(null);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const current = useRef(url);

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
      return 0;
    }
  }, [url]);

  const loadMore = () => posts?.length && fetchPage(posts[posts.length - 1].id);
  const prepend = (post) => setPosts((old) => [post, ...(old || [])]);
  const replace = (post) => setPosts((old) => old.map((p) => (p.id === post.id ? post : p)));

  return { posts, hasMore, loading, error, loadMore, prepend, replace, refreshTop, reload: () => fetchPage() };
}
