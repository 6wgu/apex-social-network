"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { IMAGE_ACCEPT, imageProblem } from "@/lib/images";
import {
  MAX_AGE,
  MIN_AGE,
  NICKNAME_TAKEN,
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
import PasswordRules from "@/components/PasswordRules";
import RacingLights from "@/components/RacingLights";
import LaunchOverlay, { GO_MS, MORPH_MS } from "@/components/LaunchOverlay";

const pause = (ms) => new Promise((done) => setTimeout(done, ms));

export default function RegisterPage() {
  const { setUser } = useAuth();
  const formRef = useRef(null);
  const [avatar, setAvatar] = useState(null);
  const [preview, setPreview] = useState("");
  const [password, setPassword] = useState("");
  const [nickname, setNickname] = useState("");
  const nicknameCheck = useNicknameCheck(nickname);
  const [errors, setErrors] = useState({});
  const [attempted, setAttempted] = useState(false);
  const [limits, setLimits] = useState(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [phase, setPhase] = useState(null);

  useEffect(() => setLimits(birthDateLimits()), []);

  useEffect(() => {
    if (!avatar) {
      setPreview("");
      return;
    }
    const url = URL.createObjectURL(avatar);
    setPreview(url);
    return () => URL.revokeObjectURL(url);
  }, [avatar]);

  function values() {
    const form = new FormData(formRef.current);
    return normalizeRegistration(Object.fromEntries(REGISTRATION_FIELDS.map((f) => [f, String(form.get(f) ?? "")])));
  }

  const setFieldError = (field, problem) => setErrors((old) => ({ ...old, [field]: problem }));

  function focusFirst(problems) {
    const field = [...REGISTRATION_FIELDS, "avatar"].find((f) => problems[f]);
    if (field && field !== "avatar") formRef.current.elements.namedItem(field)?.focus();
  }

  function onFieldChange(e) {
    const field = e.target.name;
    if (REGISTRATION_FIELDS.includes(field) && errors[field]) setFieldError(field, registrationProblem(field, values()));
  }

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
    if (!avatar) form.delete("avatar");
    const calm = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    if (!calm) setPhase("wait");
    try {
      const user = await api("/api/register", { method: "POST", body: form, redirectOn401: false });
      if (!calm) {
        setPhase("morph");
        await pause(MORPH_MS);
        setPhase("go");
        await pause(GO_MS);
      }
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
    const problem = file ? imageProblem(file) : "";
    if (problem) {
      e.target.value = "";
      setAvatar(null);
      setFieldError("avatar", `avatar ${problem}`);
      return;
    }
    setFieldError("avatar", "");
    setAvatar(file);
  }

  const shown = (field) => sentence(errors[field]);
  const invalid = (field) => (errors[field] ? true : undefined);
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
                <input name="avatar" type="file" accept={IMAGE_ACCEPT} onChange={chooseAvatar} className="visually-hidden" />
                {preview ? (
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
