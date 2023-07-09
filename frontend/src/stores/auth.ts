import { reactive } from 'vue';

export const auth = reactive({
  jwt: '',
  setJwt (jwt: string) { this.jwt = jwt; sessionStorage.setItem('auth_jwt', jwt); },
  getJwt () {
    if (!this.jwt && sessionStorage.getItem('auth_jwt')){
      this.jwt = sessionStorage.getItem('auth_jwt') as string;
    }
    return this.jwt;
  },
});