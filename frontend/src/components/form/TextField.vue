<script setup lang="ts">

const props = defineProps<{
  ref?: string;
  name?: string;
  placeholder?: string;
  type?: string;
  required?: boolean;
  class?: string;
  label?: string;
  readonly?: boolean;
  step?: string;
  disabled?: boolean;
  modelValue?: String | Number
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
  <div class="flex w-full border-[1px] border-yellow-600 flex items-center justify-center hover:bg-stone-900" :class="class">
    <div v-if="$slots.prepend" class="px-2 flex items-center justify-center hover:bg-stone-900">
      <slot name="prepend"></slot>
    </div>
    <input class="bg-stone-900 h-[28px] w-full py-1 px-2 hover:bg-stone-800 focus:bg-stone-800 focus:outline-none" :name="name" :placeholder="placeholder" :type="type || 'text'" :required="required" :autocomplete="`new-${name}`" :id="name" :value="modelValue" :readonly="readonly" :step="step" :ref="ref" @input="updateValue" :disabled="disabled"/>
    <div v-if="$slots.append" class="px-2 flex items-center justify-center hover:bg-stone-900">
      <slot name="append"></slot>
    </div>
  </div>
</template>