<script setup lang="ts">
defineProps<{ type?: "reset" | "button" | "submit" | undefined; }>()
</script>
<template>
  <button class="h-[28px] w-full mb-2 bg-red-600 hover:bg-red-700 active:bg-red-800 py-1 px-2 text-white rounded" :type="type || 'button'">
    <slot></slot>
  </button>
</template>