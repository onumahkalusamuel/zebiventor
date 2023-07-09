import { reactive } from 'vue';

export const setup = reactive({
    licenseActivated: false,
    adminCreated: false,
    storeSetUp: false,
    // set(jwt: string) { this.jwt = jwt; sessionStorage.setItem('auth_jwt', jwt); },
    // get() {
    //     if (!this.jwt && sessionStorage.getItem('auth_jwt')) {
    //         this.jwt = sessionStorage.getItem('auth_jwt') as string;
    //     }
    //     return this.jwt;
    // },
});