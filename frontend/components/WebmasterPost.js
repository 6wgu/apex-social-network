import Icon from "@/components/Icon";

// The webmaster's first post and its replies, hard-coded (with its images in
// public/landing/) so every copy of the app shows it: on the landing page and
// pinned at the top of the feed, whatever is in the database.
const DEV = { name: "Dev", avatar: "/landing/dev.webp" };
const ABS = { name: "abs", avatar: "/landing/abs.png" };
const REPLIES = [
  { ...DEV, text: "I'm a developer :(", image: "/landing/dev-reply.png" },
  { ...ABS, text: "Bro be quiet 😂😂😂" },
  { ...DEV, image: "/landing/dev-reply-2.jpg" },
];

// The real post, if this database has it, so the feed does not show it twice.
export const isWebmasterPost = (p) => p.author?.nickname === "abs" && p.content === "Im the webmaster";

/* eslint-disable @next/next/no-img-element */
export default function WebmasterPost({ pinned = false }) {
  return (
    <article className={pinned ? "card post wm-post wm-pinned" : "wm-post"}>
      <header className="wm-head">
        <img className="wm-avatar" src="/landing/abs.png" alt="" width={40} height={40} />
        <div>
          <strong>Abbas Al Abdulwahab</strong>
          <span>@abs · webmaster</span>
        </div>
        {pinned && (
          <span className="wm-pin">
            <Icon name="pin" size={13} /> Pinned
          </span>
        )}
      </header>
      <p className="wm-text">Im the webmaster</p>
      <img className="wm-image" src="/landing/webmaster.gif" alt="The webmaster's GIF" width={200} height={336} loading="lazy" />
      <footer className="wm-foot">
        <span className="wm-like">
          <Icon name="heart" size={16} /> 2
        </span>
        <span>
          <Icon name="comment" size={16} /> 3
        </span>
      </footer>
      <ul className="wm-replies">
        {REPLIES.map((r, i) => (
          <li key={i} className="wm-reply">
            <img className="wm-avatar wm-avatar-sm" src={r.avatar} alt="" width={28} height={28} />
            <div className="wm-bubble">
              <strong>{r.name}</strong>
              {r.text && <p>{r.text}</p>}
              {r.image && <img src={r.image} alt={`${r.name}'s reply`} loading="lazy" />}
            </div>
          </li>
        ))}
      </ul>
    </article>
  );
}
