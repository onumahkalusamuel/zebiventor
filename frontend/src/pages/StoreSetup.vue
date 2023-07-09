<script lang="ts" setup>
import TextField from '../components/form/TextField.vue';
import PrimaryButton from '../components/form/PrimaryButton.vue';
import { onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import { toasts } from '../stores/toasts';
import { Setup } from '../../wailsjs/go/controllers/General'
import { controllers } from '../../wailsjs/go/models'

const router = useRouter();
const storesetup = ref({address: '', email: '', logo: '', name: '', phone: ''} as controllers.SetupRequest);
const logo = ref(null);

const hospitalSetup = async () => {
  const hospitalSetup = await Setup(storesetup.value);
  if(hospitalSetup?.message) {
    toasts.addToast({message: hospitalSetup.message, type: 'success'});
    router.push({name: 'login'})
  }
}
</script>;

<template>
  <p class="text-xl">Hospital Setup</p>
  <form v-on:submit.prevent="hospitalSetup" enctype="multipart/form-data">
    <div>
      <input v-model="storesetup.name" name="hospital_name" placeholder="Hospital Name" class="w-full mb-2"/>
      <input v-model="storesetup.address" name="hospital_address" placeholder="Physical Address" class="w-full mb-2"/>
      <input  v-model="storesetup.email" name="hospital_email" placeholder="Email Address" class="w-full mb-2"/>
      <input v-model="storesetup.phone" name="hospital_phone" placeholder="Phone Number" class="w-full mb-2"/>
      <input label="Hospital Logo" name="hospital_logo" class="w-full mb-2 font-xl" ref="logo" type="file"/>
      <PrimaryButton class="w-full mb-2" type="submit">Submit</PrimaryButton>
    </div>
  </form>
</template>
