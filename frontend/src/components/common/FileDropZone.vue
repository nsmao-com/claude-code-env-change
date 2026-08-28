<template>
  <div
    class="relative flex cursor-pointer flex-col items-center justify-center gap-3 rounded-xl border-2 border-dashed px-6 text-center transition-colors"
    :class="[
      compact ? 'py-7' : 'py-11',
      dragging
        ? 'border-brand bg-brand/10'
        : fileName
          ? 'border-brand/40 bg-brand/5'
          : 'border-border bg-muted/30 hover:border-brand/40 hover:bg-muted/50',
    ]"
    style="--wails-drop-target: drop"
    @click="openPicker"
    @dragenter.prevent="onEnter"
    @dragover.prevent="dragging = true"
    @dragleave.prevent="onLeave"
    @drop.prevent.stop="onDrop"
  >
    <input
      ref="inputRef"
      type="file"
      class="sr-only"
      :accept="accept"
      @change="onPick"
      @click.stop
    >
    <div
      class="flex size-12 items-center justify-center rounded-full bg-background ring-1 ring-black/[0.06] dark:ring-white/10"
      :class="fileName || dragging ? 'text-brand' : 'text-muted-foreground'"
    >
      <FileJson v-if="fileName" class="size-5" />
      <Upload v-else class="size-5" />
    </div>
    <div class="space-y-1">
      <p class="text-sm font-medium">
        {{ heading }}
      </p>
      <p class="text-xs text-muted-foreground">
        {{ fileName ? metaLabel : hint }}
      </p>
    </div>
    <div class="flex flex-wrap items-center justify-center gap-2">
      <Button type="button" variant="outline" size="sm" @click.stop="openPicker">
        {{ fileName ? t('importModal.replace') : t('importModal.choose') }}
      </Button>
      <Button v-if="fileName" type="button" variant="ghost" size="sm" @click.stop="clear">
        <X />
        {{ t('importModal.clear') }}
      </Button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { FileJson, Upload, X } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { useI18n } from '@/composables/useI18n'

const { t } = useI18n()

const props = withDefaults(defineProps<{
  accept?: string
  title?: string
  hint?: string
  compact?: boolean
  maxBytes?: number
}>(), {
  accept: '.json,application/json,text/plain',
  title: '拖拽文件到这里',
  hint: '或点击选择 JSON 文件',
  compact: false,
  maxBytes: 8 * 1024 * 1024,
})

const emit = defineEmits<{
  file: [payload: { name: string, text: string, size: number }]
  clear: []
  error: [message: string]
}>()

const dragging = ref(false)
const fileName = ref('')
const fileSize = ref(0)
const inputRef = ref<HTMLInputElement | null>(null)

const heading = computed(() => {
  if (dragging.value) return t('importModal.release')
  return fileName.value || props.title
})

const metaLabel = computed(() => formatSize(fileSize.value))

function formatSize(n: number) {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / (1024 * 1024)).toFixed(1)} MB`
}

function openPicker() {
  inputRef.value?.click()
}

function onEnter() {
  dragging.value = true
}

function onLeave(event: DragEvent) {
  const next = event.relatedTarget as Node | null
  if (next && (event.currentTarget as Node).contains(next)) return
  dragging.value = false
}

async function applyFile(file: { name: string, text: string, size?: number }, silent = false) {
  fileName.value = file.name
  fileSize.value = file.size ?? file.text.length
  if (!silent) emit('file', { name: file.name, text: file.text, size: fileSize.value })
}

async function readFile(file: File) {
  if (file.size > props.maxBytes) {
    emit('error', t('importModal.tooBig'))
    return
  }
  const text = await file.text()
  await applyFile({ name: file.name, text, size: file.size })
}

async function onDrop(event: DragEvent) {
  dragging.value = false
  const file = event.dataTransfer?.files?.[0]
  if (!file) return
  await readFile(file)
}

async function onPick(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  await readFile(file)
}

function clear() {
  fileName.value = ''
  fileSize.value = 0
  emit('clear')
}

defineExpose({ applyFile, clear })
</script>
