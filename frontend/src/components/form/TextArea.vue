<script setup lang="ts">

defineProps<{
  ref?: string;
  name?: string;
  placeholder?: string;
  type?: string;
  required?: boolean;
  class?: string;
  label?: string;
  value?: string;
  readonly?: boolean;
  rows?: string;
}>()
</script>
<template>
  <label v-if="label" class="text-uppercase cursor-pointer py-1 block" :for="name">
    {{ label }}
    <span v-if="required" class="text-red-600">*</span>
  </label>
  <div class="flex w-full border-[1px] border-[#888] rounded flex items-center justify-center bg-white hover:bg-gray-100" :class="class">
    <div v-if="$slots.prepend" class="px-2 flex items-center justify-center hover:bg-gray-100 rounded">
      <slot name="prepend"></slot>
    </div>
    <textarea :rows="rows" class="w-full rounded py-1 px-2 hover:bg-gray-100 focus:outline-none" :name="name" :placeholder="placeholder" :type="type || 'text'" :required="required" :autocomplete="`new-${name}`" :id="name" :readonly="readonly" :ref="ref">{{ value }}</textarea>
    <div v-if="$slots.append" class="px-2 flex items-center justify-center hover:bg-gray-100 rounded">
      <slot name="append"></slot>
    </div>
  </div>
</template>