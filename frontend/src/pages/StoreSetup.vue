<script lang="ts" setup>
import TextField from '../components/form/TextField.vue';
import PrimaryButton from '../components/form/PrimaryButton.vue';
import { useRouter } from 'vue-router';
import { toasts } from '../stores/toasts';
import { Setup } from '../../wailsjs/go/controllers/General'
import { controllers } from '../../wailsjs/go/models'
import { store } from '../stores';

const router = useRouter();

const storeSetup = async () => {
  const storeSetup = await Setup(store as controllers.SetupRequest);
  if(!storeSetup.success && storeSetup.code !== 1) {
    toasts.addToast({message: storeSetup.message, type: 'error'});
    return;
  }
  toasts.addToast({message: storeSetup.message, type: 'success'});
  router.push({name: 'login'})
}
</script>;

<template>
  <p class="text-xl">Store Setup</p>
  <form v-on:submit.prevent="storeSetup" enctype="multipart/form-data" class="my-5">
    <div>
      <TextField v-model="store.name" name="name" placeholder="Store Name" class="w-full mb-2"/>
      <TextField v-model="store.address" name="address" placeholder="Physical Address" class="w-full mb-2"/>
      <TextField  v-model="store.email" name="email" placeholder="Email Address" class="w-full mb-2"/>
      <TextField v-model="store.phone" name="phone" placeholder="Phone Number" class="w-full mb-2"/>
      <div>
        <div class="py-2">Store Logo</div>
        <div class="border-[1px] border-yellow-600 p-2 pb-1">
          <input label="Store Logo" class="w-full mb-2 font-xl" type="file" @change="store.getBase64"/>
        </div>
      </div>
      <PrimaryButton class="w-full mb-2" type="submit">Submit</PrimaryButton>
    </div>
  </form>
</template>
