import { randomUUID } from 'node:crypto';

import type { ManagementPermissionKey } from '@zhblogs/db';

import type { FastifyReply, FastifyRequest } from 'fastify';

import { signJwt, verifyJwt } from '@/domain/auth/service/auth-token.service';
import type { UserRow } from '@/domain/auth/service/auth-user.service';
import { buildAuthUser } from '@/domain/auth/service/auth-user.service';
import {
  AuthError,
  type AuthUser,
  type RefreshSessionRecord,
} from '@/domain/auth/types/auth.types';
import {
  readCacheJson,
  removeCacheKey,
  writeCacheJson,
} from '@/infrastructure/auth/cache/auth-session.store';
import type { CacheClient } from '@/shared/runtime/types/app-dependencies.types';

export const ACCESS_COOKIE_NAME = 'zhblogs_access_token';
export const REFRESH_COOKIE_NAME = 'zhblogs_refresh_token';
const REFRESH_SESSION_PREFIX = 'zhblogs:auth:refresh-session:';

const sessionKey = (sessionId: string): string => `${REFRESH_SESSION_PREFIX}${sessionId}`;

export type VerifyPayload = ReturnType<typeof verifyJwt>;

export type SessionDeps = {
  cache: CacheClient | undefined;
  jwt: {
    accessSecret: string;
    refreshSecret: string;
    accessTtlSeconds: number;
    refreshTtlSeconds: number;
  };
  cookieBaseOptions: {
    httpOnly: true;
    sameSite: 'lax';
    secure: boolean;
    path: '/';
    domain?: string;
  };
  readUserById: (userId: string) => Promise<UserRow | null>;
  readUserPermissions: (userId: string) => Promise<ManagementPermissionKey[]>;
  readUserHasGithub: (userId: string) => Promise<boolean>;
};

export const readCookiePayload = (
  request: FastifyRequest,
  cookieName: string,
  secret: string,
  tokenType: 'access' | 'refresh',
): VerifyPayload => {
  const token = request.cookies[cookieName];

  if (!token) {
    throw new AuthError('missing_token', 'Login required');
  }

  return verifyJwt(token, secret, tokenType);
};

export const createSessionHelpers = (deps: SessionDeps) => {
  const readRefreshSession = async (
    currentSessionId: string,
  ): Promise<RefreshSessionRecord | null> => {
    const session = await readCacheJson<RefreshSessionRecord>(
      deps.cache,
      sessionKey(currentSessionId),
    );

    if (!session) {
      return null;
    }

    if (session.expiresAt <= Date.now()) {
      await removeCacheKey(deps.cache, sessionKey(currentSessionId));
      return null;
    }

    return session;
  };

  const writeRefreshSession = async (
    currentSessionId: string,
    payload: RefreshSessionRecord,
  ): Promise<void> => {
    await writeCacheJson(
      deps.cache,
      sessionKey(currentSessionId),
      payload,
      deps.jwt.refreshTtlSeconds,
    );
  };

  const clearSession = async (reply: FastifyReply, currentSessionId?: string): Promise<void> => {
    if (currentSessionId) {
      await removeCacheKey(deps.cache, sessionKey(currentSessionId));
    }

    reply.clearCookie(ACCESS_COOKIE_NAME, deps.cookieBaseOptions);
    reply.clearCookie(REFRESH_COOKIE_NAME, deps.cookieBaseOptions);
  };

  const setSessionCookies = (
    reply: FastifyReply,
    accessToken: string,
    refreshToken: string,
  ): void => {
    reply.setCookie(ACCESS_COOKIE_NAME, accessToken, {
      ...deps.cookieBaseOptions,
      maxAge: deps.jwt.accessTtlSeconds,
    });
    reply.setCookie(REFRESH_COOKIE_NAME, refreshToken, {
      ...deps.cookieBaseOptions,
      maxAge: deps.jwt.refreshTtlSeconds,
    });
  };

  const validateAuthenticatedUser = async (payload: VerifyPayload): Promise<AuthUser> => {
    const session = await readRefreshSession(payload.sessionId);

    if (!session) {
      throw new AuthError('session_expired', 'Login session expired');
    }

    const userRecord = await deps.readUserById(payload.sub);

    if (!userRecord || !userRecord.is_active) {
      throw new AuthError('user_not_found', 'User is unavailable');
    }

    const permissions = await deps.readUserPermissions(userRecord.id);
    const hasGithub = await deps.readUserHasGithub(userRecord.id);
    const user = buildAuthUser(userRecord, permissions, hasGithub);

    if (
      session.userId !== user.id ||
      session.authVersion !== user.authVersion ||
      payload.authVersion !== user.authVersion ||
      payload.role !== user.role
    ) {
      throw new AuthError('session_invalidated', 'Login session invalidated');
    }

    return user;
  };

  const createTokensForUser = async (
    user: AuthUser,
    currentSessionId?: string,
  ): Promise<{ accessToken: string; refreshToken: string; sessionId: string }> => {
    const nextSessionId = currentSessionId ?? randomUUID();

    await writeRefreshSession(nextSessionId, {
      userId: user.id,
      authVersion: user.authVersion,
      expiresAt: Date.now() + deps.jwt.refreshTtlSeconds * 1_000,
    });

    const accessToken = signJwt({
      subject: user.id,
      role: user.role,
      authVersion: user.authVersion,
      sessionId: nextSessionId,
      tokenType: 'access',
      secret: deps.jwt.accessSecret,
      ttlSeconds: deps.jwt.accessTtlSeconds,
    });
    const refreshToken = signJwt({
      subject: user.id,
      role: user.role,
      authVersion: user.authVersion,
      sessionId: nextSessionId,
      tokenType: 'refresh',
      secret: deps.jwt.refreshSecret,
      ttlSeconds: deps.jwt.refreshTtlSeconds,
    });

    return {
      accessToken,
      refreshToken,
      sessionId: nextSessionId,
    };
  };

  return {
    clearSession,
    createTokensForUser,
    setSessionCookies,
    validateAuthenticatedUser,
  };
};
