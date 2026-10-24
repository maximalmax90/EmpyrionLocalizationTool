

import { t } from './i18n.svelte.js';
import { api } from './api.js';

function defaultName() {

  return t('admin.title');
}

let authenticatedUser = $state(null);

export function currentOwnerName() {
  return authenticatedUser?.name || defaultName();
}

export function isAdminUser() {
  return authenticatedUser?.isAdmin ?? false;
}

export function isSignedIn() {
  return authenticatedUser !== null;
}

export function canManageProject() {
  return !isSignedIn() || isAdminUser();
}

export async function refreshAuthenticatedIdentity() {
  try {
    const me = await api.get('/me');
    authenticatedUser = me.authenticated ? { name: me.name, isAdmin: !!me.is_admin } : null;
  } catch {

  }
}

