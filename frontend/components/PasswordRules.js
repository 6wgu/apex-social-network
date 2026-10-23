import { PASSWORD_RULES } from "@/lib/validation";

// The password rules under a new password field. Each one turns green with
// a ✓ as soon as it is met; until then it is grey, or red once the user has
// tried to submit (strict).
export default function PasswordRules({ password, strict }) {
  return (
    <span className="password-rules" role="list" aria-label="Password rules">
      {PASSWORD_RULES.map((rule) => {
        const met = rule.met(password);
        return (
          <span key={rule.label} role="listitem" className={`password-rule${met ? " is-met" : strict ? " is-missing" : ""}`}>
            <span className="password-rule-mark" aria-hidden="true">
              {met ? "✓" : strict ? "✕" : "•"}
            </span>
            {rule.label}
            <span className="visually-hidden">{met ? " (done)" : " (not yet)"}</span>
          </span>
        );
      })}
    </span>
  );
}
