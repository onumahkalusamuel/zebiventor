<script setup lang="ts">
import { RouterLink, RouteLocationRaw } from 'vue-router'

export interface BreadcrumbItem {
  title: string,
  link?: RouteLocationRaw,
  current?: boolean
}

defineProps<{ items: BreadcrumbItem[] }>()
</script>

<template>
  <div class="breadcrumbs-container">
    <div class="breadcrumb-item-container" v-for="(item, i) in items" :key="i">
      <router-link 
        class="breadcrumb-item" 
        :class="item.current?'breadcrumb-current':'breakcrumb-link'" 
        :to="item.current?'javascript:void(0)':(item.link ?? '')"
      >{{item.title}}</router-link>
      <div class="separator-container" v-if="i < items.length-1">
        <img src="/icons/chevron-right.svg" alt="separator" class="separator" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.breadcrumbs-container {
  height: 36px;
  display: flex;
  align-items: center;
  padding: 0 15px;
  color: #333;
  flex: 0 0 auto;
}
.breadcrumb-item {
  height: 100%;
  text-decoration: none;
  color: #0078d4;
}
.breadcrumb-item-container {
  display: flex;
  align-items: center;
  justify-content: center;
}
.separator-container {
  height: 100%;
  padding: 0 9px;
  display: flex;
  align-items: center;
  justify-content: center;
}
.breadcrumb-current {
  color: inherit;
  cursor: default;
}
.breakcrumb-link:hover{
  text-decoration: underline;

}
</style>