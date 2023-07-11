import { reactive } from 'vue';

type Keys = 'name' | 'address' | 'logo' | 'email' | 'phone';
type Data = { [key: string]: any; };

export const store = reactive({
  name: '',
  address: '',
  logo: '',
  email: '',
  phone: '',
  setAll (data: Data) {
    (Object.keys(data) as Keys[]).forEach((key) => {
      this[key] = data[key];
    });
  },
  set (key: Keys, value: any) { this[key] = value; sessionStorage.setItem(`${key}`, value); },
  get (key: Keys) {
    if (!this[key] && sessionStorage.getItem(`${key}`)) {
      this[key] = sessionStorage.getItem(`${key}`) as any;
    }
    return this[key];
  },
});