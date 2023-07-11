<script lang="ts" setup>
import { useRouter } from 'vue-router';
import { ref } from 'vue';
import { toasts } from '../stores/toasts';
import PrimaryButton from '../components/form/PrimaryButton.vue';
import TextField from '../components/form/TextField.vue';
import { CreateAdmin } from '../../wailsjs/go/controllers/General'
import { controllers } from '../../wailsjs/go/models'

const router = useRouter();
const createadmin = ref({ name: '', password: '', username: '' } as controllers.CreateAdminRequest);

const createAdmin = async () => {
  const create = await CreateAdmin(createadmin.value);
  if (!create.success && create.id !== 1) {
    toasts.addToast({ message: create.message, type: 'error' });
    return;
  }
  toasts.addToast({ message: create.message, type: 'success' });
  router.push({ name: 'store-setup' })
}
</script>;

<template>
  <p class="text-xl">Create Super Admin</p>
  <form method="post" class="my-5" v-on:submit.prevent="createAdmin">
    <div>
      <TextField v-model="createadmin.name" placeholder="Name" class="w-full mb-2" />
      <TextField v-model="createadmin.username" placeholder="Username" class="w-full mb-2" />
      <TextField v-model="createadmin.password" placeholder="Password" class="w-full mb-2" />
      <PrimaryButton class="w-full mb-2" type="submit">Create Super Admin</PrimaryButton>
    </div>
  </form>
  <small>The super admin is the first user on the app. The account will have access to every section of the app.</small>
</template>
