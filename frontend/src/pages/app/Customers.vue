<script lang="ts" setup>
import { auth, toasts } from '../../stores';
import { TrashIcon } from '@heroicons/vue/24/solid';
import TextField from '../../components/form/TextField.vue';
import PrimaryButton from '../../components/form/PrimaryButton.vue';
import { onMounted, ref } from 'vue';
import { Create, ReadAll, Delete } from '../../../wailsjs/go/controllers/Customer'
import { models } from '../../../wailsjs/go/models';

const customer = ref(new models.Customer());
const customers = ref([] as Array<models.Customer>);

const addCustomer = async () => {
  if(customer.value.name) {
    console.log(customer.value);
    const create = await Create(customer.value as never as models.Customer);
    if(!create.success) {
      toasts.addToast({message: create.message, type: 'error'});
      return;
    }
    toasts.addToast({message: "customer created successfully", type: 'success'});
    fetchCustomers();
    customer.value.name = customer.value.customer_code = '';
    customer.value.phone = customer.value.email = '';
  }
}

const fetchCustomers = async () => {customers.value = await ReadAll({query: '', customer_id: ''})}

onMounted(async() => { await fetchCustomers(); })

const deleteCustomer = async (id: string) => {
  const remove = await Delete(id);
    if(!remove.success) {
      toasts.addToast({message: remove.message, type: 'error'});
      return;
    }
    toasts.addToast({message: "customer deleted successfully", type: 'success'});
    fetchCustomers();
}
</script>;

<template>
  <div class="p-[15px]">
    <div class="text-2xl pt-2 pb-4">Customers</div>
    <div></div>
    <div class="flex">
      <div class="w-[50%] border-[1px] border-yellow-600 px-3 p-2 mr-[10px] ">
        <h1 class="mt-3 text-xl">Add Customer</h1>
        <div class="my-4">
          <form @submit.prevent="addCustomer" class="space-y-5">

            <div class="flex space-x-2 w-full">
              <div class="w-full">                
                <TextField label="Customer Name" v-model="customer.name" placeholder="Name" required/>
              </div>
              <div class="w-full">
                <TextField label="Customer Code" v-model="customer.customer_code" placeholder="Code" />
              </div>
            </div>
            <div class="flex space-x-2 w-full">
              <div class="w-full">
                <TextField label="Phone" v-model="customer.phone" placeholder="Phone"/>
              </div>
              <div class="w-full">
                <TextField label="Email" v-model="customer.email" placeholder="Email"/>
              </div>
            </div>
            <div>
              <PrimaryButton type="submit">Submit</PrimaryButton>
            </div>
          </form>
      </div>
      </div>
      <div class="w-[50%] border-[1px] border-yellow-600 px-3 p-2">
        <div class="h-[65vh] overflow-scroll no-scrollbar pr-1">
            <table class="table-auto w-full">
              <thead>
                <tr>
                  <th class="border-[1px] border-yellow-600 px-3">S/N</th>
                  <th class="border-[1px] border-yellow-600 px-3">CODE</th>
                  <th class="border-[1px] border-yellow-600 px-3 w-full">NAME</th>
                  <th class="border-[1px] border-yellow-600 px-3"></th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item, key in customers" :key="key">
                  <td class="border-[1px] border-yellow-600 text-center w-[50px]">{{ key+1 }}</td>
                  <td class="border-[1px] border-yellow-600 px-3 w-[100px] ellipsis">{{ item.customer_code }}</td>
                  <td class="border-[1px] border-yellow-600 px-3 ellipsis">{{ item.name }}</td>
                  <td class="border-[1px] border-yellow-600">
                    <button title="Delete item" @click="() => deleteCustomer(item.id)" class="border-[1px] w-[50px] border-yellow-600 px-3 py-1 hover:bg-yellow-600 hover:text-stone-950 active:bg-yellow-700 flex items-center justify-center text-center">
                      <TrashIcon class="w-5 h-5" />
                    </button>
                  </td>
                </tr>
                <tr v-if="!customers.length">
                <td class="text-center py-5 px-2 border-[1px] border-yellow-600" colspan="6">No records found.</td>
                </tr>
              </tbody>
            </table>
          </div>
      </div>
    </div>
   </div>
</template>