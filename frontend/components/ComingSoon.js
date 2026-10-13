// Placeholder for a section that a later phase builds, so navbar links never
// lead to a 404.
export default function ComingSoon({ icon, title, text }) {
  return (
    <section className="card private-notice">
      <span className="lock" aria-hidden="true">
        {icon}
      </span>
      <h2>{title}</h2>
      <p className="muted">{text}</p>
    </section>
  );
}
