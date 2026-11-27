import { bool, choice, nullable, number, record, text } from './parse.ts';
export function parsePolicy(value: unknown) {
  const v = record(value);
  return {
    enabled: bool(v.enabled),
    carrier: choice(v.carrier, ['both', 'header']),
    ttl_seconds: number(v.ttl_seconds),
    revision: number(v.revision),
  };
}
export type Policy = ReturnType<typeof parsePolicy>;
export function parseKey(value: unknown) {
  const v = record(value),
    key = record(v.public_key);
  return {
    id: text(v.id),
    platform: choice(v.platform, ['CF', 'EO']),
    state: text(v.state),
    created_at: text(v.created_at),
    published_at: nullable(v.published_at),
    revoked_at: nullable(v.revoked_at),
    public_key: {
      kty: text(key.kty),
      crv: text(key.crv),
      x: text(key.x),
      y: text(key.y),
      kid: text(key.kid),
      alg: text(key.alg),
      use: text(key.use),
    },
  };
}
export type SigningKey = ReturnType<typeof parseKey>;
export function parseEvent(value: unknown) {
  const v = record(value);
  return {
    id: text(v.id),
    key_id: nullable(v.key_id),
    actor_id: nullable(v.actor_id),
    action: text(v.action),
    created_at: text(v.created_at),
  };
}
export type SigningEvent = ReturnType<typeof parseEvent>;
export function parseClaims(value: unknown) {
  const v = record(value);
  return {
    method: text(v.method),
    url: text(v.url),
    platform: choice(v.platform, ['CF', 'EO']),
    iat: number(v.iat),
    exp: number(v.exp),
    jti: text(v.jti),
  };
}
export function parseProof(value: unknown) {
  const v = record(value);
  return {
    status: choice(v.status, ['valid', 'invalid', 'expired', 'revoked', 'missing']),
    binding: choice(v.binding || 'unbound', ['unbound', 'matched', 'mismatch']),
    claims: v.claims == null ? null : parseClaims(v.claims),
  };
}
export type ProofCheck = ReturnType<typeof parseProof>;
