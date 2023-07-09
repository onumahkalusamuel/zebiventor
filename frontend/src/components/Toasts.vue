<script setup lang="ts">
// import { computed } from 'vue';
import Toast from './Toast.vue'
import { toasts } from '../stores/toasts';

// const items = computed(()=> toasts.toasts);

</script>

<template>
  <div class="fixed right-[15px] top-[15px] items-center flex flex-col" v-if="toasts.toasts.length">
    <Toast v-for="(toast, index) of toasts.toasts" :key="index" :toast="toast" />
  </div>
</template>

<style scoped>
</style>