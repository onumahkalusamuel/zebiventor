<script setup lang="ts">
defineProps<{
  name: string;
  label?: string;
  required?: boolean;
  class?: string;
  options?: [string, (string|number)?][];
  value?: string;
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
    <select class="h-[28px] w-full rounded py-1 px-2 bg-white hover:bg-gray-100 focus:outline-none" :name="name" :required="required" :autocomplete="`new-${name}`" :id="name">
      <option 
        v-for="opt,i in options" 
        :key="i" 
        :value="opt[1] || opt[0]"
        :selected="value==(opt[1] || opt[0])"
      >
        {{ opt[0] }}
      </option>
    </select>
    <div v-if="$slots.append" class="px-2 flex items-center justify-center hover:bg-gray-100 rounded">
      <slot name="append"></slot>
    </div>
  </div>
</template>