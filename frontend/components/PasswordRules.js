import { PASSWORD_RULES } from "@/lib/validation";

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
