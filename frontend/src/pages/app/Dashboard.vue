<script lang="ts" setup>
import { toasts } from '../../stores';
import { PlusIcon, TrashIcon, MagnifyingGlassIcon, ArrowRightIcon } from '@heroicons/vue/24/solid';
import TextField from '../../components/form/TextField.vue';
import SelectField from '../../components/form/SelectField.vue';
import PrimaryButton from '../../components/form/PrimaryButton.vue';
import { computed, onMounted, ref } from 'vue';
import {v4 as uuid} from 'uuid';
import { FindByCode } from '../../../wailsjs/go/controllers/Product'
import { models } from '../../../wailsjs/go/models'
import { ReadAll as ReadAllProducts } from '../../../wailsjs/go/controllers/Product'
import { ReadAll as ReadAllCustomers } from '../../../wailsjs/go/controllers/Customer'
import { Create } from '../../../wailsjs/go/controllers/Sales'
import { router } from '../../router';

const cartItems = ref({[uuid()]: {}} as {[key:string]: CartItem});
const discount = ref(0);
const sale = ref({} as models.Sale)
const customers = ref([] as Array<models.Customer>)
const subTotal = computed((): number => {
    let sum = 0;
    Object.values(cartItems.value).forEach(element => {
      element.total = (element.qty ?? 0) * (element.price ?? 0)
      sum += element.total
    });
    return sum
})

interface CartItem {
  id?: string;
  code?: string;
  name?: string;
  qty?: number;
  price?: number;
  total?: number;
  stock_quantity?: number;
}

const customerOptions = ref([] as [string, (string|number)?][]);

const addToCart = () => {
  cartItems.value[uuid()] = {};
}
const removeFromCart = (key: string) => {
  delete cartItems.value[key]
  if(!Object.values(cartItems.value).length) cartItems.value[uuid()] = {}
}

const clearCart = () => {
  cartItems.value = {};
  cartItems.value[uuid()] = {}

}
const checkout = async () => {
  sale.value.details = [];
  let overQty = [] as string[];

  const cartKeys = Object.keys(cartItems.value);

  Object.values(cartItems.value).forEach((item, index) => {
    if((item.qty as number) > (item.stock_quantity as number)) {
      overQty.push(`(${item.name} => from ${item.qty} to ${item.stock_quantity})`)
      cartItems.value[cartKeys[index]].qty = cartItems.value[cartKeys[index]].stock_quantity
    }
    if((item.qty as number) > 0) {
      sale.value.details.push({
        product_id: item.id || '',
        product_code: item.code || '',
        product_name: item.name || '',
        qty: item.qty || 0,
        total: item.total || 0,
        unit_price: item.price || 0
      });
    }
  });
 
  if(overQty.length > 0) {
    toasts.addToast({type: 'error', message: `Confirm changes. Quantity adjustment:\n ${overQty.join("\n")}`, })
    return;
  }

  sale.value.discount_amount = discount.value;
  sale.value.sub_total = subTotal.value;

  const makeSales = await Create(sale.value);


  if(!makeSales.success) {
      toasts.addToast({message: makeSales.message, type: 'error'});
      return;
    }
    // toasts.addToast({message: "Sales  completed.", type: 'success'});
    // redirect
    router.push({ name: 'sales-preview', params: {id: makeSales.id }});
}

const fetchProduct = async (key: string, code: string) => {
  if(code == "undefined" || code.length == 0) return;
  const product = await FindByCode(code);
  
  if(product.id) {
    cartItems.value[key].id = product.id;
    cartItems.value[key].price = product.price;
    cartItems.value[key].name = product.name;
    cartItems.value[key].stock_quantity = product.stock_quantity;
    cartItems.value[key].qty = 1;
  } else {
    toasts.addToast({message: `Product with code "${code}" not found`, type: 'error'});
  }

}

const fetchCustomers = async () => {customers.value = await ReadAllCustomers({customer_id: '', query: ''})}

onMounted(async () => {
  await fetchCustomers()
  customers.value.forEach((user) => {
    customerOptions.value.push([`${user.name} (${user.customer_code})`, user.id]);
  })
})

const dialogState = ref(false);
const selectedProducts = ref([] as Array<models.Product>);
const searchResults = ref([] as Array<models.Product>);
const searchField = ref('');
const searchForProducts = async () => {
  if(searchField.value.length > 0) {
    searchResults.value = await ReadAllProducts({query: searchField.value, category_id: ''})
  }
}
const addToSelected = (item: models.Product) => (selectedProducts.value = [...selectedProducts.value.filter((p) => p.code!=item.code), item])
const removeFromSelected = (index: number) => selectedProducts.value.splice(index, 1);
const updateCartWithelected = () => {
  
  if(selectedProducts.value.length === 0) return;

  let cartKeys = Object.keys(cartItems.value)
  let cartValues = Object.values(cartItems.value);

  if(cartValues.length == 1) {
    if(!cartValues[0].id) {
      delete cartItems.value[cartKeys[0]]
      cartKeys = cartValues = [];
    }
  }

  selectedProducts.value.forEach((item) => {
    let updated = false;
    cartValues.map((i, index) => {
      // progress
      if(item.id === i.id) {
        (cartItems.value[cartKeys[index]].qty as number)++
        updated = true;
        return;
      }
    });

    // create new record
    if(!updated) {
      cartItems.value[uuid()] = {
        code: item.code,
        id: item.id,
        name: item.name,
        price: item.price,
        stock_quantity: item.stock_quantity,
        qty: 1,
      };
    }
    
  });

  // empty the basket
  selectedProducts.value = [];
}
</script>;

<template>
  <GDialog v-model="dialogState" max-width="720">
    <div class="bg-stone-950 border-2 border-yellow-600 text-yellow-600">
      <div class="px-[15px]">
        <div class="text-xl py-4">Find Product (s)</div>
        <form class="flex" @submit.prevent="searchForProducts">
          <TextField placeholder="Enter part of product name or code" v-model="searchField" />
          <button @click.stop="searchForProducts" type="submit" class="border-[1px] border-yellow-600 hover:bg-yellow-600 hover:text-stone-950 px-3 focus:outline-none">
            <ArrowRightIcon class="w-5 h-5"/>
          </button>
        </form>
        <div class="flex my-3 space-x-2">
          <div class="w-[50%] p-2 border-yellow-600 border-[1px] h-[60vh] overflow-scroll no-scrollbar pr-1">
            <div class="list">
              <table class="table-fixed w-full">
                <thead>
                  <tr>
                    <th class="border-[1px] border-yellow-600 px-3">PRODUCTS</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="listItem, x in searchResults" :key="x">
                    <td class="border-[1px] border-yellow-600 px-3">
                      <div class="flex items-center justify-between py-1">
                        <div>{{ listItem.name }} ({{ listItem.stock_quantity.toLocaleString("en-US") }} available)</div>
                        <button @click="() => addToSelected(listItem)" class="border-[1px] border-yellow-600 px-1 ml-1">
                          <PlusIcon class="w-6 h-6" />
                        </button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
          <div class="w-[50%] p-2 border-yellow-600 border-[1px] h-[60vh] overflow-scroll no-scrollbar pr-1">

            <table class="table-fixed w-full">
              <thead>
                <tr>
                  <th class="border-[1px] border-yellow-600 px-3">SELECTED</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item, x in selectedProducts" :key="x">
                  <td class="border-[1px] border-yellow-600 px-3">
                    <div class="flex items-center justify-between py-1">
                      <div>{{ item.name }}</div>
                      <button @click="() => removeFromSelected(x)" class="border-[1px] border-yellow-600 px-1 ml-1">
                        <TrashIcon class="w-6 h-6"/>
                      </button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <div class="flex space-x-2 px-[15px] pb-3">
        <div class="flex">
          <PrimaryButton @click="() => {dialogState = false; updateCartWithelected()}">Update Cart</PrimaryButton>
        </div>
        <div class="flex">
          <PrimaryButton @click="dialogState = false">Close</PrimaryButton>
        </div>
      </div>
    </div>
  </GDialog>
  
  <div class="flex flex-col p-[15px]">
    <div class="text-2xl pt-2 pb-4">New Sale</div>
    <div class="flex flex-1">
      <div class="flex-1 border-[1px] border-yellow-600 px-3 p-2 mr-[10px]">
        <div class="">
          <div class="flex justify-between my-2">
            <div class="font-bold">ACTIVE CART</div>
            <div class="flex space-x-2">
              <button @click="dialogState = true" class="border-[1px] border-yellow-600 px-3 py-1 hover:bg-yellow-600 hover:text-stone-950 active:bg-yellow-700 flex items-center">
                Find Product (s)
                <MagnifyingGlassIcon class="w-5 h-5 ml-3" />
              </button>
              <button @click="addToCart" class="border-[1px] border-yellow-600 px-3 py-1 hover:bg-yellow-600 hover:text-stone-950 active:bg-yellow-700 flex items-center">
                Add
                <PlusIcon class="w-5 h-5 ml-3" />
              </button>
              <button @click="clearCart" class="border-[1px] border-yellow-600 px-3 py-1 hover:bg-yellow-600 hover:text-stone-950 active:bg-yellow-700 flex items-center">
                Clear List
                <TrashIcon class="w-5 h-5 ml-3" />
              </button>
            </div>
          </div>
          <div class="pr-1">
            <table class="table-auto w-full">
              <thead>
                <tr>
                  <th class="border-[1px] border-yellow-600 px-3">CODE</th>
                  <th class="border-[1px] border-yellow-600 px-3">NAME</th>
                  <th class="border-[1px] border-yellow-600 px-3">QTY</th>
                  <th class="border-[1px] border-yellow-600 px-3">PRICE</th>
                  <th class="border-[1px] border-yellow-600 px-3">TOTAL</th>
                  <th class="border-[1px] border-yellow-600 px-3"></th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item, key in cartItems" :key="key">
                  <td class="border-[1px] border-yellow-600 text-center w-[100px]">
                    <div class="flex">
                      <TextField class="w-[100px] bg-stone-800 text-center border-[1px] border-yellow-600" v-model="item.code" name="code" :disabled="!!item.id"/>
                      <button @click="() => fetchProduct(`${key}`, `${item.code}`)" class="border-[1px] border-yellow-600 hover:bg-yellow-600 hover:text-stone-950" :disabled="!!item.id" :class="!!item.id?'hidden':''">
                        <ArrowRightIcon class="w-5 h-5"/>
                      </button>
                    </div>
                  </td>
                  <td class="border-[1px] border-yellow-600 px-3 ellipsis"><span v-if="item.name">{{ item.name }} ({{ (item.stock_quantity || 0).toLocaleString("en-US") }} available)</span></td>
                  <td class="border-[1px] border-yellow-600 w-[100px]">
                    <TextField class="w-[100px] bg-stone-800 text-center border-[1px] border-yellow-600" v-model.number="item.qty" type="number" :disabled="!item.id" :max="item.stock_quantity" />
                  </td>
                  <td class="border-[1px] border-yellow-600 w-[120px]">
                    <TextField class="w-[120px] bg-stone-800 text-center border-[1px] border-yellow-600" v-model.number="item.price" type="number" :disabled="!item.id" />
                  </td>
                  <td class="border-[1px] border-yellow-600 px-3 text-right w-[140px]">
                      {{ ((item.qty || 0) * (item.price || 0)).toLocaleString("en-US") }}
                  </td>
                  <td class="border-[1px] border-yellow-600 w-[50px]">
                    <button title="Delete item" @click="() => removeFromCart(`${key}`)" class="border-[1px] w-[50px] border-yellow-600 py-1 hover:bg-yellow-600 hover:text-stone-950 active:bg-yellow-700 flex items-center justify-center text-center">  
                      <TrashIcon class="w-5 h-5" />
                    </button>
                  </td>
                </tr>
              </tbody>
              <tfoot>
                <tr>
                  <th class="border-[1px] border-yellow-600 px-3 text-right" colspan="4">SUB TOTAL</th>
                  <th class="text-xl font-bold border-[1px] border-yellow-600 px-3 text-right">{{ subTotal.toLocaleString("en-US") }}</th>
                  <th class="border-[1px] border-yellow-600 px-3"></th>
                </tr>
              </tfoot>
            </table>
          </div>
        <div class="flex justify-between py-3 pt-5">
          <div>
            DISCOUNT: 
            <div class="w-[150px]">
              <TextField v-model.number="discount" type="number"/>
            </div>
          </div>
          <div>
            GRAND TOTAL: 
            <div class="border-[1px] border-yellow-600 w-[120px] px-2 font-bold text-xl text-right">
              {{ (subTotal - discount).toLocaleString("en-US") }}
            </div>
          </div>
        </div>
      </div>
      </div>
      <div class="w-[400px] border-[1px] border-yellow-600 px-3 p-2">
        <form @submit.prevent="checkout" class="space-y-5 flex flex-col h-full">
            <div class="w-full">
              <SelectField label="Customer" v-model="sale.customer_id"  :options="customerOptions" required/>
            </div>
            <div class="w-full">
              <SelectField label="Payment Method" v-model="sale.payment_method"  :options="[['Cash'],['Card'],['Transfer']]" required/>
            </div>
            <div class="w-full" :class="sale.payment_method=='Cash'?'hidden':''">
              <TextField label="Payment Reference" v-model="sale.payment_reference"/>
            </div>
            <div class="flex-1"></div>
            <div>
              <PrimaryButton type="submit" :disabled="!subTotal" :class="!subTotal?'bg-gray-500 hover:bg-gray-500 active:bg-gray-500':''">Checkout</PrimaryButton>
            </div>
          </form>
      </div>
    </div>
  </div>
</template>