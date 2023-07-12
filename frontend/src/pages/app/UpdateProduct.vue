<script lang="ts" setup>
import { toasts } from '../../stores';
import TextField from '../../components/form/TextField.vue';
import PrimaryButton from '../../components/form/PrimaryButton.vue';
import SelectField from '../../components/form/SelectField.vue';
import { onMounted, ref } from 'vue';
import { Update, ReadOne } from '../../../wailsjs/go/controllers/Product'
import { ReadAll as ReadAllCategories } from '../../../wailsjs/go/controllers/Category'
import { models } from '../../../wailsjs/go/models';
import { useRoute } from 'vue-router';

const categories = ref([] as Array<models.Category>);
const product = ref(new models.Product());
const categoryOptions = ref([] as [string, (string|number)?][]);
const route = useRoute()

const updateProduct = async () => {
  if(product.value.name) {
    const update = await Update(route.params.id as string, product.value);
    if(!update.success) {
      toasts.addToast({message: update.message, type: 'error'});
      return;
    }
    toasts.addToast({message: "product updated successfully", type: 'success'});
    fetchProduct();
    product.value.name = product.value.code = '';
    product.value.stock_quantity = product.value.price = 0;
  }
}

const fetchCategories = async () => {categories.value = await ReadAllCategories()}
const fetchProduct = async () => {product.value = await ReadOne(route.params.id as string)}
onMounted(async() => {
  await fetchCategories();
  await fetchProduct()

  categories.value.forEach((cat) => {
    categoryOptions.value.push([cat.name, cat.id]);
  })
})

</script>;

<template>
  <div class="p-[15px]">

    <div class="flex justify-between items-center mb-4">
      <div class="text-2xl pt-2">Update {{ product.name }}</div>
      <div>
        <router-link :to="{name:'products'}" class="flex items-center border-[1px] p-1 px-2 border-yellow-600 hover:bg-yellow-600 hover:text-stone-950 active:bg-yellow-700 active:border-yellow-700">
          <div>Cancel</div>
        </router-link>
      </div>
    </div>
    <div class="">
      <div class="border-[1px] border-yellow-600 px-3 p-2 mb-[10px] ">
        <h1 class="mt-3 text-xl">Add Product</h1>
        <div class="my-4">
          <form @submit.prevent="updateProduct" class="space-y-5">
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
    </div>
   </div>
</template>