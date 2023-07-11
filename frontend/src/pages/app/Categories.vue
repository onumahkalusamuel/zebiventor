<script lang="ts" setup>
import { auth, toasts } from '../../stores';
import { TrashIcon } from '@heroicons/vue/24/solid';
import TextField from '../../components/form/TextField.vue';
import PrimaryButton from '../../components/form/PrimaryButton.vue';
import { onMounted, ref } from 'vue';
import { Create, ReadAll, Delete } from '../../../wailsjs/go/controllers/Category'
import { models } from '../../../wailsjs/go/models';


const category = ref({name: ''});
const categories = ref([] as Array<models.Category>);

const addCategory = async () => {
  if(category.value.name) {
    const create = await Create(category.value.name);
    if(!create.success) {
      toasts.addToast({message: create.message, type: 'error'});
      return;
    }
    toasts.addToast({message: "category created successfully", type: 'success'});
    fetchCategories();
    category.value.name = '';
  }
}

const fetchCategories = async () => {categories.value = await ReadAll()}
onMounted(async() => { await fetchCategories()})


const deleteCategory = async (id: string) => {
  const remove = await Delete(id);
    if(!remove.success) {
      toasts.addToast({message: remove.message, type: 'error'});
      return;
    }
    toasts.addToast({message: "category deleted successfully", type: 'success'});
    fetchCategories();
}
</script>;

<template>
  <div class="p-[15px]">
    <div class="text-2xl pt-2 pb-4">Categories</div>
    <div></div>
    <div class="flex">
      <div class="w-[50%] border-[1px] border-yellow-600 px-3 p-2 mr-[10px] ">
        <h1 class="mt-3">Add Category</h1>
        <div class="my-4">
          <form @submit.prevent="addCategory" class="space-y-5">
            <div>
              <TextField v-model="category.name" placeholder="Category Name" class="bg-stone-800 text-center border-[1px] border-yellow-600" required/>
            </div>
            <div>
              <PrimaryButton type="submit">Submit</PrimaryButton>
            </div>
          </form>
      </div>
      </div>
      <div class="w-[50%] border-[1px] border-yellow-600 px-3 p-2">
        <div class="h-[65vh] overflow-scroll pr-1">
            <table class="table-auto w-full">
              <thead>
                <tr>
                  <th class="border-[1px] border-yellow-600 px-3">S/N</th>
                  <th class="border-[1px] border-yellow-600 px-3">NAME</th>
                  <th class="border-[1px] border-yellow-600 px-3"></th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item, key in categories" :key="key">
                  <td class="border-[1px] border-yellow-600 text-center w-[100px]">{{ key+1 }}</td>
                  <td class="border-[1px] border-yellow-600 px-3 ellipsis w-full">{{ item.name }}</td>
                  <td class="border-[1px] border-yellow-600">
                    <button title="Delete item" @click="() => deleteCategory(item.id)" class="border-[1px] w-[50px] border-yellow-600 py-1 hover:bg-yellow-600 hover:text-stone-950 active:bg-yellow-700 flex items-center justify-center text-center">
                      <TrashIcon class="w-5 h-5" />
                    </button>
                  </td>
                </tr>
                <tr v-if="!categories.length">
                <td class="text-center py-5 px-2 border-[1px] border-yellow-600" colspan="3">No records found.</td>
                </tr>
              </tbody>
            </table>
          </div>
      </div>
    </div>
   </div>
</template>