<script setup lang="ts">
import { CheckboxGroupRoot } from 'reka-ui'
import { Checkbox } from '@/components/ui/checkbox'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { ref, reactive } from 'vue'
import { useWorkbench, workbench } from '@/composables/useWorkbench'
import { useConfigStore } from '@/stores/configStore'
import { useConfirm } from '@/composables/useConfirm'
import type { EnvConfig } from '@/types'
const { tx, busy, error, run } = useWorkbench(),
  config = useConfigStore(),
  confirm = useConfirm()
const link = ref(''),
  imports = ref<EnvConfig[]>([]),
  selected = ref<number[]>([]),
  message = ref('')
const universal = reactive({
  name: '',
  base_url: '',
  api_key: '',
  model: '',
  format: 'chat_completions',
  providers: ['claude', 'codex'],
})
const tools = [
  { id: 'claude', name: 'Claude Code' },
  { id: 'codex', name: 'Codex' },
  { id: 'antigravity', name: 'Gemini' },
  { id: 'opencode', name: 'OpenCode' },
  { id: 'grok', name: 'Grok' },
  { id: 'claude_desktop', name: 'Claude Desktop' },
]
async function preview(text: string) {
  imports.value = await workbench('PreviewExternalImport', text)
  selected.value = imports.value.map((_, i) => i)
}
async function file() {
  await run(async () => {
    const path = await workbench<string>('PickImportFile', 'config')
    if (!path) return
    await preview(await workbench<string>('ReadExternalImport', path))
  })
}
async function commit() {
  await run(async () => {
    const count = await workbench<number>(
      'CommitExternalImport',
      selected.value.map((i) => imports.value[i]),
    )
    message.value = tx(
      `已导入 ${count} 个环境，同名配置已自动重命名。`,
      `Imported ${count} environments. Name collisions were renamed.`,
    )
    imports.value = []
    link.value = ''
    await config.loadConfig()
  })
}
async function saveUniversal() {
  if (
    !(await confirm.show(
      tx('保存通用供应商', 'Save universal provider'),
      tx(
        '将在所选工具中创建关联配置；已有同名关联配置会更新，应用请到环境页操作。',
        'Creates linked environments in the selected tools and updates existing linked entries. Apply them from Environments.',
      ),
    ))
  )
    return
  await run(async () => {
    const count = await workbench<number>('SaveUniversalProvider', { ...universal })
    message.value = tx(`已同步 ${count} 个环境配置。`, `Updated ${count} environments.`)
    universal.api_key = ''
    await config.loadConfig()
  })
}
</script>
<template>
  <p v-if="error" role="alert" class="wb-error">{{ error }}</p>
  <p v-if="message" class="wb-hint">{{ message }}</p>
  <section class="wb-card">
    <h2>{{ tx('从 CC Switch 导入', 'Import from CC Switch') }}</h2>
    <p class="wb-hint">
      {{
        tx(
          '支持 CC Switch JSON 导出、供应商分享链接与 AI ENV 配置文件。预览仅显示名称和工具，密钥不会展示。',
          'Supports CC Switch JSON exports, provider share links and AI ENV configuration files. The preview only shows names and tools, never keys.',
        )
      }}
    </p>
    <div class="wb-row">
      <Button variant="outline" type="button" :disabled="busy" @click="file">
        {{ tx('选择配置文件', 'Choose configuration file') }}
      </Button>
    </div>
    <form @submit.prevent="run(() => preview(link))">
      <label
        >{{ tx('粘贴分享链接', 'Paste a share link')
        }}
        <Input
          v-model="link"
          type="password"
          autocomplete="off"
          placeholder="ccswitch://v1/import?…"
          required
        /></label
      >
      <Button variant="outline" type="submit" class="mt-3" :disabled="busy || !link">
        {{ tx('预览导入', 'Preview import') }}
      </Button>
    </form>
    <CheckboxGroupRoot
      v-if="imports.length"
      v-model="selected"
      :disabled="busy"
      :roving-focus="false"
      class="mt-5"
    >
      <label v-for="(item, i) in imports" :key="i" class="wb-check mb-3">
        <Checkbox :value="i" />
        <strong>{{ item.name }}</strong>
        <span class="text-muted-foreground">{{ item.provider }}</span>
      </label>
      <Button variant="default" type="button" :disabled="busy || !selected.length" @click="commit">
        {{ tx('导入所选配置', 'Import selected') }}
        (
        {{ selected.length }}
        )
      </Button>
    </CheckboxGroupRoot>
  </section>
  <section class="wb-card">
    <h2>{{ tx('通用供应商', 'Universal provider') }}</h2>
    <p class="wb-hint">
      {{
        tx(
          '填写一次连接信息，生成多工具关联配置。跨协议调用需在路由页开启对应工具网关。',
          'Enter connection details once to create linked tool environments. Enable the tool gateway on the Routing page for protocol conversion.',
        )
      }}
    </p>
    <form @submit.prevent="saveUniversal">
      <div class="wb-grid">
        <label
          >{{ tx('供应商名称', 'Provider name')
          }}
          <Input v-model="universal.name" required maxlength="100" /></label
        ><label
          >Base URL
          <Input v-model="universal.base_url" type="url" placeholder="https://api.example.com/v1" required /></label
        ><label
          >API Key
          <Input v-model="universal.api_key" type="password" autocomplete="off" required /></label
        ><label>{{ tx('默认模型', 'Default model') }}
          <Input v-model="universal.model" required /></label
        ><label
          >{{ tx('上游协议', 'Upstream protocol')
          }}
          <Select v-model="universal.format" :disabled="busy">
            <SelectTrigger class="h-9 w-full min-w-0">
              <SelectValue />
            </SelectTrigger>
            <SelectContent position="popper" align="start">
              <SelectItem value="chat_completions">OpenAI Chat Completions</SelectItem>
              <SelectItem value="responses">OpenAI Responses</SelectItem>
              <SelectItem value="anthropic_messages">Anthropic Messages</SelectItem>
            </SelectContent>
          </Select></label
        >
      </div>
      <CheckboxGroupRoot
        v-model="universal.providers"
        :disabled="busy"
        :roving-focus="false"
        class="wb-row mt-4"
      >
        <label class="wb-check" v-for="tool in tools" :key="tool.id">
          <Checkbox :value="tool.id" />
          {{ tool.name }}
        </label>
      </CheckboxGroupRoot>
      <Button variant="default" type="submit" :disabled="busy || !universal.providers.length">
        {{ tx('保存到所选工具', 'Save to selected tools') }}
      </Button>
    </form>
  </section>
</template>
