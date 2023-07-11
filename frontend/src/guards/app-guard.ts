import { router } from '../router';
import { auth } from '../stores';

import { CheckLogin } from '../../wailsjs/go/controllers/General';

export async function appGuard () {
  // early escape
  if (auth.id && auth.name && auth.role && Date.now() < auth.timestamp + 1800) {
    auth.updateTimestamp();
    return;
  }

  const check = await CheckLogin();
  if (check.id) {
    auth.id = check.id;
    auth.name = check.name;
    auth.role = check.role;
    auth.username = check.username;
    auth.updateTimestamp();
  } 

  if (Date.now() > auth.timestamp + 1800 || auth.id === '') {
    auth.reset();
    router.push({ name: 'login' });
  }
}