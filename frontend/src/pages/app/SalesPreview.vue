<script lang="ts" setup>
import { onMounted, ref } from 'vue';
import { models } from '../../../wailsjs/go/models';
import { useRoute, useRouter } from 'vue-router';
import { ReadOne } from '../../../wailsjs/go/controllers/Sales';
import { PrinterIcon } from '@heroicons/vue/24/solid';

const sale = ref(new models.Sale());
const route = useRoute()
const router = useRouter()

onMounted(async() => {
  const salePreview = await ReadOne(route.params.id as string)
  sale.value = salePreview
  if(!sale.value.id) {
    router.push({ name: 'dashboard' })
  }
  console.log(sale.value, salePreview);
})
</script>

<template>
  <div class="p-[15px]">
    <div class="flex justify-between items-center">
      <div class="text-2xl pt-2 pb-4">Sale Preview</div>
      <div>
        <button @click="" class="flex items-center border-2 p-2 border-yellow-600 hover:bg-yellow-600 hover:text-stone-950 active:bg-yellow-700 active:border-yellow-700">
          <PrinterIcon class="h-5 w-5 mr-2"/>
          <div>PRINT</div>
        </button></div>
    </div>
    <div class="py-5">
      <div class="flex justify-between space-y-2 flex-wrap">
        <div class="flex">
          <div class="w-[120px] border-2 bg-yellow-600 border-yellow-600 text-stone-950 p-1 px-3 font-bold">Customer</div>
          <div class="w-[200px] p-1 px-3 border-[1px] border-yellow-600">{{  sale.customer?.name  }} ({{ sale.customer?.customer_code }})</div>
        </div>

        <div class="flex">
          <div class="w-[120px] border-2 bg-yellow-600 border-yellow-600 text-stone-950 p-1 px-3 font-bold">Discount</div>
          <div class="w-[200px] p-1 px-3 border-[1px] border-yellow-600"> {{ (sale.discount_amount||0).toLocaleString("en-US") }}</div>
        </div>

        <div class="flex">
          <div class="w-[120px] border-2 bg-yellow-600 border-yellow-600 text-stone-950 p-1 px-3 font-bold">Sub Total</div>
          <div class="w-[200px] p-1 px-3 border-[1px] border-yellow-600">{{ (sale.sub_total||0).toLocaleString("en-US") }}</div>
        </div>

        <div class="flex">
          <div class="w-[120px] border-2 bg-yellow-600 border-yellow-600 text-stone-950 p-1 px-3 font-bold">Grand Total</div>
          <div class="w-[200px] p-1 px-3 border-[1px] border-yellow-600">{{  (sale.grand_total||0).toLocaleString("en-US")  }}</div>
        </div>
      
        <div class="flex">
          <div class="w-[120px] border-2 bg-yellow-600 border-yellow-600 text-stone-950 p-1 px-3 font-bold">Payment</div>
          <div class="w-[200px] p-1 px-3 border-[1px] border-yellow-600">{{  sale.payment_method  }} <span v-if="sale.payment_reference">({{ sale.payment_reference }})</span></div>
        </div>
      </div>
    </div>

    <div class="text-xl py-4">Packing List</div>
    <div class="flex">
      <div class="w-full border-[1px] border-yellow-600 px-3 p-2 h-[50vh] overflow-scroll no-scrollbar">
        <div class="">
            <table class="table-auto w-full">
              <thead>
                <tr>
                  <th class="border-[1px] border-yellow-600 px-3">S/N</th>
                  <th class="border-[1px] border-yellow-600 px-3">CODE</th>
                  <th class="border-[1px] border-yellow-600 px-3">PRODUCT NAME</th>
                  <th class="border-[1px] border-yellow-600 px-3">QTY</th>
                  <th class="border-[1px] border-yellow-600 px-3">UNIT PRICE</th>
                  <th class="border-[1px] border-yellow-600 px-3">TOTAL</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item, key in sale.details" :key="key">
                  <td class="border-[1px] border-yellow-600 text-center w-[50px]">{{ key+1 }}</td>
                  <td class="border-[1px] border-yellow-600 px-3 w-[100px] ellipsis">{{ item.product_code }}</td>
                  <td class="border-[1px] border-yellow-600 px-3 ellipsis">{{ item.product_name }}</td>
                  <td class="border-[1px] border-yellow-600 px-3 w-[80px] text-center">{{ item.qty.toLocaleString("en-US") }}</td>
                  <td class="border-[1px] border-yellow-600 px-3 w-[120px] text-right">{{ item.unit_price.toLocaleString("en-US") }}</td>
                  <td class="border-[1px] border-yellow-600 px-3 w-[120px] text-right">{{ item.total.toLocaleString("en-US") }}</td>
                </tr>
              </tbody>
            </table>
          </div>
      </div>
    </div>
   </div>
</template>