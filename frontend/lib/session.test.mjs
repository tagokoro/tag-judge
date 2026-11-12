import { test } from "node:test";
import assert from "node:assert/strict";
import { actorOf, devHeadersOf, identityOf, isOwner, SEED_USERS } from "./session.ts";

const devUser = SEED_USERS[0];
const base = { session: null, cookieChecked: true, devEnabled: false, devUser };

test("anonymous without a session or dev auth", () => {
  assert.equal(actorOf(base), null);
  assert.equal(identityOf(base), "anon");
  assert.equal(devHeadersOf(base), null);
});

test("dev headers are sent only when dev auth is enabled", () => {
  const state = { ...base, devEnabled: true };
  assert.deepEqual(actorOf(state), { userId: devUser.userId, role: devUser.role, source: "dev" });
  assert.deepEqual(devHeadersOf(state), { "X-User-ID": devUser.userId, "X-User-Role": devUser.role });
});

test("a cookie session wins over dev auth and sends no dev headers", () => {
  const session = { user_id: "11111111-1111-4111-8111-111111111111", role: "user" };
  const state = { ...base, session, devEnabled: true };
  assert.equal(actorOf(state).source, "cookie");
  assert.equal(devHeadersOf(state), null);
  assert.equal(identityOf(state), `cookie:${session.user_id}:user`);
});

test("identity changes with the dev user, so pages remount", () => {
  const author = identityOf({ ...base, devEnabled: true });
  const admin = identityOf({ ...base, devEnabled: true, devUser: SEED_USERS[1] });
  assert.notEqual(author, admin);
});

test("isOwner compares user ids case-insensitively and is false when anonymous", () => {
  const actor = actorOf({ ...base, devEnabled: true });
  assert.equal(isOwner(actor, devUser.userId.toUpperCase()), true);
  assert.equal(isOwner(actor, SEED_USERS[1].userId), false);
  assert.equal(isOwner(null, devUser.userId), false);
});
