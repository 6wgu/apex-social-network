// The register form's rules, the same ones the backend enforces
// (backend/pkg/handlers/validation.go), with the same messages. The server
// checks everything again; these let the page point at a problem before the
// form is sent.

// Age limits for the date of birth, in whole years. A birthday not reached
// yet this year does not count. Change them here and MinAge / MaxAge in the
// backend together.
export const MIN_AGE = 7;
export const MAX_AGE = 120;

const MAX_EMAIL_LENGTH = 254;
const MAX_EMAIL_LOCAL = 64;
const MAX_EMAIL_LABEL = 63;
const MIN_PASSWORD_LENGTH = 8;
const MAX_PASSWORD_BYTES = 72; // bcrypt ignores the rest
const MAX_NAME_LENGTH = 50;
const MAX_NICKNAME_LENGTH = 30;
const MAX_ABOUT_LENGTH = 500;

// Characters, counted the way the backend counts them (code points).
const length = (s) => [...s].length;

// A plain address like name@example.com (lowercase it first):
//   - at most 254 characters, and exactly one "@"
//   - before the "@": 1 to 64 letters, digits or . _ % + -, no dot at the
//     start or end, no two dots in a row. A leading digit is fine
//     (123ahmed@gmail.com).
//   - after the "@": two or more labels joined by single dots, each 1 to 63
//     letters, digits or hyphens, not starting or ending with a hyphen
//   - the last label (the TLD) is 2 or more letters
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

// The password rules, in the order the checklist shows them. `missing` is
// how a problem message names a rule that is not met.
export const PASSWORD_RULES = [
  {
    label: `At least ${MIN_PASSWORD_LENGTH} characters`,
    missing: `at least ${MIN_PASSWORD_LENGTH} characters`,
    met: (p) => length(p) >= MIN_PASSWORD_LENGTH,
  },
  { label: "An uppercase letter", missing: "an uppercase letter", met: (p) => /\p{Lu}/u.test(p) },
  { label: "A lowercase letter", missing: "a lowercase letter", met: (p) => /\p{Ll}/u.test(p) },
  { label: "A number", missing: "a number", met: (p) => /\p{Nd}/u.test(p) },
  // Punctuation and symbols: ! @ # $ % ^ & * - _ + = ? and the like.
  { label: "A special character (!@#$…)", missing: "a special character", met: (p) => /[\p{P}\p{S}]/u.test(p) },
];

// "a", "a and b", "a, b and c"
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

// Calendar dates are { y, m, d } (m is 1-12), so no time zone can shift them.
const pad = (n) => String(n).padStart(2, "0");
const toISO = ({ y, m, d }) => `${String(y).padStart(4, "0")}-${pad(m)}-${pad(d)}`;
const daysInMonth = (y, m) => new Date(Date.UTC(y, m, 0)).getUTCDate();
const localToday = (now) => ({ y: now.getFullYear(), m: now.getMonth() + 1, d: now.getDate() });

// "YYYY-MM-DD" -> { y, m, d }, or null when it is not a real date.
function parseISODate(value) {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match) return null;
  const [y, m, d] = match.slice(1).map(Number);
  return m >= 1 && m <= 12 && d >= 1 && d <= daysInMonth(y, m) ? { y, m, d } : null;
}

const before = (a, b) => a.y < b.y || (a.y === b.y && (a.m < b.m || (a.m === b.m && a.d < b.d)));

// Age in whole years on the day `on`: one less while that year's birthday
// has not come yet (someone born on 29 February is a year older on 1 March).
function ageOn(born, on) {
  const notYet = on.m < born.m || (on.m === born.m && on.d < born.d);
  return on.y - born.y - (notYet ? 1 : 0);
}

// The same day `years` years earlier; 29 February becomes the 28th.
function yearsBefore({ y, m, d }, years) {
  return { y: y - years, m, d: Math.min(d, daysInMonth(y - years, m)) };
}

function nextDay({ y, m, d }) {
  if (d < daysInMonth(y, m)) return { y, m, d: d + 1 };
  return m < 12 ? { y, m: m + 1, d: 1 } : { y: y + 1, m: 1, d: 1 };
}

// The oldest and newest birth dates allowed today, as YYYY-MM-DD, for the
// date input's min and max: the day after someone would turn MAX_AGE + 1,
// and the day someone turns MIN_AGE.
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

// The 409 message for a nickname someone else has (case does not matter).
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

// Trims the text fields and lowercases the email, as the backend does. The
// password is kept exactly as typed.
export function normalizeRegistration(values) {
  const out = {};
  for (const [key, value] of Object.entries(values)) out[key] = key === "password" ? value : value.trim();
  out.email = (out.email || "").toLowerCase();
  return out;
}

// Checks one field of a normalized registration; "" when it is fine.
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

// Every problem of a normalized registration, as { field: message }.
export function registrationProblems(values) {
  const problems = {};
  for (const field of REGISTRATION_FIELDS) {
    const problem = registrationProblem(field, values);
    if (problem) problems[field] = problem;
  }
  return problems;
}

// Messages are lowercase like the API's; this capitalizes one for display.
export function sentence(message) {
  return message ? message[0].toUpperCase() + message.slice(1) : message;
}
