import { reactive } from 'vue';
import { Toast } from '../types';
import { v4 as uuidv4 } from 'uuid';

export const toasts = reactive({
  toasts: [] as Toast[],
  addToast (toast: Toast) { this.toasts.push({ ...toast, id: uuidv4() }); },
  clearToast (title: string) {
    const index = this.toasts.findIndex((toast) => toast.title === title);
    this.toasts.splice(index, 1);
  },
});