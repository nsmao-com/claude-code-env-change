<template>
  <div
    :class="[
      'group min-w-0 overflow-hidden rounded-lg border border-border bg-background transition-colors hover:border-primary/50',
      compact ? 'p-2.5' : 'p-4',
    ]"
  >
    <div class="flex min-w-0 items-center justify-between gap-3">
      <div class="flex min-w-0 flex-1 items-center gap-3">
        <div
          :class="[
            'flex shrink-0 items-center justify-center rounded-lg bg-primary/10',
            compact ? 'size-8' : 'size-10',
          ]"
        >
          <Globe v-if="server.type === 'http'" :class="['text-primary', compact ? 'size-3.5' : 'size-5']" />
          <Terminal v-else :class="['text-primary', compact ? 'size-3.5' : 'size-5']" />
        </div>

        <div class="min-w-0 flex-1 overflow-hidden">
          <AppTooltip :content="server.name" wrap class="block w-full min-w-0">
            <h4 :class="['min-w-0 truncate font-semibold', compact ? 'text-xs' : 'text-sm']">{{ server.name }}</h4>
          </AppTooltip>
          <div class="mt-1 flex min-w-0 flex-wrap items-center gap-2">
            <PlatformChips
              :enabled="platforms"
              :items="MCP_PLATFORM_ITEMS"
              :compact="compact"
              @toggle="$emit('toggle-platform', $event)"
            />
            <AppTooltip v-if="testResult && !testResult.success" :content="testResult.message || t('mcp.card.checkFailed')" wrap>
              <Badge variant="outline" :class="testResultClass">
                {{ t('mcp.card.failed') }}
              </Badge>
            </AppTooltip>
            <Badge
              v-else-if="testResult"
              variant="outline"
              :class="testResultClass"
            >
              <Check v-if="testResult.success" />
              {{ testResult.latency }}ms
            </Badge>
            <Badge v-if="oauth?.signed_in" variant="outline" class="border-green-500/20 bg-green-500/10 text-[10px] text-green-600">
              {{ locale === 'zh' ? 'OAuth 已登录' : 'OAuth signed in' }}
            </Badge>
            <Badge v-else-if="oauth?.pending" variant="outline" class="text-[10px]">
              {{ locale === 'zh' ? '等待浏览器授权…' : 'Waiting for browser…' }}
            </Badge>
            <AppTooltip v-else-if="oauth?.error" :content="oauth.error" wrap>
              <Badge variant="outline" class="border-red-500/20 bg-red-500/10 text-[10px] text-red-500">
                {{ locale === 'zh' ? 'OAuth 登录失败' : 'OAuth failed' }}
              </Badge>
            </AppTooltip>
          </div>

          <AppTooltip v-if="!compact" :content="detailInfo" wrap class="mt-1 block w-full min-w-0">
            <div class="truncate font-mono text-xs text-muted-foreground">
              {{ detailInfo }}
            </div>
          </AppTooltip>

          <div v-if="!compact && server.tips" class="mt-1 line-clamp-2 break-words text-xs text-muted-foreground">
            {{ server.tips }}
          </div>
          <details v-if="testResult" class="mt-2 text-xs text-muted-foreground"><summary class="cursor-pointer">MCP · {{ testResult.stage }} · {{ testResult.protocol }} · {{ testResult.tools?.length || 0 }} tools</summary><p class="mt-2 break-words">{{ testResult.message }}</p><ul class="mt-2 max-h-40 overflow-auto"><li v-for="tool in testResult.tools" :key="tool">{{ tool }}</li></ul></details>
        </div>
      </div>

      <div
        :class="[
          'flex shrink-0 gap-1.5 transition-opacity',
          compact ? 'opacity-100' : 'opacity-0 group-hover:opacity-100',
        ]"
      >
        <AppTooltip :content="t('mcp.card.testConnection')">
        <Button
          variant="ghost"
          size="icon-sm"
          :disabled="isTesting"
          @click="$emit('test')"
        >
          <Loader2 v-if="isTesting" class="animate-spin" />
          <Zap v-else />
        </Button>
        </AppTooltip>
        <AppTooltip
          v-if="server.type === 'http'"
          :content="oauth?.signed_in
            ? (locale === 'zh' ? '退出 OAuth 登录（各工具恢复直连原地址）' : 'Sign out (tools go back to the original URL)')
            : (locale === 'zh' ? 'OAuth 登录：授权一次后，各工具经本机网关访问并自动带上令牌' : 'OAuth sign-in: authorize once, tools reach it through the local gateway with the token added')"
          wrap
        >
        <Button
          variant="ghost"
          size="icon-sm"
          :disabled="oauth?.pending"
          :aria-label="oauth?.signed_in ? 'OAuth sign out' : 'OAuth sign in'"
          @click="oauth?.signed_in ? $emit('oauth-logout') : $emit('oauth-login')"
        >
          <KeyRound :class="oauth?.signed_in ? 'text-green-600' : ''" />
        </Button>
        </AppTooltip>
        <AppTooltip v-if="server.website" :content="t('mcp.card.website')">
        <Button
          as="a"
          :href="server.website"
          target="_blank"
          variant="ghost"
          size="icon-sm"
        >
          <ExternalLink />
        </Button>
        </AppTooltip>
        <AppTooltip :content="t('mcp.card.edit')">
        <Button
          variant="ghost"
          size="icon-sm"
          @click="$emit('edit')"
        >
          <Pencil />
        </Button>
        </AppTooltip>
        <AppTooltip :content="t('mcp.card.delete')">
        <Button
          variant="ghost"
          size="icon-sm"
          class="text-muted-foreground hover:text-destructive"
          @click="$emit('delete')"
        >
          <Trash2 />
        </Button>
        </AppTooltip>
      </div>
    </div>

    <div
      v-if="!compact && hasPlaceholder"
      class="mt-2 flex items-start gap-1.5 rounded-lg border border-yellow-500/20 bg-yellow-500/10 p-2 text-xs text-yellow-600"
    >
      <TriangleAlert class="mt-0.5 size-3.5 shrink-0" />
      {{ t('mcp.card.missingPlaceholders', { names: server.missing_placeholders.join(', ') }) }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from '@/composables/useI18n'
import { computed } from 'vue'
import { Check, ExternalLink, Globe, KeyRound, Loader2, Pencil, Terminal, Trash2, TriangleAlert, Zap } from '@lucide/vue'
import type { MCPServer, MCPTestResult } from '@/types'
import AppTooltip from '@/components/common/AppTooltip.vue'
import PlatformChips from '@/components/common/PlatformChips.vue'
import { MCP_PLATFORM_ITEMS } from '@/lib/platforms'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

const { t, locale } = useI18n()

interface Props {
  server: MCPServer
  testResult?: MCPTestResult
  isTesting?: boolean
  compact?: boolean
  oauth?: { signed_in: boolean; pending: boolean; error?: string }
}

const props = defineProps<Props>()

defineEmits<{
  test: []
  edit: []
  delete: []
  'toggle-platform': [platform: string]
  'oauth-login': []
  'oauth-logout': []
}>()

const platforms = computed(() => props.server.enable_platform || [])
const hasPlaceholder = computed(() =>
  props.server.missing_placeholders && props.server.missing_placeholders.length > 0
)

const detailInfo = computed(() => {
  if (props.server.type === 'http') {
    return props.server.url || '-'
  }
  return `${props.server.command || ''} ${(props.server.args || []).join(' ')}`
})

const testResultClass = computed(() => {
  if (!props.testResult) return ''
  return props.testResult.success
    ? 'border-green-500/20 bg-green-500/10 text-[10px] text-green-500'
    : 'border-red-500/20 bg-red-500/10 text-[10px] text-red-500'
})
</script>
