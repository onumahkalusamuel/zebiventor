import { reactive } from 'vue';

type Keys = 'name' | 'address' | 'logo' | 'email' | 'phone' | 'logo_string' | 'logo_type';
type Data = { [key: string]: any; };

export const store = reactive({
  name: '',
  address: '',
  logo: '',
  email: '',
  phone: '',
  logo_string: '',
  logo_type: '',
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
  async getBase64 (e: Event) {
    const imageFile = ((e.target as any).files as FileList)[0];
    const imageType = imageFile.type.split('/')[1];
    if(!['png', 'jpg', 'jpeg'].includes(imageType)) {
      this.logo_string = '';
      this.logo_type = '';
      return;
    }

    const fr = new FileReader();
    fr.onload = (data) => {
      const imageData = (data.target as FileReader).result as string;
      imageData.split('base64,')[1]
      this.logo_string = imageData;
      // this.logo_string = imageData.split('base64,')[1];
      this.logo_type = imageType;
    };

    fr.readAsDataURL(imageFile);
  }
});