<script lang="ts" setup>
import { onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import TextField from '../components/form/TextField.vue';
import { DocumentDuplicateIcon } from '@heroicons/vue/24/solid'
import PrimaryButton from '../components/form/PrimaryButton.vue';
import { toasts } from '../stores/toasts';
import { Setup, InstallationCode } from '../../wailsjs/go/controllers/General'
import { controllers } from '../../wailsjs/go/models'
const installation_code = ref('');
const router = useRouter();
const form = ref({address: '', email: '', logo: '', name: '', phone: ''} as controllers.SetupRequest);
const installationCodeField = ref(null);

onMounted(async() => { installation_code.value = await InstallationCode();})

const copyInstallationCode = async () => {
  await navigator.clipboard.writeText(installation_code.value as string);
  toasts.addToast({message: 'Installation code copied to clipboard.', type: 'success'});
}

const activate = async () => {
  const activate = await Setup(form.value)
  if(activate.message) {
    toasts.addToast({message: activate.message, type: 'success'});
    router.push({name: 'create-admin'})
  }
}
</script>;

<template>
  <p class="text-xl">Activate ZebiVentor to continue</p>
  <form v-on:submit.prevent="activate">
    <div>
      <div class="w-full mb-3 flex items-center">
        <TextField ref="installationCodeField" readonly :value="installation_code" class="w-full">
          <template #append>
            <button class="w-5 h-5" type="button" v-on:click.stop.prevent="copyInstallationCode">
              <DocumentDuplicateIcon class="w-5 h-5" />
            </button>
          </template>
        </TextField>
      </div>
      <TextField name="activation_code" placeholder="Activation Code" class="w-full mb-2" />
      <PrimaryButton class="w-full mb-2" type="submit">Activate</PrimaryButton>
    </div>
  </form>
  <small>Copy the installation code above and forward to software team for activation.</small>
</template>
