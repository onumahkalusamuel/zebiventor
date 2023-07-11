<script lang="ts" >
import { RouterView } from 'vue-router';
export default {
  name: 'MainLayout',
}; </script>;

<template>
  <div class="flex items-center flex-col justify-center h-screen bg-stone-950 text-yellow-600">
    <div class="py-[3rem] px-[2rem] inner-container border-[1px] border-yellow-600 text-center w-[400px]">
      <router-view></router-view>
    </div>
  </div>
</template>

<style>
</style>
