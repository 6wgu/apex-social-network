"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import {
  MAX_AGE,
  MIN_AGE,
  NICKNAME_TAKEN,
  PASSWORD_RULES,
  REGISTRATION_FIELDS,
  birthDateLimits,
  normalizeRegistration,
  registrationProblem,
  registrationProblems,
  sentence,
} from "@/lib/validation";
import useNicknameCheck from "@/lib/useNicknameCheck";
import { useAuth } from "@/components/AuthProvider";
import Field, { PasswordField } from "@/components/Field";
import Icon from "@/components/Icon";
import RacingLights from "@/components/RacingLights";
import LaunchOverlay, { GO_MS, MORPH_MS } from "@/components/LaunchOverlay";

const AVATAR_ACCEPT = "image/jpeg,image/png,image/gif,image/webp,.jpg,.jpeg,.png,.gif,.webp";
const AVATAR_TYPES = new Set(["image/jpeg", "image/png", "image/gif", "image/webp"]);

const pause = (ms) => new Promise((done) => setTimeout(done, ms));

function isAllowedAvatar(file) {
  return file && AVATAR_TYPES.has(file.type) && /\.(jpe?g|png|gif|webp)$/i.test(file.name);
}

// The password rules under the password field. Each one turns green with a
// ✓ as soon as it is met; until then it is grey, or red once the user has
// tried to submit (strict).
function PasswordRules({ password, strict }) {
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

export default function RegisterPage() {
  const { setUser } = useAuth();
  const formRef = useRef(null);
  const [avatar, setAvatar] = useState(null);
  const [preview, setPreview] = useState("");
  const [password, setPassword] = useState(""); // a copy of the password input, for the checklist
  const [nickname, setNickname] = useState(""); // a copy of the nickname input, for the availability check
  const nicknameCheck = useNicknameCheck(nickname);
  const [errors, setErrors] = useState({}); // field -> problem, shown under that field
  const [attempted, setAttempted] = useState(false); // the user has pressed the button once
  const [limits, setLimits] = useState(null); // { min, max } for the date picker, from today's date
  const [error, setError] = useState(""); // a problem that belongs to no field
  const [busy, setBusy] = useState(false);
  const [phase, setPhase] = useState(null); // the launch animation, see LaunchOverlay

  useEffect(() => setLimits(birthDateLimits()), []);

  // Show the chosen picture before it is uploaded.
  useEffect(() => {
    if (!avatar) {
      setPreview("");
      return;
    }
    const url = URL.createObjectURL(avatar);
    setPreview(url);
    return () => URL.revokeObjectURL(url);
  }, [avatar]);

  // The form's text fields, trimmed and with the email lowercased.
  function values() {
    const form = new FormData(formRef.current);
    return normalizeRegistration(Object.fromEntries(REGISTRATION_FIELDS.map((f) => [f, String(form.get(f) ?? "")])));
  }

  const setFieldError = (field, problem) => setErrors((old) => ({ ...old, [field]: problem }));

  function focusFirst(problems) {
    const field = [...REGISTRATION_FIELDS, "avatar"].find((f) => problems[f]);
    if (field && field !== "avatar") formRef.current.elements.namedItem(field)?.focus();
  }

  // An error already shown follows the input, so it goes away as soon as
  // the field is fixed.
  function onFieldChange(e) {
    const field = e.target.name;
    if (REGISTRATION_FIELDS.includes(field) && errors[field]) setFieldError(field, registrationProblem(field, values()));
  }

  // Leaving a field shows what is wrong with it; an empty one waits for the
  // first submit.
  function onFieldBlur(e) {
    const field = e.target.name;
    if (!REGISTRATION_FIELDS.includes(field)) return;
    const v = values();
    if (v[field] || attempted) setFieldError(field, registrationProblem(field, v));
    if (field === "nickname") nicknameCheck.checkNow();
  }

  async function submit(e) {
    e.preventDefault();
    setError("");
    setAttempted(true);
    const v = values();
    const problems = registrationProblems(v);
    if (!problems.nickname && nicknameCheck.status === "taken") problems.nickname = NICKNAME_TAKEN;
    if (Object.keys(problems).length) {
      setErrors({ ...problems, avatar: errors.avatar });
      focusFirst(problems);
      return;
    }
    setErrors({});
    setBusy(true);
    const form = new FormData(formRef.current);
    for (const [field, value] of Object.entries(v)) form.set(field, value);
    if (!avatar) form.delete("avatar"); // an empty file input still sends a part
    const calm = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    if (!calm) setPhase("wait");
    try {
      // The server checks everything again; its problems come back per field.
      const user = await api("/api/register", { method: "POST", body: form, redirectOn401: false });
      if (!calm) {
        setPhase("morph");
        await pause(MORPH_MS);
        setPhase("go");
        await pause(GO_MS);
      }
      // Setting the user makes AuthGate send us on (home, or verification).
      setUser(user);
    } catch (err) {
      setPhase(null);
      if (err.fields) {
        setErrors(err.fields);
        focusFirst(err.fields);
      } else {
        setError(err.message);
      }
      setBusy(false);
    }
  }

  function chooseAvatar(e) {
    const file = e.target.files[0] || null;
    if (file && !isAllowedAvatar(file)) {
      e.target.value = "";
      setAvatar(null);
      setFieldError("avatar", "avatar must be a JPEG, PNG, GIF or WebP file");
      return;
    }
    setFieldError("avatar", "");
    setAvatar(file);
  }

  const shown = (field) => sentence(errors[field]);
  const invalid = (field) => (errors[field] ? true : undefined);
  // The nickname's own error comes first (format, or the server's 409);
  // otherwise the live check says whether it is taken or available.
  const nicknameError = shown("nickname") || (nicknameCheck.status === "taken" ? NICKNAME_TAKEN : "");
  const nicknameFree = !nicknameError && nicknameCheck.status === "available";

  return (
    <div className="auth-screen">
      <LaunchOverlay phase={phase} />
      <section className="card auth-card auth-card-wide">
        <div className="auth-heading">
          <h1>Create your account</h1>
          <p>Your corner of the network starts here.</p>
        </div>
        <form ref={formRef} onSubmit={submit} onChange={onFieldChange} onBlur={onFieldBlur} className="form auth-form" noValidate>
          <div className="form-row">
            <Field label="First name" icon="user" error={shown("first_name")}>
              <input name="first_name" autoComplete="given-name" placeholder="Alex" maxLength={50} aria-invalid={invalid("first_name")} required />
            </Field>
            <Field label="Last name" icon="user" error={shown("last_name")}>
              <input name="last_name" autoComplete="family-name" placeholder="Rivers" maxLength={50} aria-invalid={invalid("last_name")} required />
            </Field>
          </div>
          <Field label="Email address" icon="mail" error={shown("email")}>
            <input name="email" type="email" autoComplete="email" placeholder="alex@example.com" maxLength={254} aria-invalid={invalid("email")} required />
          </Field>
          <div className="form-row">
            <PasswordField
              name="password"
              autoComplete="new-password"
              placeholder="Strong password"
              onChange={(e) => setPassword(e.target.value)}
              aria-invalid={invalid("password")}
              error={shown("password")}
              below={<PasswordRules password={password} strict={attempted || Boolean(errors.password)} />}
              required
            />
            <Field
              label="Date of birth"
              icon="calendar"
              error={shown("date_of_birth")}
              hint={errors.date_of_birth ? null : `You must be ${MIN_AGE} to ${MAX_AGE} years old.`}
            >
              <input
                name="date_of_birth"
                type="date"
                min={limits?.min}
                max={limits?.max}
                aria-invalid={invalid("date_of_birth")}
                required
              />
            </Field>
          </div>

          <fieldset className="optional">
            <legend>
              Profile customization <span className="optional-tag">(Optional)</span>
            </legend>
            <div className="customize">
              <label className={`avatar-drop${preview ? " has-image" : ""}`} title="Choose an avatar">
                <input name="avatar" type="file" accept={AVATAR_ACCEPT} onChange={chooseAvatar} className="visually-hidden" />
                {preview ? (
                  // eslint-disable-next-line @next/next/no-img-element
                  <img src={preview} alt="Chosen avatar" />
                ) : (
                  <>
                    <Icon name="camera" size={20} />
                    <span>Avatar</span>
                  </>
                )}
              </label>
              <div className="customize-fields">
                <Field
                  icon="user"
                  hint="Start with a letter; letters, numbers, _ or - (up to 30)."
                  error={nicknameError}
                  below={nicknameFree && <span className="field-ok" role="status">✓ Available</span>}
                >
                  <input
                    name="nickname"
                    maxLength={30}
                    placeholder="Nickname (@handle)"
                    aria-label="Nickname"
                    onChange={(e) => setNickname(e.target.value)}
                    aria-invalid={nicknameError ? true : undefined}
                  />
                </Field>
                <Field icon="message" error={shown("about_me")}>
                  <textarea name="about_me" rows={1} maxLength={500} placeholder="Tell us about yourself…" aria-label="About me" aria-invalid={invalid("about_me")} />
                </Field>
              </div>
            </div>
            <p className="hint">Avatar: JPEG, PNG, GIF or WebP, up to 50 MB.</p>
            {errors.avatar && (
              <p className="form-error field-error" role="alert">
                {shown("avatar")}
              </p>
            )}
          </fieldset>

          {error && (
            <p className="form-error" role="alert">
              {error}
            </p>
          )}
          <button className="btn-primary btn-block" disabled={busy}>
            {busy ? (
              <>
                <RacingLights small /> Creating account…
              </>
            ) : (
              "Complete registration"
            )}
          </button>
        </form>
        <p className="muted auth-footer">
          Already have an account? <Link href="/login">Log in</Link>
        </p>
      </section>
    </div>
  );
}
