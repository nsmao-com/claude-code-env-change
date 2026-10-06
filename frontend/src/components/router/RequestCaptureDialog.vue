<template>
  <AppModal v-model="isOpen" size="xl" :title="t('router.capture.title')">
    <p v-if="loading" class="text-xs text-muted-foreground">{{ t('router.capture.loading') }}</p>
    <p v-else-if="error" role="alert" class="text-xs text-destructive">{{ error }}</p>
    <template v-else-if="item">
      <p class="mb-3 font-mono text-[11px] text-muted-foreground">
        {{ item.time }} · {{ item.method }} {{ item.path }} · HTTP {{ item.status || '—' }}
      </p>
      <SegmentedPills v-model="tab" layout-id="capture-tab" dense :items="tabs" />
      <div class="mt-3 space-y-3">
        <section>
          <div class="mb-1 flex items-center justify-between">
            <p class="text-xs font-medium">{{ t('router.capture.headers') }}</p>
          </div>
          <pre class="max-h-40 overflow-auto rounded-lg bg-muted p-3 font-mono text-[11px] leading-relaxed whitespace-pre-wrap break-all">{{ headersText }}</pre>
        </section>
        <section>
          <div class="mb-1 flex items-center justify-between gap-2">
            <p class="text-xs font-medium">
              {{ t('router.capture.body') }}
              <span v-if="truncated" class="ml-1 font-normal text-amber-600">{{ t('router.capture.truncated') }}</span>
            </p>
            <Button variant="ghost" size="sm" class="h-7 px-2 text-xs" :disabled="!bodyText" @click="copy(bodyText)">
              <Copy class="size-3.5" />{{ t('router.capture.copy') }}
            </Button>
          </div>
          <pre class="max-h-[46vh] overflow-auto rounded-lg bg-muted p-3 font-mono text-[11px] leading-relaxed whitespace-pre-wrap break-all">{{ bodyText || t('router.capture.empty') }}</pre>
        </section>
      </div>
      <p class="mt-3 text-[11px] text-muted-foreground">{{ t('router.capture.note') }}</p>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Copy } from '@lucide/vue'
import { useI18n } from '@/composables/useI18n'
import { useToast } from '@/composables/useToast'
import { callService } from '@/services/appBridge'
import type { RequestCapture } from '@/types'
import AppModal from '@/components/common/AppModal.vue'
import SegmentedPills from '@/components/layout/SegmentedPills.vue'
import { Button } from '@/components/ui/button'

const props = defineProps<{ modelValue: boolean; requestId: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean] }>()
const { t } = useI18n()
const toast = useToast()

const isOpen = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v),
})
const item = ref<RequestCapture | null>(null)
const loading = ref(false)
const error = ref('')
const tab = ref('request')
const tabs = computed(() => [
  { value: 'request', label: t('router.capture.request') },
  { value: 'response', label: t('router.capture.response') },
])

watch(
  () => [props.modelValue, props.requestId] as const,
  async ([open, id]) => {
    if (!open || !id) return
    loading.value = true
    error.value = ''
    item.value = null
    tab.value = 'request'
    try {
      item.value = await callService<RequestCapture>('RouterService', 'GetRequestCapture', id)
    } catch (e) {
      error.value = e instanceof Error ? e.message : String(e)
    } finally {
      loading.value = false
    }
  },
  { immediate: true },
)

function pretty(text: string) {
  const trimmed = text.trim()
  if (!trimmed.startsWith('{') && !trimmed.startsWith('[')) return text
  try {
    return JSON.stringify(JSON.parse(trimmed), null, 2)
  } catch {
    return text
  }
}

const headersText = computed(() => {
  const h = tab.value === 'request' ? item.value?.request_headers : item.value?.response_headers
  return Object.entries(h || {})
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([k, v]) => `${k}: ${v}`)
    .join('\n')
})
const bodyText = computed(() => {
  if (!item.value) return ''
  return pretty(tab.value === 'request' ? item.value.request_body : item.value.response_body)
})
const truncated = computed(() => (tab.value === 'request' ? item.value?.request_truncated : item.value?.response_truncated))

async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    toast.success(t('router.capture.copied'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  }
}
</script>
