<script lang="ts" setup>
import PrimaryButton from '../components/form/PrimaryButton.vue';
import TextField from '../components/form/TextField.vue';
import { toasts } from '../stores/toasts';
import { Login, StoreDetails } from '../../wailsjs/go/controllers/General'
import { onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';

const loginForm = ref({ username: '', password: '' })
const storeDetails = ref({ name: '', address: '', logo: '' });
const router = useRouter();

onMounted(async () => {
  try {
    const store = await StoreDetails();
    console.log(store);
    if (store) {
      storeDetails.value.name = store.name;
      storeDetails.value.address = store.address;
      storeDetails.value.logo = store.logo;
    }
  }
  catch (e) {
    console.log(e);
  }
})

const login = async () => {
  const login = await Login(loginForm.value)
  if (!login) {
    toasts.addToast({ message: 'Invalid login details', type: 'error' })
    return;
  }
  
  // toasts.addToast({ message: 'Login successful', type: 'success' })
  router.push({ name: "dashboard" });
}
</script>;

<template>
  <div class="h-screen flex flex-col sm:flex-row bg-stone-950 text-yellow-600">
    <div class="bg-[url('assets/images/login-image.jpg')] bg-cover bg-no-repeat flex-1"></div>
    <div class="p-5 flex flex-col min-w-[400px] max-w-[450px] min-h-[85vh] justify-between mx-auto sm:py-10">
      <div class="flex justify-between items-center">
        <div class="w-full">
          <div class="text-2xl">{{ storeDetails.name }}</div>
          <div v-if="storeDetails.address" class="mt-1">{{ storeDetails.address }}</div>
        </div>
        <div class="logo">
          <img :src="storeDetails.logo" class="max-w-[70px]" alt="logo">
        </div>
      </div>
      <div class="form-proper">
        <p class="text-xl mt-10 mb-4">Login to continue</p>
        <form v-on:submit.prevent="login" autocomplete="off">
          <TextField v-model="loginForm.username" class="mb-2" name="username" placeholder="Username" required />
          <TextField v-model="loginForm.password" class="mb-2" name="password" placeholder="Password" type="password" required />
          <PrimaryButton type="submit">Login</PrimaryButton>
        </form>
        <div class="text-center mt-5"><small>ZEBINV v1.0.0</small></div>
      </div>
    </div>
  </div>
</template>