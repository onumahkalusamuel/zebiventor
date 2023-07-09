<script setup lang="ts">
import { XMarkIcon } from '@heroicons/vue/24/solid'
import {popupStore} from '../stores';

defineProps<{ id: string; title: string }>();
const close = () => {popupStore.id = ''; popupStore.show = false;}
</script>
<template>
  <div class="bg-[#00000043] h-screen absolute top-0 w-full flex justify-center items-center" v-if="popupStore.show && popupStore.id == id">
    <div class="bg-white w-[500px] min-h-[50px] flex flex-col mb-2 p-2 justify-center border-[1px] transition-opacity ease-in duration-700 opacity-100">
      <div class="header flex justify-between items-center w-full">
        <div class="text-[.9rem]">{{ title }}</div>
        <XMarkIcon class="w-4 h-4 p-[1px] cursor-pointer" @click="close"/>
      </div>
      <div class="my-2 border-t-[1px] border-gray-300"></div>
      <div class="">
        <slot></slot>
      </div>
    </div>
  </div>
</template>