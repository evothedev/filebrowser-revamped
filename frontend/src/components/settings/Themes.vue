<template>
  <fieldset class="themes">
    <legend class="themes__legend">{{ t("settings.themes.title") }}</legend>

    <label
      v-for="option in options"
      :key="option.name"
      class="theme-card"
      :class="{ 'theme-card--selected': option.name === theme }"
    >
      <input
        class="themes__radio"
        type="radio"
        name="theme"
        :value="option.name"
        :checked="option.name === theme"
        @change="select(option.name)"
      />

      <span class="theme-card__preview" :style="previewStyle(option)">
        <span
          class="theme-card__bar"
          :style="{ background: option.swatch.surface }"
        >
          <span
            class="theme-card__dot"
            :style="{ background: option.swatch.accent }"
          ></span>
        </span>
        <span class="theme-card__body">
          <span
            class="theme-card__line"
            :style="{ background: option.swatch.text }"
          ></span>
          <span
            class="theme-card__line theme-card__line--short"
            :style="{ background: option.swatch.text }"
          ></span>
        </span>
      </span>

      <span class="theme-card__label">{{ option.label }}</span>
    </label>
  </fieldset>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { getMediaPreference } from "@/utils/theme";
import { getThemeDefinition, THEMES, type Theme } from "@/utils/themes";

const { t } = useI18n();

defineProps<{
  theme: UserTheme;
}>();
const emit = defineEmits<{
  (e: "update:theme", val: UserTheme): void;
}>();

interface ThemeOption {
  name: UserTheme;
  label: string;
  swatch: Theme["swatch"];
}

// "System default" has no palette of its own — it renders whichever theme the
// operating system asks for — so it borrows that theme's swatch to show what
// picking it will actually look like.
const options = computed<ThemeOption[]>(() => {
  const system = getThemeDefinition(getMediaPreference());

  return [
    {
      name: "",
      label: t("settings.themes.default"),
      swatch: system?.swatch ?? THEMES[0].swatch,
    },
    ...THEMES.map((theme) => ({
      name: theme.name as UserTheme,
      label: t(`settings.themes.${theme.name}`),
      swatch: theme.swatch,
    })),
  ];
});

const select = (name: UserTheme) => {
  emit("update:theme", name);
};

const previewStyle = (option: ThemeOption) => ({
  background: option.swatch.background,
  borderColor: option.swatch.text,
});
</script>

<style scoped>
.themes {
  border: 0;
  margin: 0;
  padding: 0;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(7.5em, 1fr));
  gap: 0.75em;
}

.themes__legend {
  padding: 0 0 0.5em;
  font-weight: 500;
}

/* The radio is what makes this a real radio group — arrow keys, focus and
   screen readers come for free — so it stays in the DOM, just not on screen. */
.themes__radio {
  position: absolute;
  width: 1px;
  height: 1px;
  margin: -1px;
  padding: 0;
  overflow: hidden;
  clip: rect(0 0 0 0);
  white-space: nowrap;
  border: 0;
}

.theme-card {
  display: block;
  padding: 0.4em;
  cursor: pointer;
  border: 1px solid var(--borderPrimary);
  border-radius: 0.3em;
  background: var(--surfacePrimary);
  transition: 0.1s ease all;
}

.theme-card:hover {
  border-color: var(--borderSecondary);
}

.theme-card--selected {
  border-color: var(--blue);
  box-shadow: 0 0 0 1px var(--blue);
}

.themes__radio:focus-visible + .theme-card__preview {
  outline: 2px solid var(--blue);
  outline-offset: 1px;
}

.theme-card__preview {
  display: block;
  border: 1px solid;
  border-radius: 0.2em;
  overflow: hidden;
  aspect-ratio: 4 / 3;
}

.theme-card__bar {
  display: flex;
  align-items: center;
  height: 30%;
  padding-left: 0.5em;
}

.theme-card__dot {
  display: block;
  width: 0.5em;
  height: 0.5em;
  border-radius: 50%;
}

.theme-card__body {
  display: block;
  padding: 0.5em;
}

.theme-card__line {
  display: block;
  height: 0.25em;
  border-radius: 0.125em;
  opacity: 0.6;
  margin-bottom: 0.35em;
}

.theme-card__line--short {
  width: 60%;
  opacity: 0.35;
}

.theme-card__label {
  display: block;
  margin-top: 0.5em;
  font-size: 0.8rem;
  color: var(--textSecondary);
  text-align: center;
}
</style>
