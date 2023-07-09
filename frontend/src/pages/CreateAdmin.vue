<script lang="ts" setup>
import { useRouter } from 'vue-router';
import { ref } from 'vue';
import { toasts } from '../stores/toasts';
import { CreateAdmin } from '../../wailsjs/go/controllers/General'
import { controllers } from '../../wailsjs/go/models'

const router = useRouter();
const createadmin = ref({ name: '', password: '', username: '' } as controllers.CreateAdminRequest);

const createAdmin = async () => {
  const create = await CreateAdmin(createadmin.value);
  if (create.message) {
    toasts.addToast({ message: create.message, type: 'success' });
    router.push({ name: 'store-setup' })
  }
}
</script>;

<template>
  <p class="text-xl">Create Super Admin</p>
  <form method="post" v-on:submit.prevent="createAdmin">
    <div>
      <input name="name" placeholder="Name" class="w-full mb-2" />
      <input name="username" placeholder="Username" class="w-full mb-2" />
      <input name="password" placeholder="Password" class="w-full mb-2" />
      <button class="w-full mb-2" type="submit">Create Super Admin</button>
    </div>
  </form>
  <small>The super admin is the first user on the app. The account will have access to every section of the app.</small>
</template>
