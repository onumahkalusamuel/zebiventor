<script lang="ts" setup>
import { auth, store, toasts } from '../../stores';
import TextField from '../../components/form/TextField.vue';
import PrimaryButton from '../../components/form/PrimaryButton.vue';
import { onMounted, ref } from 'vue';
import { Update, ReadProfile } from '../../../wailsjs/go/controllers/Staff'
import { controllers, models } from '../../../wailsjs/go/models';
import { UpdateStore } from '../../../wailsjs/go/controllers/General';

const staff = ref(new controllers.UpdateStaffRequest());
const record = ref(new models.Staff())

const updateProfile = async () => {
  if(staff.value.name) {
    const update = await Update(record.value.id, staff.value);
    if(!update.success) {
      toasts.addToast({message: update.message, type: 'error'});
      return;
    }
    toasts.addToast({message: "record updated successfully", type: 'success'});
    fetchProfile();
    staff.value.name = staff.value.username = staff.value.phone = staff.value.password = '';
  }
}

const fetchProfile = async () => {
  record.value = await ReadProfile()
  staff.value.name = record.value.name
  staff.value.username = record.value.username
  staff.value.phone = record.value.phone
}

const updateStore = async () => {  
  const update = await UpdateStore(store as controllers.SetupRequest);
  if(!update.success) {
    toasts.addToast({message: update.message, type: 'error'});
    return;
  }
  toasts.addToast({message: "store updated successfully", type: 'success'});
  store.logo_string = store.logo_type = ''
}

onMounted(async() => { await fetchProfile(); })
</script>;

<template>
  <div class="p-[15px]">
    <div class="text-2xl pt-2 pb-4">Settings</div>
    <div></div>
    <div class="flex">
      <div class="w-[50%] border-[1px] border-yellow-600 px-3 p-2 mr-[10px]">
        <h1 class="mt-3 text-xl">Profile Settings</h1>
        <div class="my-4">
          <form @submit.prevent="updateProfile" class="space-y-5">

            <div class="flex space-x-2 w-full">
              <div class="w-full">                
                <TextField label="Name" v-model="staff.name" placeholder="Name" required/>
              </div>
              <div class="w-full">
                <TextField label="Phone" v-model="staff.phone" placeholder="Phone"/>
              </div>
            </div>
            <div class="flex space-x-2 w-full">
              <div class="w-full">
                <TextField label="Password" v-model="staff.password" placeholder="Password" />
              </div>
            </div>
            <div>
              <PrimaryButton type="submit">Submit</PrimaryButton>
            </div>
          </form>
      </div>
      </div>
      <div class="w-[50%] border-[1px] border-yellow-600 px-3 p-2" v-if="auth.role == 1">
        <h1 class="mt-3 text-xl">System Settings</h1>
        <div class="my-4">
          <form @submit.prevent="updateStore" class="space-y-5">

            <div class="flex space-x-2 w-full">
              <div class="w-full">                
                <TextField v-model="store.name" label="Store Name" class="w-full mb-2" required />
              </div>
              <div class="w-full">
                <TextField v-model="store.address" label="Physical Address" class="w-full mb-2" required />
              </div>
            </div>
            <div class="flex space-x-2 w-full">
              <div class="w-full">
                <TextField  v-model="store.email" label="Email Address" class="w-full mb-2"/>
              </div>
              <div class="w-full">
                <TextField v-model="store.phone" label="Phone Number" class="w-full mb-2"/>
              </div>
            </div>
            <div class="flex space-x-2 w-full">
              <div class="w-full">
                <div class="py-2">Store Logo</div>
                <div class="border-[1px] border-yellow-600 p-2 pb-1">
                  <input label="Store Logo" class="w-full mb-2 font-xl" ref="logo" type="file" @change="store.getBase64"/>
                </div>
              </div>
            </div>
            <div>
              <PrimaryButton type="submit">Submit</PrimaryButton>
            </div>
          </form>
      </div>
      </div>
    </div>
   </div>
</template>