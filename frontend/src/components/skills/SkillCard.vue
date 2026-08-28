<template>
  <div
    :class="[
      'group min-w-0 overflow-hidden rounded-lg border border-border bg-background transition-colors hover:border-primary/50',
      compact ? 'p-2.5' : 'flex h-full flex-col p-4',
    ]"
  >
    <div :class="compact ? 'flex min-w-0 items-center justify-between gap-3' : 'flex min-w-0 flex-1 flex-col'">
      <div :class="compact ? 'flex min-w-0 flex-1 items-center gap-3' : 'min-w-0 flex-1 overflow-hidden'">
        <div
          v-if="compact"
          class="flex size-8 shrink-0 items-center justify-center rounded-lg bg-primary/10"
        >
          <Layers class="size-3.5 text-primary" />
        </div>
        <div class="min-w-0 flex-1 overflow-hidden">
          <div class="flex min-w-0 items-center gap-2">
            <AppTooltip :content="skill.name" wrap class="min-w-0 flex-1">
              <h4 :class="['min-w-0 truncate font-semibold', compact ? 'text-xs' : 'text-sm']">{{ skill.name }}</h4>
            </AppTooltip>
            <AppTooltip
              v-if="!skill.has_frontmatter || !skill.has_name || !skill.has_description"
              content="SKILL.md frontmatter 可能不完整"
            >
              <Badge variant="destructive" class="shrink-0">格式问题</Badge>
            </AppTooltip>
          </div>
          <p
            v-if="!compact"
            class="mt-1 line-clamp-3 whitespace-pre-line text-xs text-muted-foreground"
          >
            {{ skill.description || skill.frontmatter_error || '（未提供 description）' }}
          </p>
          <div :class="compact ? 'mt-1' : 'mt-3'">
            <PlatformChips
              :enabled="skill.enable_platform || []"
              :compact="compact"
              @toggle="$emit('toggle-platform', $event)"
            />
          </div>
        </div>
      </div>

      <div
        :class="[
          'flex shrink-0 items-center gap-1.5',
          compact ? 'opacity-100' : 'mt-3 justify-end',
        ]"
      >
        <AppTooltip content="编辑">
          <Button variant="ghost" size="icon-sm" @click="$emit('edit')">
            <Pencil />
          </Button>
        </AppTooltip>
        <AppTooltip content="删除">
          <Button variant="ghost" size="icon-sm" class="text-muted-foreground hover:text-destructive" @click="$emit('delete')">
            <Trash2 />
          </Button>
        </AppTooltip>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Layers, Pencil, Trash2 } from '@lucide/vue'
import type { Skill } from '@/types'
import AppTooltip from '@/components/common/AppTooltip.vue'
import PlatformChips from '@/components/common/PlatformChips.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

defineProps<{
  skill: Skill
  compact?: boolean
}>()

defineEmits<{
  edit: []
  delete: []
  'toggle-platform': [platform: string]
}>()
</script>
