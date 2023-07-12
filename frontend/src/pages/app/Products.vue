<script lang="ts" setup>
import { auth, toasts } from '../../stores';
import { TrashIcon, PencilIcon } from '@heroicons/vue/24/solid';
import TextField from '../../components/form/TextField.vue';
import PrimaryButton from '../../components/form/PrimaryButton.vue';
import SelectField from '../../components/form/SelectField.vue';
import { onMounted, ref } from 'vue';
import { Create, ReadAll, Delete } from '../../../wailsjs/go/controllers/Product'
import { ReadAll as ReadAllCategories } from '../../../wailsjs/go/controllers/Category'
import { models } from '../../../wailsjs/go/models';

const categories = ref([] as Array<models.Category>);
const product = ref({category_id: '', name: '', code: '', price: 0, stock_quantity: 0 });
const products = ref([] as Array<models.Product>);
const categoryOptions = ref([] as [string, (string|number)?][]);

const addProduct = async () => {
  if(product.value.name) {
    console.log(product.value);
    const create = await Create(product.value as never as models.Product);
    if(!create.success) {
      toasts.addToast({message: create.message, type: 'error'});
      return;
    }
    toasts.addToast({message: "product created successfully", type: 'success'});
    fetchProducts();
    product.value.name = product.value.code = '';
    product.value.stock_quantity = product.value.price = 0;
  }
}

const fetchCategories = async () => {categories.value = await ReadAllCategories()}
const fetchProducts = async () => {products.value = await ReadAll({query: '', category_id: ''})}
onMounted(async() => {
  await fetchCategories();
  await fetchProducts()

  categories.value.forEach((cat) => {
    categoryOptions.value.push([cat.name, cat.id]);
  })
})


const deleteProduct = async (id: string) => {
  const remove = await Delete(id);
    if(!remove.success) {
      toasts.addToast({message: remove.message, type: 'error'});
      return;
    }
    toasts.addToast({message: "product deleted successfully", type: 'success'});
    fetchProducts();
}
</script>;

<template>
  <div class="p-[15px]">
    <div class="text-2xl pt-2 pb-4">Products</div>
    <div></div>
    <div class="">
      <div class="border-[1px] border-yellow-600 px-3 p-2 mb-[10px] ">
        <h1 class="mt-3 text-xl">Add Product</h1>
        <div class="my-4">
          <form @submit.prevent="addProduct" class="space-y-5">
            <div class="flex space-x-2">
              <div class="w-full">
              <SelectField label="Product Category" v-model="product.category_id"  :options="categoryOptions" required/>
              </div>
              <div class="w-full">
                <TextField label="Product Code" v-model="product.code" placeholder="Code" />
              </div>
              <div class="w-full">
                <TextField label="Product Name" v-model="product.name" placeholder="Name" required/>
              </div>
            </div>
            <div class="flex space-x-2">
              <div class="w-full">
                <TextField label="Price" v-model.number="product.price" placeholder="Price" type="number"/>
              </div>
              <div class="w-full">
                <TextField label="Stock Quantity" v-model.number="product.stock_quantity" placeholder="Stock Quantity" type="number"/>
              </div>
              <div class="w-full">
                <div class="flex-1 py-2 mb-1 h-[30px]"></div>
                <PrimaryButton type="submit">Submit</PrimaryButton>
              </div>
            </div>
          </form>
      </div>
      </div>
      <div class="border-[1px] border-yellow-600 px-3 p-2">
        <div class="pr-1">
            <table class="table-auto w-full">
              <thead>
                <tr>
                  <th class="border-[1px] border-yellow-600 px-3 w-[50px]">S/N</th>
                  <th class="border-[1px] border-yellow-600 px-3 w-[100px]">CODE</th>
                  <th class="border-[1px] border-yellow-600 px-3 w-full">NAME</th>
                  <th class="border-[1px] border-yellow-600 px-3">CATEGORY</th>
                  <th class="border-[1px] border-yellow-600 px-3 w-[100px]">PRICE</th>
                  <th class="border-[1px] border-yellow-600 px-3 w-[100px]">STOCK</th>
                  <th class="border-[1px] border-yellow-600 px-3"></th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item, key in products" :key="key">
                  <td class="border-[1px] border-yellow-600 text-center">{{ key+1 }}</td>
                  <td class="border-[1px] border-yellow-600 px-3 ellipsis">{{ item.code }}</td>
                  <td class="border-[1px] border-yellow-600 px-3 ellipsis">{{ item.name }}</td>
                  <td class="border-[1px] border-yellow-600 px-3 ellipsis">{{ item.category?.name }}</td>
                  <td class="border-[1px] border-yellow-600 px-3 text-right">{{ item.price.toLocaleString("en-US") }}</td>
                  <td class="border-[1px] border-yellow-600 px-3 text-right">{{ item.stock_quantity.toLocaleString("en-US") }}</td>
                  <td class="border-[1px] border-yellow-600">
                    <div class="flex">
                      <router-link :to="{name: 'update-product', params: { id: item.id }}" title="Update item" class="border-[1px] w-[50px] border-yellow-600 px-3 py-1 hover:bg-yellow-600 hover:text-stone-950 active:bg-yellow-700 flex items-center justify-center text-center">
                        <PencilIcon class="w-5 h-5" />
                      </router-link>
                      <button title="Delete item" @click="() => deleteProduct(item.id)" class="border-[1px] w-[50px] border-yellow-600 px-3 py-1 hover:bg-yellow-600 hover:text-stone-950 active:bg-yellow-700 flex items-center justify-center text-center">
                        <TrashIcon class="w-5 h-5" />
                      </button>
                    </div>
                  </td>
                </tr>
                <tr v-if="!products.length">
                <td class="text-center py-5 px-2 border-[1px] border-yellow-600" colspan="7">No records found.</td>
                </tr>
              </tbody>
            </table>
          </div>
      </div>
    </div>
   </div>
</template>