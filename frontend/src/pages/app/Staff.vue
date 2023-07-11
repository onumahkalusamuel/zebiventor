<script lang="ts" setup>
import { auth, toasts } from '../../stores';
import { TrashIcon } from '@heroicons/vue/24/solid';
import TextField from '../../components/form/TextField.vue';
import PrimaryButton from '../../components/form/PrimaryButton.vue';
import { onMounted, ref } from 'vue';
import { Create, ReadAll, Delete } from '../../../wailsjs/go/controllers/Staff'
import { controllers, models } from '../../../wailsjs/go/models';

const staff = ref(new controllers.CreateStaffRequest());
const staffs = ref([] as Array<models.Staff>);

const addStaff = async () => {
  if(staff.value.name) {
    const create = await Create(staff.value);
    if(!create.success) {
      toasts.addToast({message: create.message, type: 'error'});
      return;
    }
    toasts.addToast({message: "staff created successfully", type: 'success'});
    fetchStaffs();
    staff.value.name = staff.value.username = staff.value.phone = staff.value.password = '';
  }
}

const fetchStaffs = async () => {staffs.value = await ReadAll({query: '', staff_id: ''})}

onMounted(async() => { await fetchStaffs(); })

const deleteStaff = async (id: string) => {
  const remove = await Delete(id);
    if(!remove.success) {
      toasts.addToast({message: remove.message, type: 'error'});
      return;
    }
    toasts.addToast({message: "staff deleted successfully", type: 'success'});
    fetchStaffs();
}
</script>;

<template>
  <div class="p-[15px]">
    <div class="text-2xl pt-2 pb-4">Staff Members</div>
    <div></div>
    <div class="flex">
      <div class="w-[50%] border-[1px] border-yellow-600 px-3 p-2 mr-[10px] ">
        <h1 class="mt-3 text-xl">Add Staff</h1>
        <div class="my-4">
          <form @submit.prevent="addStaff" class="space-y-5">

            <div class="flex space-x-2 w-full">
              <div class="w-full">                
                <TextField label="Staff Name" v-model="staff.name" placeholder="Name" required/>
              </div>
              <div class="w-full">
                <TextField label="Phone" v-model="staff.phone" placeholder="Phone"/>
              </div>
            </div>
            <div class="flex space-x-2 w-full">
              <div class="w-full">
                <TextField label="Username" v-model="staff.username" placeholder="Username" required />
              </div>
              <div class="w-full">
                <TextField label="Password" v-model="staff.password" placeholder="Password" required />
              </div>
            </div>
            <div>
              <PrimaryButton type="submit">Submit</PrimaryButton>
            </div>
          </form>
      </div>
      </div>
      <div class="w-[50%] border-[1px] border-yellow-600 px-3 p-2">
        <div class="h-[65vh] overflow-scroll pr-1">
            <table class="table-auto">
              <thead>
                <tr>
                  <th class="border-[1px] border-yellow-600 px-3">S/N</th>
                  <!-- <th class="border-[1px] border-yellow-600 px-3">CODE</th> -->
                  <th class="border-[1px] border-yellow-600 px-3 w-full">NAME (USERNAME)</th>
                  <th class="border-[1px] border-yellow-600 px-3">PHONE</th>
                  <th class="border-[1px] border-yellow-600 px-3"></th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item, key in staffs" :key="key">
                  <td class="border-[1px] border-yellow-600 text-center w-[50px]">{{ key+1 }}</td>
                  <!-- <td class="border-[1px] border-yellow-600 px-3 w-[100px] ellipsis">{{ item.staff_code }}</td> -->
                  <td class="border-[1px] border-yellow-600 px-3 ellipsis">{{ item.name }} ({{ item.username }})</td>
                  <td class="border-[1px] border-yellow-600 px-3 ellipsis">{{ item.phone }}</td>
                  <td class="border-[1px] border-yellow-600">
                    <button v-if="item.role!==1 && auth.role ==1" title="Delete item" @click="() => deleteStaff(item.id)" class="border-[1px] w-[50px] border-yellow-600 px-3 py-1 hover:bg-yellow-600 hover:text-stone-950 active:bg-yellow-700 flex items-center justify-center text-center">
                      <TrashIcon class="w-5 h-5" />
                    </button>
                  </td>
                </tr>
                <tr v-if="!staffs.length">
                <td class="text-center py-5 px-2 border-[1px] border-yellow-600" colspan="6">No records found.</td>
                </tr>
              </tbody>
            </table>
          </div>
      </div>
    </div>
   </div>
</template>