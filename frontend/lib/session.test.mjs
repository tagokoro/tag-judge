import { test } from "node:test";
import assert from "node:assert/strict";
import { actorOf, canUseServerData, devHeadersOf, identityOf, isOwner, SEED_USERS } from "./session.ts";

const devUser = SEED_USERS[0];
const base = { session: null, cookieChecked: true, devEnabled: false, devUser, devMe: null };

test("anonymous without a session or dev auth", () => {
  assert.equal(actorOf(base), null);
  assert.equal(identityOf(base), "anon");
  assert.equal(devHeadersOf(base), null);
});

test("dev headers are sent only when dev auth is enabled", () => {
  const state = { ...base, devEnabled: true };
  assert.deepEqual(actorOf(state), {
    userId: devUser.userId,
    role: devUser.role,
    source: "dev",
    handle: null,
  });
  assert.deepEqual(devHeadersOf(state), { "X-User-ID": devUser.userId, "X-User-Role": devUser.role });
});

test("a dev user's handle comes from their own /me answer only", () => {
  const devMe = { user_id: devUser.userId, role: devUser.role, handle: "seed-author" };
  assert.equal(actorOf({ ...base, devEnabled: true, devMe }).handle, "seed-author");
  assert.equal(actorOf({ ...base, devEnabled: true, devMe, devUser: SEED_USERS[1] }).handle, null);
});

test("a cookie session wins over dev auth and sends no dev headers", () => {
  const session = { user_id: "11111111-1111-4111-8111-111111111111", role: "user", handle: "alice" };
  const state = { ...base, session, devEnabled: true };
  assert.equal(actorOf(state).source, "cookie");
  assert.equal(actorOf(state).handle, "alice");
  assert.equal(devHeadersOf(state), null);
  assert.equal(identityOf(state), `cookie:${session.user_id}:user`);
});

test("identity changes with the dev user, so pages remount", () => {
  const author = identityOf({ ...base, devEnabled: true });
  const admin = identityOf({ ...base, devEnabled: true, devUser: SEED_USERS[1] });
  assert.notEqual(author, admin);
});

test("isOwner compares handles and is false without one", () => {
  const session = { user_id: "11111111-1111-4111-8111-111111111111", role: "user", handle: "alice" };
  const actor = actorOf({ ...base, session });
  assert.equal(isOwner(actor, "alice"), true);
  assert.equal(isOwner(actor, "bob"), false);
  assert.equal(isOwner(actorOf({ ...base, devEnabled: true }), "alice"), false);
  assert.equal(isOwner(null, "alice"), false);
});

test("server-rendered data counts only for the actor the server rendered for", () => {
  const session = { user_id: "11111111-1111-4111-8111-111111111111", role: "user", handle: "alice" };
  const signedIn = { ...base, session };
  const serverIdentity = identityOf(signedIn);
  // Before the first session check, and for the same actor afterwards.
  assert.equal(canUseServerData({ ...base, cookieChecked: false }, null), true);
  assert.equal(canUseServerData(signedIn, serverIdentity), true);
  // A logout on the page must not keep showing what the owner could see.
  assert.equal(canUseServerData(base, serverIdentity), false);
  // A login on a page rendered for an anonymous visitor fetches again too.
  assert.equal(canUseServerData(signedIn, identityOf(base)), false);
  // The server never sees dev headers.
  assert.equal(canUseServerData({ ...base, devEnabled: true }, identityOf(base)), false);
});
