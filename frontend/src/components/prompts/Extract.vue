<template>
  <div class="card floating" id="extract">
    <div class="card-title">
      <h2>{{ t("prompts.extract") }}</h2>
    </div>

    <div class="card-content">
      <p>{{ t("prompts.extractMessage", { name }) }}</p>

      <button
        id="focus-prompt"
        class="button button--block"
        :aria-label="$t('prompts.extractHere')"
        :title="$t('prompts.extractHere')"
        @click="confirm(false)"
      >
        <i class="material-icons">unarchive</i>
        {{ t("prompts.extractHere", { folder }) }}
      </button>

      <button
        class="button button--block"
        :aria-label="$t('prompts.extractToFolder')"
        :title="$t('prompts.extractToFolder')"
        @click="confirm(true)"
      >
        <i class="material-icons">create_new_folder</i>
        {{ t("prompts.extractToFolder", { folder: folderName }) }}
      </button>

      <p>
        <input id="extract-overwrite" v-model="overwrite" type="checkbox" />
        <label for="extract-overwrite">
          {{ t("prompts.extractOverwrite") }}
        </label>
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useLayoutStore } from "@/stores/layout";
import { archiveBaseName } from "@/utils/archive";
import { removeLastDir } from "@/utils/url";

const layoutStore = useLayoutStore();
const { t } = useI18n();

const name = computed(() => layoutStore.currentPrompt?.props?.name ?? "");
const path = computed(() => layoutStore.currentPrompt?.props?.path ?? "/");

// The new folder is named after the archive, minus the archive suffixes, so
// "backup.tar.gz" unpacks into "backup" rather than "backup.tar".
const folderName = computed(() => archiveBaseName(name.value));

// The folder the first option extracts into, spelled the way the user sees it
// in the address bar.
const folder = computed(
  () => `${removeLastDir(path.value)}/${folderName.value}`
);

const overwrite = ref(false);

// Which destination was picked, not what it is spelled: the caller knows the
// encoding the request needs, and does not need this prompt's opinion on it.
const confirm = (newFolder: boolean) => {
  layoutStore.currentPrompt?.confirm({ newFolder, overwrite: overwrite.value });
  layoutStore.closeHovers();
};
</script>
