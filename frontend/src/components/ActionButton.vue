<script setup lang="ts">
import { FunctionalComponent, HTMLAttributes, VNodeProps } from 'vue';

defineProps<{
  iconSrc?: FunctionalComponent<HTMLAttributes & VNodeProps>;
  disabled?: boolean;
  outline?: boolean;
  classes?: string;
  iconClass?: string;
}>()
</script>
<template>
  <div style="display:inline-block">  
    <div class='cursor-pointer flex items-center h-[36px] px-[4px] hover:bg-[azure] text-[#333]' :class="( disabled ?' disabled ':' ') + (outline ? ' outline ':' ') +(classes ? classes : '')">
      <div class="text-[#0078d4] flex items-center px-[4px]" v-if="iconSrc">
        <component :is="iconSrc" :class="iconClass" class="w-[16px] h-[16px]" />
      </div>
      <div class="px-[4px]"><slot></slot></div>
    </div>
  </div>
</template>
<style scoped>
.disabled {
  cursor: default !important;
  opacity: 0.5;
}
.outline {
  border: 1px solid #555;
  box-sizing: border-box;
}
</style>
