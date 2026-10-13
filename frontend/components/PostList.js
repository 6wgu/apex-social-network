import PostCard from "@/components/PostCard";

// A list of PostCards with loading, empty, error and "Load more" states.
// feed is what usePagedPosts returns.
export default function PostList({ feed, empty }) {
  const { posts, hasMore, loading, error, loadMore } = feed;
  return (
    <div className="post-list">
      {posts && posts.map((p) => <PostCard key={p.id} post={p} />)}
      {posts && posts.length === 0 && !loading && <p className="muted center card">{empty}</p>}
      {error && <p className="form-error">{error}</p>}
      {loading && <p className="muted center">Loading posts…</p>}
      {hasMore && !loading && (
        <button className="btn-secondary load-more" onClick={loadMore}>
          Load more
        </button>
      )}
    </div>
  );
}
