<script lang="ts" setup>
import { RouterView, RouterLink, useRouter } from 'vue-router';
import { auth, store } from '../stores';
import {
  ListBulletIcon,
  UsersIcon,
  SparklesIcon,
  UserGroupIcon,
  Cog8ToothIcon,
  ShoppingBagIcon,
  Square3Stack3DIcon,
  LockClosedIcon,
} from '@heroicons/vue/24/outline';
import { StoreDetails } from '../../wailsjs/go/controllers/General'
import { onMounted } from 'vue';

const router = useRouter();

onMounted(async() => {
  try {
    const req = await StoreDetails();
    console.log(req);
    if (req) store.setAll(req);
  }
  catch (e) {
    console.log(e);
  }
})

const logout = async () => {
  auth.reset();
  router.push({name:'login'});
}
</script>;

<template>
  <div class="flex flex-col h-screen bg-stone-950 text-yellow-600">
    <header class="w-full flex toolbar bg-yellow-600 h-[40px] text-stone-950 justify-between">
      <div class="flex">
        <router-link class="header-icon-link font-bold text-[18px] px-3 hover:bg-yellow-700" :to="{name: 'dashboard'}">
          <SparklesIcon class="h-6 w-6 mr-2" />
          {{ store.name }} <small class="opacity-[.7]">&nbsp[{{ auth.username }}]</small>
        </router-link>   
      </div>
      <div style="display:flex; flex: 0 0 auto">
        <router-link title="Staff" class="header-icon-link px-3 hover:bg-yellow-700" :to="{name: 'staff'}">
          <users-icon class="h-5 w-5"/>
          <span class="pl-2 hidden lg:block">Staff</span>
        </router-link>
        <router-link title="Customers" class="header-icon-link px-3 hover:bg-yellow-700" :to="{name: 'customers'}">
          <user-group-icon class="h-5 w-5"/>
          <span class="pl-2 hidden lg:block">Customers</span>
        </router-link>
        <router-link title="Categories" class="header-icon-link px-3 hover:bg-yellow-700" :to="{name: 'categories'}">
          <list-bullet-icon class="h-5 w-5"/>
          <span class="pl-2 hidden lg:block">Categories</span>
        </router-link>
        <router-link title="Products" class="header-icon-link px-3 hover:bg-yellow-700" :to="{name: 'products'}">
          <square3-stack3-d-icon class="h-5 w-5"/>
          <span class="pl-2 hidden lg:block">Products</span>
        </router-link>
        <router-link title="Sales" class="header-icon-link px-3 hover:bg-yellow-700" :to="{name: 'sales'}">
          <shopping-bag-icon class="h-5 w-5"/>
          <span class="pl-2 hidden lg:block">Sales</span>
        </router-link>
      </div>
      <div class="flex">
        <router-link title="Settings" class="header-icon-link px-3 hover:bg-yellow-700" :to="{name: 'settings'}">
          <cog8-tooth-icon class="h-5 w-5"/>
          <span class="pl-2 hidden lg:block">Settings</span>
        </router-link>
        <a class="header-icon-link px-3 hover:bg-yellow-700" @click="logout">
          <lock-closed-icon class="h-5 w-5 mr-2"/>
          <span>Logout</span>
        </a>
      </div>
    </header>
    <main class="flex-1 h-[calc(100vh-70px)] overflow-scroll no-scrollbar">
      <router-view></router-view>
    </main>
    <footer class="w-full flex border-t-2 border-yellow-600 h-[30px] text-yellow-600 text-sm justify-center items-center">  
      <router-link class="font-bold text-[12px] px-3 hover:bg-stone-800" to="mailto:zebitechprojects@gmail.com">
        ZEBINV 1.0.0
      </router-link>
    </footer>
  </div>
</template>

<style scoped>
.toolbar {
  flex: 0 0 auto;
  box-sizing: border-box;
}
.header-icon-link {
  height: 40px;
  display:flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  text-decoration: none;
  cursor:pointer;
}
.search-container {
  margin-right: 15px;
  min-width: 200px;
  margin-left: 7px;
  display: flex;
  flex-grow: 1;
  align-items: center;
  justify-content: center;
}

.avatar {
  height: 28px;
  width: 28px;
  border-radius: 28px;
  border:0;
}

.avatarmenu-username {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 14px;
  line-height: normal;
  max-width: 160px;
}
.avatarmenu-userid {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  font-weight: 600;
  font-size: 10px;
  line-height: normal;
  max-width: 160px;
}
.avatarmenu-image-container {
  height: 28px;
  width: 28px;
  flex: 0 0 auto;
  padding-left: 6px;
  padding-right: 6px;
  box-sizing: content-box;
}
</style>