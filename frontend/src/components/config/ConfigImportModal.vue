<template>
  <AppModal v-model="isOpen" :title="t('importModal.title')" size="lg">
    <div class="space-y-4">
      <p class="text-sm text-muted-foreground">
        {{ t('importModal.hint') }}
      </p>
      <FileDropZone
        ref="zoneRef"
        :title="t('importModal.dropTitle')"
        :hint="t('importModal.dropHint')"
        @file="onFile"
        @clear="reset"
        @error="onZoneError"
      />
      <div class="flex flex-wrap items-center gap-2">
        <Button type="button" variant="ghost" size="sm" @click="pasteJson">
          <ClipboardPaste />
          {{ t('importModal.paste') }}
        </Button>
        <span class="text-xs text-muted-foreground">{{ t('importModal.orPaste') }}</span>
      </div>
      <div v-if="error" class="rounded-xl border border-destructive/20 bg-destructive/10 px-3 py-2 text-xs text-destructive">
        {{ error }}
      </div>
      <div v-else-if="preview" class="overflow-hidden rounded-xl bg-muted/40 ring-1 ring-black/[0.04] dark:ring-white/10">
        <div class="flex items-center justify-between gap-3 px-3 py-2">
          <p class="text-sm font-medium">{{ t('importModal.preview', { count: preview.total }) }}</p>
          <span class="text-xs text-muted-foreground">{{ t('importModal.noOverwrite') }}</span>
        </div>
        <div class="divide-y">
          <div
            v-for="(item, index) in visibleItems"
            :key="`${item.name}-${index}`"
            class="flex min-w-0 items-center gap-3 px-3 py-2"
          >
            <div class="flex size-7 shrink-0 items-center justify-center rounded-md bg-background text-sm">
              <ConfigIcon :value="item.icon" class="size-4" />
            </div>
            <div class="min-w-0 flex-1">
              <p class="truncate text-sm font-medium">{{ item.name }}</p>
              <p v-if="item.description" class="truncate text-xs text-muted-foreground">{{ item.description }}</p>
            </div>
            <span class="flex shrink-0 items-center gap-1 text-xs text-muted-foreground">
              <BrandIcon :provider="item.provider" class="size-3" />
              {{ providerLabel(item.provider) }}
            </span>
          </div>
        </div>
        <p v-if="hiddenCount > 0" class="px-3 py-2 text-xs text-muted-foreground">
          {{ t('importModal.more', { count: hiddenCount }) }}
        </p>
      </div>
    </div>
    <template #footer>
      <Button type="button" variant="outline" @click="isOpen = false">{{ t('common.cancel') }}</Button>
      <Button type="button" :disabled="importing || !payload" @click="submit">
        <Loader2 v-if="importing" class="animate-spin" />
        {{ t('importModal.submit') }}
      </Button>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { ClipboardPaste, Loader2 } from '@lucide/vue'
import { useConfigStore } from '@/stores/configStore'
import { useToast } from '@/composables/useToast'
import { useI18n } from '@/composables/useI18n'
import { parseConfigExport, providerLabel, type ImportPreview } from '@/lib/configImport'
import { ClipboardGetText } from '../../../wailsjs/runtime/runtime'
import AppModal from '@/components/common/AppModal.vue'
import FileDropZone from '@/components/common/FileDropZone.vue'
import BrandIcon from '@/components/common/BrandIcon.vue'
import ConfigIcon from '@/components/common/ConfigIcon.vue'
import { Button } from '@/components/ui/button'

const props = defineProps<{
  modelValue: boolean
  seed?: { name: string, text: string } | null
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const isOpen = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value),
})

const configStore = useConfigStore()
const toast = useToast()
const { t } = useI18n()
const payload = ref('')
const error = ref('')
const preview = ref<ImportPreview | null>(null)
const importing = ref(false)
const zoneRef = ref<{ applyFile: (file: { name: string, text: string, size?: number }, silent?: boolean) => Promise<void>, clear: () => void } | null>(null)

const visibleItems = computed(() => preview.value?.items.slice(0, 8) || [])
const hiddenCount = computed(() => Math.max(0, (preview.value?.total || 0) - visibleItems.value.length))

watch(isOpen, (open) => {
  if (!open) {
    reset()
    zoneRef.value?.clear()
    return
  }
  if (props.seed?.text) void applySeed(props.seed)
})

watch(() => props.seed, (seed) => {
  if (!isOpen.value) return
  if (seed?.text) {
    void applySeed(seed)
    return
  }
  reset()
  zoneRef.value?.clear()
})

function reset() {
  payload.value = ''
  error.value = ''
  preview.value = null
}

function setPayload(text: string) {
  error.value = ''
  const parsed = parseConfigExport(text)
  if (parsed.error) {
    error.value = parsed.error
    preview.value = null
    payload.value = ''
    return false
  }
  payload.value = text.replace(/^\uFEFF/, '')
  preview.value = parsed.preview
  return true
}

async function applySeed(seed: { name: string, text: string }) {
  if (!setPayload(seed.text)) return
  await nextTick()
  zoneRef.value?.applyFile({ name: seed.name, text: seed.text, size: seed.text.length }, true)
}

function onFile(file: { name: string, text: string }) {
  setPayload(file.text)
}

function onZoneError(message: string) {
  error.value = message
  preview.value = null
  payload.value = ''
}

async function pasteJson() {
  try {
    let text = ''
    try {
      text = await ClipboardGetText()
    } catch {
      text = await navigator.clipboard.readText()
    }
    if (!text.trim()) {
      error.value = t('importModal.emptyClipboard')
      return
    }
    await applySeed({ name: 'clipboard.json', text })
  } catch {
    error.value = t('importModal.clipboardFail')
  }
}

async function submit() {
  if (!payload.value || importing.value) return
  importing.value = true
  try {
    const count = await configStore.importConfigJSON(payload.value)
    if (count > 0) {
      toast.success(t('toast.imported', { count }))
      isOpen.value = false
    } else {
      toast.error(t('toast.importFailed', { error: t('importModal.emptyResult') }))
    }
  } catch (e: unknown) {
    toast.error(t('toast.importFailed', { error: e instanceof Error ? e.message : String(e) }))
  } finally {
    importing.value = false
  }
}
</script>
