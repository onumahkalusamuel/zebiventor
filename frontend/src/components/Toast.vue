<script setup lang="ts">
import { onMounted } from 'vue';
import { toasts } from '../stores/toasts';
import { Toast } from '../types';
import { XMarkIcon } from '@heroicons/vue/24/solid'

const props = 
defineProps<{ toast: Toast }>()

const types = {
  success: 'border-green-500 bg-green-50',
  error: 'border-red-500 bg-red-50',
  warning: 'border-yellow-500 bg-yellow-50',
  info: 'border-blue-500 bg-blue-50',
  neutral: 'border-gray-500 bg-white'
}

const clearToast = () => toasts.clearToast(props.toast.id as string);

onMounted(() => setTimeout(clearToast, 5000));

</script>
<template>
  <div :class="types[toast.type]" class="w-[500px] min-h-[50px] flex flex-col mb-2 p-2 justify-center border-[1px] transition-opacity ease-in duration-700 opacity-100">
    <div class="header flex justify-between items-center w-full" v-if="toast.title">
      <div class="text-[.9rem]">{{ toast.title }}</div>
      <XMarkIcon class="w-4 h-4 p-[1px] cursor-pointer" @click="clearToast"/>
    </div>
    <div class="my-2 border-t-[1px] border-gray-300" v-if="toast.title"></div>
    <div class="content flex justify-between">
      <p class="">{{ toast.message }}</p>
      <XMarkIcon class="w-4 h-4 p-[1px] cursor-pointer" @click="clearToast" v-if="!toast.title"/>
    </div>
  </div>
</template>