import { saveProfile } from '../../api/account-profile/profile.browser.ts';
import type { ProfileAction } from '../../api/account-profile/profile.types.ts';
import type { SessionUser } from '../../api/auth/auth.types.ts';

import { displayNameIssue, usernameIssue } from './auth-fields.ts';
import { profileError } from './profile.messages.ts';

export function createProfileController(initial: SessionUser) {
  let user = $state(initial);
  let username = $state(initial.username);
  let displayName = $state(initial.display_name);
  let busy = $state(false);
  let confirming = $state(false);
  let error = $state('');
  let status = $state('');
  let uncertain = $state(false);
  let signIn = $state(false);
  let fieldErrors = $state<Record<string, string>>({});

  async function submit(kind: ProfileAction['kind']): Promise<boolean> {
    if (busy || uncertain) return false;
    error = '';
    status = '';
    fieldErrors = {};
    if (kind !== 'display') {
      const issue = usernameIssue(username);
      if (issue) fieldErrors.username = issue;
    }
    if (kind !== 'username') {
      const issue = displayNameIssue(displayName);
      if (issue) fieldErrors.display_name = issue;
    }
    if (Object.keys(fieldErrors).length) return false;
    if (kind === 'username' && !confirming) {
      confirming = true;
      return false;
    }
    busy = true;
    const action: ProfileAction =
      kind === 'confirm'
        ? { kind, username, display_name: displayName }
        : kind === 'display'
          ? { kind, display_name: displayName }
          : { kind, username };
    try {
      const result = await saveProfile(action);
      if (!result.ok) {
        uncertain = result.uncertain;
        signIn =
          result.code === 'unauthenticated' ||
          result.code === 'session_expired' ||
          result.code === 'invalid_token';
        const field =
          !result.uncertain &&
          (result.code === 'invalid_username' || result.code === 'username_taken'
            ? 'username'
            : result.code === 'invalid_display_name'
              ? 'display_name'
              : null);
        if (field) fieldErrors = { [field]: profileError(result.code, false) };
        else error = profileError(result.code, result.uncertain);
        confirming = false;
        return false;
      }
      user = result.user;
      username = user.username;
      displayName = user.display_name;
      confirming = false;
      status = '已保存';
      return true;
    } finally {
      busy = false;
    }
  }
  return {
    get user() {
      return user;
    },
    get username() {
      return username;
    },
    set username(value: string) {
      username = value;
      confirming = false;
    },
    get displayName() {
      return displayName;
    },
    set displayName(value: string) {
      displayName = value;
    },
    get busy() {
      return busy;
    },
    get confirming() {
      return confirming;
    },
    get error() {
      return error;
    },
    get fieldErrors() {
      return fieldErrors;
    },
    get status() {
      return status;
    },
    get uncertain() {
      return uncertain;
    },
    get signIn() {
      return signIn;
    },
    cancelConfirmation() {
      confirming = false;
    },
    submit,
  };
}
