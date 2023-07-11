<script setup lang="ts">
defineProps<{
  name?: string;
  label?: string;
  required?: boolean;
  class?: string;
  options?: [string, (string|number)?][];
  modelValue?: String
}>()

const emit = defineEmits(['update:modelValue'])

const updateValue = (event: any) => {
    emit('update:modelValue', event.target.value)
}


</script>
<template>
  <label v-if="label" class="text-uppercase cursor-pointer py-1 block" :for="name">
    {{ label }}
    <span v-if="required" class="text-red-600">*</span>
  </label>
  <div class="flex w-full border-[1px] border-yellow-600 flex items-center justify-center bg-stone-900 hover:bg-stone-900" :class="class">
    <div v-if="$slots.prepend" class="px-2 flex items-center justify-center hover:bg-stone-900">
      <slot name="prepend"></slot>
    </div>
    <select :value="modelValue" class="h-[28px] w-full py-1 px-2 bg-stone-900 hover:bg-stone-800 focus:bg-stone-800 focus:outline-none" :name="name" :required="required" :autocomplete="`new-${name}`" :id="name" @input="updateValue">
      <option 
        v-for="opt,i in options" 
        :key="i" 
        :value="opt[1] || opt[0]"
        :selected="modelValue==(opt[1] || opt[0])"
      >
        {{ opt[0] }}
      </option>
    </select>
    <div v-if="$slots.append" class="px-2 flex items-center justify-center hover:bg-stone-900">
      <slot name="append"></slot>
    </div>
  </div>
</template>