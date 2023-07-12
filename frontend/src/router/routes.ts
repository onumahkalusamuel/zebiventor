import { RouteRecordRaw } from 'vue-router';
import { appGuard, softwareGuard } from '../guards';

import MainLayout from '../layouts/MainLayout.vue';
import AppLayout from '../layouts/AppLayout.vue';

import Activate from '../pages/Activate.vue';
import CreateAdmin from '../pages/CreateAdmin.vue';
import StoreSetup from '../pages/StoreSetup.vue';
import Login from '../pages/Login.vue';

import Dashboard from '../pages/app/Dashboard.vue';
import Categories from '../pages/app/Categories.vue';
import Products from '../pages/app/Products.vue';
import UpdateProduct from '../pages/app/UpdateProduct.vue';
import Sales from '../pages/app/Sales.vue';
import SalesPreview from '../pages/app/SalesPreview.vue';
import Customers from '../pages/app/Customers.vue';
import Staff from '../pages/app/Staff.vue';
import Settings from '../pages/app/Settings.vue';

export const routes: RouteRecordRaw[] = [
  {
    path: '/', component: MainLayout, beforeEnter: softwareGuard, children: [
      { path: '', redirect: { name: 'login' } },
      { path: 'activate', component: Activate, name: 'activate' },
      { path: 'create-admin', component: CreateAdmin, name: 'create-admin' },
      { path: 'store-setup', component: StoreSetup, name: 'store-setup' },
    ]
  },
  { path: '/login', beforeEnter: softwareGuard, component: Login, name: 'login' },
  {
    path: '/app/', component: AppLayout, beforeEnter: [appGuard, softwareGuard], children: [
      { path: 'dashboard', component: Dashboard, name: 'dashboard' },
      { path: 'categories', component: Categories, name: 'categories' },
      { path: 'products', component: Products, name: 'products' },
      { path: 'products/:id', component: UpdateProduct, name: 'update-product' },
      { path: 'sales', component: Sales, name: 'sales' },
      { path: 'sales-preview/:id', component: SalesPreview, name: 'sales-preview' },
      { path: 'customers', component: Customers, name: 'customers' },
      { path: 'staff', component: Staff, name: 'staff' },
      { path: 'settings', component: Settings, name: 'settings' },
      { path: ':pathMatch(.*)', redirect: { name: 'dashboard' } },
    ]
  },
  { path: '/:pathMatch(.*)*', redirect: { name: 'login' } },
];