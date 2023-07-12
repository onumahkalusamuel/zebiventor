<script lang="ts" setup>
import { auth, toasts } from '../../stores';
import { TrashIcon, ShoppingBagIcon, EyeIcon } from '@heroicons/vue/24/solid';
import { computed, onMounted, ref } from 'vue';
import { ReadAll, Delete } from '../../../wailsjs/go/controllers/Sales'
import { models } from '../../../wailsjs/go/models';

const sales = ref([] as Array<models.Sale>);

const fetchSales = async () => {
  sales.value = await ReadAll({customer_id: '', created_at: ''})
}
onMounted(async() => { await fetchSales()})
// const sales = computed(():Array<models.Sale> => {
//   return []
// })

const deleteSale = async (id: string) => {
  const remove =   await Delete(id);
    if(!remove.success) {
      toasts.addToast({message: remove.message, type: 'error'});
      return;
    }
    toasts.addToast({message: "Sale deleted successfully", type: 'success'});
    fetchSales();
}

const salesFor = ref('today');
</script>;

<template>
  <div class="p-[15px]">
    <div class="flex justify-between items-center mb-4">
      <div class="text-2xl pt-2">Sales</div>
      <div>
        <router-link :to="{name:'dashboard'}" class="flex items-center border-[1px] p-1 px-2 border-yellow-600 hover:bg-yellow-600 hover:text-stone-950 active:bg-yellow-700 active:border-yellow-700">
          <ShoppingBagIcon class="h-5 w-5 mr-2"/>
          <div>New Sale</div>
        </router-link>
      </div>
    </div>
      <div class="flex space-x-4 my-4 items-center hidden">
        <div>SHOW SALES FOR: </div>
        <label class="border-2 px-2 py-1 cursor-pointer border-yellow-600" :class="salesFor=='today'?'bg-yellow-600 text-stone-950':''">
          <input type="radio" @change="fetchSales" v-model="salesFor" value="today" class="hidden" /> Today
        </label>
        <label class="border-2 px-2 py-1 cursor-pointer border-yellow-600" :class="salesFor=='week'?'bg-yellow-600 text-stone-950':''">
          <input type="radio" @change="fetchSales" v-model="salesFor" value="week" class="hidden" /> This Week
        </label>
        <label class="border-2 px-2 py-1 cursor-pointer border-yellow-600" :class="salesFor=='month'?'bg-yellow-600 text-stone-950':''">
          <input type="radio" @change="fetchSales" v-model="salesFor" value="month" class="hidden" /> This Month
        </label>
        <label class="border-2 px-2 py-1 cursor-pointer border-yellow-600" :class="salesFor=='year'?'bg-yellow-600 text-stone-950':''">
          <input type="radio" @change="fetchSales" v-model="salesFor" value="year" class="hidden" /> This Year
        </label>
      </div>

    <div class="w-full border-[1px] border-yellow-600 px-3 p-2">
      <div class="pr-1">
          <table class="table-auto w-full">
            <thead>
              <tr>
                <th class="border-[1px] border-yellow-600 px-3">S/N</th>
                <th class="border-[1px] border-yellow-600 px-3 w-[150px]">DATE</th>
                <th class="border-[1px] border-yellow-600 px-3">CUSTOMER</th>
                <th class="border-[1px] border-yellow-600 px-3 w-[150px]">GRAND TOTAL</th>
                <th class="border-[1px] border-yellow-600 px-3 w-[150px]">PAYMENT</th>
                <th class="border-[1px] border-yellow-600 px-3"></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item, key in sales" :key="key">
                <td class="border-[1px] border-yellow-600 text-center w-[100px]">{{ key+1 }}</td>
                <td class="border-[1px] border-yellow-600 px-3 text-center">{{ (item?.created_at|| '').split('T')[0] }}</td>
                <td class="border-[1px] border-yellow-600 px-3 ellipsis">{{ item?.customer?.name }} ({{ item?.customer?.customer_code }})</td>
                <td class="border-[1px] border-yellow-600 px-3">{{ item?.grand_total }}</td>
                <td class="border-[1px] border-yellow-600 px-3">
                  {{ item.payment_method }} 
                  <span v-if="item.payment_reference">({{ item?.payment_reference }})</span>
                </td>
                <td class="border-[1px] border-yellow-600 w-[100px]">
                  <div class="flex">
                    <router-link title="View Item" :to="{name: 'sales-preview', params: {id: item.id}}" class="border-[1px] w-[50px] border-yellow-600 py-1 hover:bg-yellow-600 hover:text-stone-950 active:bg-yellow-700 flex items-center justify-center text-center">  
                      <EyeIcon class="w-5 h-5" />
                    </router-link>
                    <button v-if="auth.role ==1" title="Delete item" @click="() => deleteSale(item.id)" class="border-[1px] w-[50px] border-yellow-600 py-1 hover:bg-yellow-600 hover:text-stone-950 active:bg-yellow-700 flex items-center justify-center text-center">  
                      <TrashIcon class="w-5 h-5" />
                    </button>
                  </div>
                </td>
              </tr>
              <tr v-if="!sales.length">
              <td class="text-center py-5 px-2 border-[1px] border-yellow-600" colspan="5">No records found.</td>
              </tr>
            </tbody>
          </table>
        </div>
    </div>
    
   </div>
</template>