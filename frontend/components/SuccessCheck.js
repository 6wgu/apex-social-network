export default function SuccessCheck() {
  return (
    <div className="verify-success" aria-hidden="true">
      <span className="verify-ring" />
      {Array.from({ length: 8 }, (_, i) => (
        <span key={i} className="verify-spark" style={{ "--i": i }} />
      ))}
      <svg className="verify-check" viewBox="0 0 64 64">
        <circle className="verify-check-circle" cx="32" cy="32" r="29" />
        <path className="verify-check-tick" d="M19 33.5 28 42.5 45.5 23" />
      </svg>
    </div>
  );
}
