<script setup lang="ts">
defineProps<{ type?: "reset" | "button" | "submit" | undefined; }>()
</script>
<template>
  <button class="h-[28px] w-full mb-2 bg-amber-500 hover:bg-amber-600 active:bg-amber-700 py-1 px-2 text-stone-950 focus:outline-none" :type="type || 'button'">
    <slot></slot>
  </button>
</template>