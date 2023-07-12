<script lang="ts" setup>
import { onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import TextField from '../components/form/TextField.vue';
import { DocumentDuplicateIcon } from '@heroicons/vue/24/solid'
import PrimaryButton from '../components/form/PrimaryButton.vue';
import { toasts } from '../stores/toasts';
import { Activate, InstallationCode, GetActivationCode } from '../../wailsjs/go/controllers/General'
import { controllers } from '../../wailsjs/go/models'
const installation_code = ref('');
const router = useRouter();
const form = ref({activation_code: ''} as controllers.ActivateRequest);
const installationCodeField = ref(null);

onMounted(async() => {
  installation_code.value = await InstallationCode();
  // const activation_code = await GetActivationCode({ installation_code: installation_code.value });
  // form.value.activation_code = activation_code.activation_code
})

const copyInstallationCode = async () => {
  await navigator.clipboard.writeText(installation_code.value as string);
  toasts.addToast({message: 'Installation code copied to clipboard.', type: 'success'});
}

const activate = async () => {
  const activate = await Activate(form.value)
  if(!activate.success) {
    toasts.addToast({message: activate.message, type: 'error'});
    return;
  }
  toasts.addToast({message: activate.message, type: 'success'});
  router.push({name: 'create-admin'})
}
</script>;

<template>
  <p class="text-xl">Activate ZebiVentor</p>
  <form v-on:submit.prevent="activate" class="my-5">
    <div>
      <div class="w-full mb-3 flex items-center">
        <div class="w-full text-left">
          <TextField ref="installationCodeField" readonly v-model="installation_code" class="w-full">
            <template #append>
              <button class="w-5 h-5" type="button" v-on:click.stop.prevent="copyInstallationCode">
                <DocumentDuplicateIcon class="w-5 h-5" />
              </button>
            </template>
          </TextField>
        </div>
      </div>
      <div class="w-full text-left">
        <TextField v-model="form.activation_code" name="activation_code" placeholder="Activation Code" class="w-full mb-2" />
      </div>
      <PrimaryButton class="w-full mb-2" type="submit">Activate</PrimaryButton>
    </div>
  </form>
  <small>Use installation code above to generate activation code.</small>
</template>
