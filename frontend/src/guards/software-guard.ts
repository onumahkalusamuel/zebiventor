import { router } from '../router';
import { setup } from '../stores';

import { CheckActivation } from '../../wailsjs/go/controllers/General'

export async function softwareGuard() {
    // early escape
    if (setup.licenseActivated && setup.adminCreated && setup.storeSetUp) return;
    const check = await CheckActivation();
    if(check === 1) router.push({ name: 'activate' });
    setup.licenseActivated = true;
    if(check === 2) router.push({ name: 'create-admin' });
    setup.adminCreated = true;
    if(check === 3) router.push({ name: 'store-setup' });
    setup.storeSetUp = true;
}