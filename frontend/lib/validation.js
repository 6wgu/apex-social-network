export const MIN_AGE = 7;
export const MAX_AGE = 120;

const MAX_EMAIL_LENGTH = 254;
const MAX_EMAIL_LOCAL = 64;
const MAX_EMAIL_LABEL = 63;
const MIN_PASSWORD_LENGTH = 8;
const MAX_PASSWORD_BYTES = 72;
const MAX_NAME_LENGTH = 50;
const MAX_NICKNAME_LENGTH = 30;
const MAX_ABOUT_LENGTH = 500;

const length = (s) => [...s].length;

export function isValidEmail(email) {
  if (length(email) > MAX_EMAIL_LENGTH) return false;
  const parts = email.split("@");
  if (parts.length !== 2) return false;
  const [local, domain] = parts;
  if (!local || local.length > MAX_EMAIL_LOCAL || !/^[a-z0-9._%+-]+$/i.test(local)) return false;
  if (local.startsWith(".") || local.endsWith(".") || local.includes("..")) return false;
  const labels = domain.split(".");
  if (labels.length < 2) return false;
  const goodLabel = (l) => l.length <= MAX_EMAIL_LABEL && /^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$/i.test(l);
  return labels.every(goodLabel) && /^[a-z]{2,}$/i.test(labels[labels.length - 1]);
}

export function emailProblem(email) {
  if (!email) return "email is required";
  if (length(email) > MAX_EMAIL_LENGTH) return `email must be ${MAX_EMAIL_LENGTH} characters or fewer`;
  if (!isValidEmail(email)) return "email must be a valid address like name@example.com";
  return "";
}

export const PASSWORD_RULES = [
  {
    label: `At least ${MIN_PASSWORD_LENGTH} characters`,
    missing: `at least ${MIN_PASSWORD_LENGTH} characters`,
    met: (p) => length(p) >= MIN_PASSWORD_LENGTH,
  },
  { label: "An uppercase letter", missing: "an uppercase letter", met: (p) => /\p{Lu}/u.test(p) },
  { label: "A lowercase letter", missing: "a lowercase letter", met: (p) => /\p{Ll}/u.test(p) },
  { label: "A number", missing: "a number", met: (p) => /\p{Nd}/u.test(p) },
  { label: "A special character (!@#$…)", missing: "a special character", met: (p) => /[\p{P}\p{S}]/u.test(p) },
];

function joinWithAnd(items) {
  return items.length === 1 ? items[0] : `${items.slice(0, -1).join(", ")} and ${items[items.length - 1]}`;
}

export function passwordProblem(password) {
  if (!password) return "password is required";
  const missing = PASSWORD_RULES.filter((rule) => !rule.met(password)).map((rule) => rule.missing);
  if (missing.length) return `password needs ${joinWithAnd(missing)}`;
  if (new TextEncoder().encode(password).length > MAX_PASSWORD_BYTES) {
    return `password must be ${MAX_PASSWORD_BYTES} bytes or fewer`;
  }
  return "";
}

const pad = (n) => String(n).padStart(2, "0");
const toISO = ({ y, m, d }) => `${String(y).padStart(4, "0")}-${pad(m)}-${pad(d)}`;
const daysInMonth = (y, m) => new Date(Date.UTC(y, m, 0)).getUTCDate();
const localToday = (now) => ({ y: now.getFullYear(), m: now.getMonth() + 1, d: now.getDate() });

function parseISODate(value) {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match) return null;
  const [y, m, d] = match.slice(1).map(Number);
  return m >= 1 && m <= 12 && d >= 1 && d <= daysInMonth(y, m) ? { y, m, d } : null;
}

const before = (a, b) => a.y < b.y || (a.y === b.y && (a.m < b.m || (a.m === b.m && a.d < b.d)));

function ageOn(born, on) {
  const notYet = on.m < born.m || (on.m === born.m && on.d < born.d);
  return on.y - born.y - (notYet ? 1 : 0);
}

function yearsBefore({ y, m, d }, years) {
  return { y: y - years, m, d: Math.min(d, daysInMonth(y - years, m)) };
}

function nextDay({ y, m, d }) {
  if (d < daysInMonth(y, m)) return { y, m, d: d + 1 };
  return m < 12 ? { y, m: m + 1, d: 1 } : { y: y + 1, m: 1, d: 1 };
}

export function birthDateLimits(now = new Date()) {
  const today = localToday(now);
  return { min: toISO(nextDay(yearsBefore(today, MAX_AGE + 1))), max: toISO(yearsBefore(today, MIN_AGE)) };
}

export function birthDateProblem(value, now = new Date()) {
  if (!value) return "date of birth is required";
  const born = parseISODate(value);
  if (!born) return "date of birth must be a valid date (YYYY-MM-DD)";
  const today = localToday(now);
  if (before(today, born)) return "date of birth can't be in the future";
  const age = ageOn(born, today);
  if (age < MIN_AGE) return `you must be at least ${MIN_AGE} years old`;
  if (age > MAX_AGE) return `you must be ${MAX_AGE} years old or younger`;
  return "";
}

function nameProblem(name, label) {
  if (!name) return `${label} is required`;
  if (length(name) > MAX_NAME_LENGTH) return `${label} must be ${MAX_NAME_LENGTH} characters or fewer`;
  return "";
}

export const NICKNAME_TAKEN = "This nickname is already taken";

export function nicknameProblem(nickname) {
  if (length(nickname) > MAX_NICKNAME_LENGTH) return `nickname must be ${MAX_NICKNAME_LENGTH} characters or fewer`;
  if (nickname && !/^[A-Za-z][A-Za-z0-9_-]*$/.test(nickname)) {
    return "nickname must start with a letter and contain only letters, numbers, underscores, or hyphens";
  }
  return "";
}

function aboutProblem(about) {
  return length(about) > MAX_ABOUT_LENGTH ? `about me must be ${MAX_ABOUT_LENGTH} characters or fewer` : "";
}

export function normalizeRegistration(values) {
  const out = {};
  for (const [key, value] of Object.entries(values)) out[key] = key === "password" ? value : value.trim();
  out.email = (out.email || "").toLowerCase();
  return out;
}

export function registrationProblem(field, values) {
  const value = values[field] ?? "";
  switch (field) {
    case "first_name":
      return nameProblem(value, "first name");
    case "last_name":
      return nameProblem(value, "last name");
    case "email":
      return emailProblem(value);
    case "password":
      return passwordProblem(value);
    case "date_of_birth":
      return birthDateProblem(value);
    case "nickname":
      return nicknameProblem(value);
    case "about_me":
      return aboutProblem(value);
    default:
      return "";
  }
}

export const REGISTRATION_FIELDS = ["first_name", "last_name", "email", "password", "date_of_birth", "nickname", "about_me"];

export function registrationProblems(values) {
  const problems = {};
  for (const field of REGISTRATION_FIELDS) {
    const problem = registrationProblem(field, values);
    if (problem) problems[field] = problem;
  }
  return problems;
}

export function sentence(message) {
  return message ? message[0].toUpperCase() + message.slice(1) : message;
}
