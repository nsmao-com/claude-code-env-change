<script setup lang="ts">
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { ref, shallowRef, reactive, computed, watch, onScopeDispose } from 'vue'
import { useWorkbench } from '@/composables/useWorkbench'
import { callService } from '@/services/appBridge'
import { Checkbox } from '@/components/ui/checkbox'
import type { DateValue } from 'reka-ui'
import WorkbenchDatePicker from './WorkbenchDatePicker.vue'
import SegmentedPills from '@/components/layout/SegmentedPills.vue'
import SessionInsights from './SessionInsights.vue'
import SessionTrash from './SessionTrash.vue'
import { useConfirm } from '@/composables/useConfirm'
import type { SessionPage, SessionDetail, SessionSummary } from '@/types/workbench'
const { tx, busy, error, run } = useWorkbench()
const confirm = useConfirm()
const view = ref<'list' | 'insights' | 'trash'>('list')
const viewItems = computed(() => [
  { value: 'list', label: tx('会话列表', 'Sessions') },
  { value: 'insights', label: tx('使用洞察', 'Insights') },
  { value: 'trash', label: tx('回收站', 'Recycle bin') },
])
const query = reactive({
  keyword: '',
  provider: '',
  project: '',
  archived: false,
  offset: 0,
  limit: 20,
  from: 0,
  to: 0,
})
const from = shallowRef<DateValue>(),
  to = shallowRef<DateValue>()
const providerFilter = computed({
  get: () => query.provider || 'all',
  set: (value: string) => { query.provider = value === 'all' ? '' : value },
})
const page = ref<SessionPage>({ items: [], total: 0, warnings: [] })
const detail = ref<SessionDetail | null>(null)
const messageOffset = ref(0)
let generation = 0
const searching = ref(false)
async function load(reset = false) {
  if (reset) query.offset = 0
  const zone = Intl.DateTimeFormat().resolvedOptions().timeZone
  query.from = from.value ? from.value.toDate(zone).getTime() : 0
  query.to = to.value ? to.value.add({ days: 1 }).toDate(zone).getTime() - 1 : 0
  const current = ++generation
  searching.value = true
  error.value = ''
  try {
    const result = await callService<SessionPage>('SessionService', 'ListSessions', { ...query })
    if (current === generation) page.value = result
  } catch (e) {
    if (current === generation) error.value = String(e)
  } finally {
    if (current === generation) searching.value = false
  }
}
let timer: ReturnType<typeof setTimeout>
function changePage(delta: number) {
  query.offset = Math.max(0, query.offset + delta)
  void load()
}
watch(
  () => [query.keyword, query.provider, query.project, query.archived, from.value, to.value],
  () => {
    clearTimeout(timer)
    timer = setTimeout(() => load(true), 300)
  },
)
onScopeDispose(() => {
  clearTimeout(timer)
  generation++
})
async function open(item: SessionSummary, offset = 0) {
  await run(async () => {
    detail.value = await callService<SessionDetail>('SessionService', 'GetSession', item.id, offset, 30)
    messageOffset.value = offset
  })
}
async function action(name: string, ...args: unknown[]) {
  await run(() => callService('SessionService', name, ...args))
}
async function archive() {
  if (!detail.value) return
  const item = detail.value.session
  await run(async () => {
    await callService('SessionService', 'ArchiveSession', item.id, !item.archived)
    detail.value = null
    await load()
  })
}
async function remove() {
  if (!detail.value) return
  const item = detail.value.session
  const ok = await confirm.show(
    tx('删除会话', 'Delete session'),
    tx(
      `「${item.title}」的原始日志会移到回收站，可随时恢复。若会话仍在 CLI 中运行，请先结束再删除。`,
      `The original log of “${item.title}” moves to the recycle bin and can be restored any time. End the session in the CLI first if it is still running.`,
    ),
    'warning',
  )
  if (!ok) return
  await run(async () => {
    await callService('SessionService', 'DeleteSession', item.id)
    detail.value = null
    await load()
  }, tx('已移到回收站', 'Moved to the recycle bin'))
}
void load()
</script>
<template>
  <div class="mb-4">
    <SegmentedPills v-model="view" layout-id="sessions-view" :items="viewItems" />
  </div>
  <SessionInsights v-if="view === 'insights'" />
  <SessionTrash v-else-if="view === 'trash'" @restored="load()" />
  <template v-else>
  <div role="alert" v-if="error" class="wb-error">{{ error }}</div>
  <div class="wb-card">
    <div class="wb-grid">
      <label
        >{{ tx('搜索标题或正文', 'Search title or messages')
        }}
        <Input v-model="query.keyword" type="search" :placeholder="tx('输入关键词…', 'Search…')" /></label>
      <label
        >{{ tx('工具', 'Tool')
        }}
        <Select v-model="providerFilter">
          <SelectTrigger class="h-9 w-full min-w-0">
            <SelectValue />
          </SelectTrigger>
          <SelectContent position="popper" align="start">
            <SelectItem value="all">{{ tx('全部', 'All') }}</SelectItem>
            <SelectItem value="claude">Claude Code</SelectItem>
            <SelectItem value="codex">Codex</SelectItem>
            <SelectItem value="antigravity">Gemini</SelectItem>
          </SelectContent>
        </Select></label
      >
      <label>{{ tx('项目路径', 'Project path') }}
        <Input v-model="query.project" type="search" /></label>
      <label>{{ tx('开始日期', 'From date') }}
        <WorkbenchDatePicker v-model="from" :max-value="to" :label="tx('开始日期', 'From date')" /></label
      ><label>{{ tx('结束日期', 'To date') }}
        <WorkbenchDatePicker v-model="to" :min-value="from" :label="tx('结束日期', 'To date')" /></label>
    </div>
    <div class="wb-row mt-4">
      <label class="wb-check"
        >
        <Checkbox :model-value="query.archived" @update:model-value="query.archived = $event === true" />{{ tx('查看归档', 'Show archived') }}</label
      >
      <Button variant="outline" type="button" :disabled="searching" @click="load()">
        {{ searching ? tx('检索中…', 'Searching…') : tx('刷新', 'Refresh') }}
      </Button>
        <span class="text-xs text-muted-foreground">{{ page.total }} {{ tx('个会话', 'sessions') }}</span>
    </div>
    <p class="wb-hint">
      {{
        tx(
          '归档仅影响 AI ENV 列表。继续会话会在原项目目录打开对应 CLI。',
          'Archiving only changes the AI ENV list. Resume opens the CLI in the original project directory.',
        )
      }}
    </p>
    <p v-for="warning in page.warnings" :key="warning" class="wb-error">{{ warning }}</p>
  </div>
  <div class="wb-split">
    <div>
      <div class="wb-list">
        <Button
          variant="outline"
          type="button"
          v-for="item in page.items"
          :key="item.id"
          :class="{ active: detail?.session.id === item.id }"
          :disabled="busy"
          @click="open(item)"
        >
          <strong class="line-clamp-2">{{ item.title }}</strong>
          <span class="text-xs opacity-70">
            {{ item.provider }}
            ·
            {{ new Date(item.updated).toLocaleString() }}
          </span>
          <span class="truncate text-xs opacity-70">{{ item.project }}</span>
        </Button>
      </div>
      <div v-if="!page.items.length && !searching" class="wb-empty">
        {{
          tx(
            '没有匹配的会话。调整筛选条件，或先在 CLI 中开始会话。',
            'No matching sessions. Adjust filters or start a session in the CLI.',
          )
        }}
      </div>
      <div class="wb-row mt-4">
        <Button
          variant="outline"
          type="button"
          :disabled="query.offset === 0 || searching"
          @click="changePage(-20)"
        >
          {{ tx('上一页', 'Previous') }}
        </Button>
          <Button
          variant="outline"
          type="button"
          :disabled="query.offset + 20 >= page.total || searching"
          @click="changePage(20)"
        >
          {{ tx('下一页', 'Next') }}
        </Button>
      </div>
    </div>
    <section class="wb-card" v-if="detail">
      <h2>{{ detail.session.title }}</h2>
      <p class="wb-hint break-all">{{ detail.session.path }}</p>
      <p v-if="detail.session.warning" class="wb-error">{{ detail.session.warning }}</p>
      <div class="wb-row">
        <Button
          variant="outline"
          type="button"
          :disabled="busy"
          @click="action('ExportSession', detail.session.id)"
        >
          {{ tx('导出 Markdown', 'Export Markdown') }}
        </Button>
          <Button
          variant="outline"
          type="button"
          :disabled="busy"
          @click="action('OpenSessionDirectory', detail.session.id)"
        >
          {{ tx('打开目录', 'Open folder') }}
        </Button>
          <Button
          variant="outline"
          type="button"
          v-if="detail.session.provider !== 'antigravity'"
          :disabled="busy"
          @click="action('ResumeSession', detail.session.id)"
        >
          {{ tx('继续会话', 'Resume') }}
        </Button>
          <Button
          variant="outline"
          type="button"
          v-if="!detail.session.path.split('\\').join('/').includes('/archived_sessions/')"
          :disabled="busy"
          @click="archive"
        >
          {{ detail.session.archived ? tx('恢复到列表', 'Unarchive') : tx('归档', 'Archive') }}
        </Button>
        <Button
          variant="ghost"
          type="button"
          class="text-destructive"
          v-if="detail.session.provider !== 'antigravity'"
          :disabled="busy"
          @click="remove"
        >
          {{ tx('删除到回收站', 'Move to recycle bin') }}
        </Button>
      </div>
      <article v-for="(msg, i) in detail.messages" :key="messageOffset + i" class="mb-4">
        <div class="wb-row text-xs text-muted-foreground">
          <strong>{{ msg.role }}</strong
          ><span>{{ msg.at }}</span>
        </div>
        <pre>{{ msg.text }}</pre>
      </article>
      <div class="wb-row">
        <Button
          variant="outline"
          type="button"
          :disabled="busy || messageOffset === 0"
          @click="open(detail.session, messageOffset - 30)"
        >
          {{ tx('前 30 条', 'Previous 30') }}
        </Button>
          <span>{{ messageOffset + 1 }}–{{ messageOffset + detail.messages.length }} / {{ detail.total }}</span
        >
        <Button
          variant="outline"
          type="button"
          :disabled="busy || messageOffset + 30 >= detail.total"
          @click="open(detail.session, messageOffset + 30)"
        >
          {{ tx('后 30 条', 'Next 30') }}
        </Button>
      </div>
    </section>
    <div v-else class="wb-empty">
      {{ tx('选择左侧会话查看消息', 'Select a session to read its messages') }}
    </div>
  </div>
  </template>
</template>
