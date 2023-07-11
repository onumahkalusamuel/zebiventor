<script lang="ts" setup>
import { onMounted, ref } from 'vue';
import { models } from '../../../wailsjs/go/models';
import { useRoute, useRouter } from 'vue-router';
import { ReadOne } from '../../../wailsjs/go/controllers/Sales';
import { PrinterIcon } from '@heroicons/vue/24/solid';
import { auth, store } from '../../stores';

const sale = ref(new models.Sale());
const route = useRoute()
const router = useRouter()
const printArea = ref();

const print = () => {
  (printArea.value as HTMLElement).classList.remove("hidden");
  window.print();
  (printArea.value as HTMLElement).classList.add("hidden");
}

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
    <div class="absolute top-0 left-0 bg-white h-screen w-screen mx-auto hidden" ref="printArea">
      <div class="w-[220px] bg-white text-stone-950 text-[9px] p-[10px] pt-1">
        <div class="text-center">
          <div class="text-[12px]"  style="text-transform:uppercase">{{ store.name }}</div>
          <div>{{ store.address }}</div>
          <div>{{ store.phone }}</div>
          <div><span class="text-[18px] font-bold">INVOICE</span> <br/>{{ sale.id }}</div>
          <div class="flex space-x-[10px] my-[5px]">
            <div class="w-[95px] text-center">
              <div class=" border-b-[1px] border-black font-bold">Customer</div>
              <div>{{ sale.customer?.customer_code }}</div>
            </div>
            <div class="w-[95px] text-center">
              <div class=" border-b-[1px] border-black font-bold">Payment</div>
              <div>{{  sale.payment_method  }} <span v-if="sale.payment_reference">({{ sale.payment_reference }})</span></div>
            </div>
          </div>
        </div>
    <table>
      <thead class="border-y-[1px] border-black">
        <tr>
          <th class="w-[30px]">QTY</th>
          <th class="w-[100px]">DESCRIPTION</th>
          <th class="w-[40px]">PRICE</th>
          <th class="w-[40px]">AMOUNT</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="item, key in sale.details" :key="key">
          <td class="text-center">{{ item.qty.toLocaleString("en-US") }}</td>
          <td class="w-[1px]">{{ item.product_name }}</td>
          <td class="w-[1px] text-right">{{ item.unit_price.toLocaleString("en-US") }}</td>
          <td class="w-[1px] text-right">{{ item.total.toLocaleString("en-US") }}</td>
        </tr>
      </tbody>
      <tfoot class="border-y-[1px] border-black">
        <tr>
          <td class=""></td>
          <td class="w-[1px]" colspan="2">SUBTOTAL</td>
          <td class="text-right font-bold">{{ (sale.sub_total || 0).toLocaleString("en-US") }}</td>
        </tr>
        <tr>
          <td class=""></td>
          <td class="w-[1px]" colspan="2">DISCOUNT</td>
          <td class="text-right font-bold">-{{ (sale.discount_amount || 0).toLocaleString("en-US") }}</td>
        </tr>
        <tr>
          <td class=""></td>
          <td class="w-[1px]" colspan="2">GRAND TOTAL</td>
          <td class="text-right font-bold">{{ (sale.grand_total || 0).toLocaleString("en-US") }}</td>
        </tr>
      </tfoot>
    </table>
    <div class="mt-[10px]"> Thank you for your patronage</div>
    <div> Trnx total is VAT inclusive</div>
    <div> NO REFUNDS. NO RETURNS. NO EXCHANGES.</div>
    <br/>
    <div class="text-[8px]">Printed by: {{ auth.username }};<br/>Date: {{ new Date().toLocaleDateString('en-us', { weekday:"short", year:"numeric", month:"short", day:"numeric", hour: "2-digit", minute: "2-digit", second: "2-digit"})  }}</div>
        













      </div>
    </div>
    <div class="flex justify-between items-center">
      <div class="text-2xl pt-2 pb-4">Sale Preview</div>
      <div>
        <button @click="print" class="flex items-center border-2 p-2 border-yellow-600 hover:bg-yellow-600 hover:text-stone-950 active:bg-yellow-700 active:border-yellow-700">
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