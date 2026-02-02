<template>
  <div v-if="show" class="emoji-picker-wrapper" @click.self="$emit('close')">
    <div class="emoji-picker-container">
      <EmojiPicker
        :native="true"
        :disable-skin-tones="true"
        :disable-sticky-group-names="true"
        theme="auto"
        @select="onSelectEmoji"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import EmojiPicker from 'vue3-emoji-picker'
import 'vue3-emoji-picker/css'

interface Props {
  show: boolean
}

defineProps<Props>()

const emit = defineEmits<{
  close: []
  select: [emoji: string]
}>()

function onSelectEmoji(emoji: { i: string; n: string[]; r: string; t: string; u: string }) {
  emit('select', emoji.i)
  emit('close')
}
</script>

<style scoped>
.emoji-picker-wrapper {
  position: fixed;
  inset: 0;
  z-index: 100;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.3);
  backdrop-filter: blur(4px);
}

.emoji-picker-container {
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
}

:deep(.v3-emoji-picker) {
  --ep-color-bg: var(--card);
  --ep-color-border: var(--border);
  --ep-color-text: var(--foreground);
  --ep-color-sbg: var(--muted);
  border: 1px solid var(--border);
}

:deep(.v3-emoji-picker .v3-header) {
  border-bottom: 1px solid var(--border);
}

:deep(.v3-emoji-picker .v3-footer) {
  display: none;
}
</style>
