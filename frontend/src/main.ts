import {createApp} from 'vue'
import App from './App.vue'
import { router } from './router';
import 'gitart-vue-dialog/dist/style.css';
import { GDialog } from 'gitart-vue-dialog';

createApp(App).use(router).component('GDialog', GDialog).mount('#app');