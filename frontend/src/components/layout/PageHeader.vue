<template>
  <div class="flex items-end justify-between gap-4">
    <div class="min-w-0">
      <div class="flex items-center gap-2.5">
        <h1 class="text-[2.5rem] leading-none font-semibold tracking-tight">
          {{ title }}
        </h1>
        <slot name="title-extra" />
      </div>
      <p v-if="description" class="mt-2 text-sm text-muted-foreground">
        {{ description }}
      </p>
      <slot />
    </div>
    <div class="flex shrink-0 flex-wrap items-center justify-end gap-2 pb-0.5">
      <slot name="actions" />
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  title: string
  description?: string
}>()
</script>
