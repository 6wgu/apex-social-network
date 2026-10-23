import Icon from "@/components/Icon";

// A sample post for the landing page's preview, where visitors are not
// logged in and cannot load real posts. It mirrors one of the demo posts
// (Maya's sunrise run); its images are in public/landing/.
const REPLIES = [
  { name: "Leo Hart", avatar: "/landing/sample-leo.jpg", text: "Great colours! 😍" },
  { name: "Iris Cole", avatar: "/landing/sample-iris.png", text: "Next time I'm in 🏃‍♀️" },
];

/* eslint-disable @next/next/no-img-element */
export default function SamplePost() {
  return (
    <article className="sample-post">
      <header className="sample-head">
        <img className="sample-avatar" src="/landing/sample-maya.png" alt="" width={40} height={40} />
        <div>
          <strong>Maya Lind</strong>
          <span>@maya · public</span>
        </div>
      </header>
      <p className="sample-text">Sunrise loop done before work 🌅 7 km and happy legs.</p>
      <img className="sample-image" src="/landing/sample-sunrise.jpg" alt="A sunrise over hills" width={480} height={300} loading="lazy" />
      <footer className="sample-foot">
        <span className="sample-like">
          <Icon name="heart" size={16} /> 4
        </span>
        <span>
          <Icon name="comment" size={16} /> 3
        </span>
      </footer>
      <ul className="sample-replies">
        {REPLIES.map((r) => (
          <li key={r.name} className="sample-reply">
            <img className="sample-avatar sample-avatar-sm" src={r.avatar} alt="" width={28} height={28} />
            <div className="sample-bubble">
              <strong>{r.name}</strong>
              <p>{r.text}</p>
            </div>
          </li>
        ))}
      </ul>
    </article>
  );
}
