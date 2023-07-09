import { router } from '../router';
import { auth } from '../stores/auth';

export function appGuard () {
  if (!auth.getJwt()) router.push({ name: 'login' });
}