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
import Sales from '../pages/app/Sales.vue';
import SalesPreview from '../pages/app/SalesPreview.vue';
import Customers from '../pages/app/Customers.vue';
import Staff from '../pages/app/Staff.vue';

// import Patients from '../pages/app/patients/Index.vue';
// import AddPatient from '../pages/app/patients/Add.vue';
// import ViewPatient from '../pages/app/patients/View.vue';
// import NextOfKin from '../pages/app/patients/NextOfKin.vue';
// import PatientHistory from '../pages/app/patients/HistoryIndex.vue';
// import PatientHistoryAdd from '../pages/app/patients/HistoryAdd.vue';
// import PatientHistoryView from '../pages/app/patients/HistoryView.vue';

// import Deliveries from '../pages/app/deliveries/Index.vue';
// import AddDelivery from '../pages/app/deliveries/Add.vue';
// import ViewDelivery from '../pages/app/deliveries/View.vue';

// import Staff from '../pages/app/staff/Index.vue';
// import AddStaff from '../pages/app/staff/Add.vue';
// import ViewStaff from '../pages/app/staff/View.vue';

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
      { path: 'sales', component: Sales, name: 'sales' },
      { path: 'sales-preview/:id', component: SalesPreview, name: 'sales-preview' },
      { path: 'customers', component: Customers, name: 'customers' },
      { path: 'staff', component: Staff, name: 'staff' },
      { path: ':pathMatch(.*)', redirect: { name: 'dashboard' } },
    ]
  },
  { path: '/:pathMatch(.*)*', redirect: { name: 'login' } },
];