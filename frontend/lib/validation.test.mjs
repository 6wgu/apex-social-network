import { test } from "node:test";
import assert from "node:assert/strict";

import {
  birthDateLimits,
  birthDateProblem,
  isValidEmail,
  nicknameProblem,
  normalizeRegistration,
  passwordProblem,
  registrationProblems,
  sentence,
} from "./validation.js";

test("isValidEmail", () => {
  const good = ["name@example.com", "123ahmed@gmail.com", "a.b_c%d+e-f@sub.example.co", `${"a".repeat(64)}@example.com`];
  const bad = [
    "",
    "plain",
    "a@b",
    "a@@example.com",
    "a@b@example.com",
    ".a@example.com",
    "a.@example.com",
    "a..b@example.com",
    `${"a".repeat(65)}@example.com`,
    "a@-example.com",
    "a@example-.com",
    "a@example..com",
    "a@example.c",
    "a@example.c0m",
    "a b@example.com",
    "ä@example.com",
    "Ada <a@example.com>",
  ];
  for (const e of good) assert.ok(isValidEmail(e), e);
  for (const e of bad) assert.ok(!isValidEmail(e), e);
});

test("passwordProblem", () => {
  const cases = {
    "": "password is required",
    "Correct-Horse1": "",
    "short1!A": "",
    alllowercase: "password needs an uppercase letter, a number and a special character",
    "Ab1!": "password needs at least 8 characters",
    NoSpecial123: "password needs a special character",
    ["Aa1!".repeat(19)]: "password must be 72 bytes or fewer",
  };
  for (const [pw, want] of Object.entries(cases)) assert.equal(passwordProblem(pw), want, pw);
});

test("birthDateProblem", () => {
  const now = new Date(2026, 2, 1, 15);
  const cases = {
    "": "date of birth is required",
    "2000-02-30": "date of birth must be a valid date (YYYY-MM-DD)",
    "01/01/2000": "date of birth must be a valid date (YYYY-MM-DD)",
    "2026-03-02": "date of birth can't be in the future",
    "2019-03-01": "",
    "2019-03-02": "you must be at least 7 years old",
    "1906-03-02": "",
    "1905-03-01": "you must be 120 years old or younger",
  };
  for (const [dob, want] of Object.entries(cases)) assert.equal(birthDateProblem(dob, now), want, dob);
});

test("birthDateLimits match birthDateProblem", () => {
  const now = new Date(2026, 2, 1, 15);
  const { min, max } = birthDateLimits(now);
  assert.deepEqual({ min, max }, { min: "1905-03-02", max: "2019-03-01" });
  assert.equal(birthDateProblem(min, now), "");
  assert.equal(birthDateProblem(max, now), "");
});

test("nicknameProblem", () => {
  const badChars = "nickname must start with a letter and contain only letters, numbers, underscores, or hyphens";
  const cases = {
    "": "",
    ada: "",
    "Ada_L-1": "",
    "1ada": badChars,
    _ada: badChars,
    "ada lovelace": badChars,
    "adä": badChars,
    ["a".repeat(30)]: "",
    ["a".repeat(31)]: "nickname must be 30 characters or fewer",
  };
  for (const [nick, want] of Object.entries(cases)) assert.equal(nicknameProblem(nick), want, nick);
});

test("normalizeRegistration trims everything but the password", () => {
  const out = normalizeRegistration({ email: "  Ada@Example.COM ", password: " pw ", first_name: " Ada " });
  assert.deepEqual(out, { email: "ada@example.com", password: " pw ", first_name: "Ada" });
});

test("registrationProblems lists every bad field", () => {
  const problems = registrationProblems(normalizeRegistration({
    first_name: "",
    last_name: "Lovelace",
    email: "nope",
    password: "Correct-Horse1",
    date_of_birth: "1990-01-01",
    nickname: "",
    about_me: "",
  }));
  assert.deepEqual(Object.keys(problems), ["first_name", "email"]);
});

test("sentence capitalizes a message", () => {
  assert.equal(sentence("email is required"), "Email is required");
  assert.equal(sentence(""), "");
});
