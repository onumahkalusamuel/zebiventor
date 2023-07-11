import { reactive } from 'vue';

export const auth = reactive({
  id: '',
  name: '',
  username: '',
  role: 0,
  timestamp: 0,
  reset() {
    this["id"] = this["name"] = this["username"] = ''
    this.role = this.timestamp = 0
  },
  updateTimestamp() {
    this.timestamp = Date.now();
  }
});