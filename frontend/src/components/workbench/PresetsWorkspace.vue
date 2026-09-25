<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
import { useWorkbench, workbench } from '@/composables/useWorkbench'
import { callService } from '@/services/appBridge'
import { useConfigStore } from '@/stores/configStore'
import { useConfirm } from '@/composables/useConfirm'
import type { WorkbenchConfig, ProjectPreset, PromptPreset } from '@/types/workbench'
import type { MCPServer, Skill } from '@/types'
const { tx, busy, error, run } = useWorkbench(),
  config = useConfigStore(),
  confirm = useConfirm()
const projects = ref<ProjectPreset[]>([]),
  prompts = ref<PromptPreset[]>([]),
  servers = ref<MCPServer[]>([]),
  skills = ref<Skill[]>([])
const selected = ref(''),
  target = ref('claude'),
  message = ref('')
const env = computed(() => config.environments.find((e) => `${e.provider}/${e.name}` === selected.value))
const form = reactive<ProjectPreset>({
  name: '',
  directory: '',
  provider: '',
  environment: '',
  model: '',
  mcp: [],
  skills: [],
  prompt: '',
})
const prompt = reactive<PromptPreset>({ name: '', content: '' })
async function load() {
  const c = await workbench<WorkbenchConfig>('GetWorkbench')
  projects.value = c.projects || []
  prompts.value = c.prompts || []
}
async function pick() {
  await run(async () => {
    const p = await workbench<string>('PickProjectDirectory')
    if (p) form.directory = p
  })
}
async function saveProject() {
  if (!env.value) return
  await run(
    async () => {
      await workbench('SaveProjectPreset', {
        ...form,
        provider: env.value!.provider,
        environment: env.value!.name,
      })
      await load()
    },
    tx('项目套装已保存', 'Project preset saved'),
  )
}
async function applyProject(p: ProjectPreset) {
  if (
    !(await confirm.show(
      tx('应用项目套装', 'Apply project preset'),
      tx(
        `将为 ${p.provider} 切换全局环境、MCP、Skills 和提示词。其他项目随后使用同一配置。`,
        `This switches the global ${p.provider} environment, MCP, Skills and prompt. Other projects will share this configuration.`,
      ),
      'warning',
    ))
  )
    return
  await run(async () => {
    message.value = await workbench('ApplyProjectPreset', p.name)
    await config.loadConfig()
  })
}
async function remove(kind: 'Project' | 'Prompt', name: string) {
  if (!(await confirm.show(tx('删除套装/模板', 'Delete preset'), name, 'warning'))) return
  await run(async () => {
    await workbench(`Delete${kind}Preset`, name)
    await load()
  })
}
async function savePrompt() {
  await run(
    async () => {
      await workbench('SavePromptPreset', { ...prompt })
      await load()
    },
    tx('提示词已保存', 'Prompt saved'),
  )
}
async function applyPrompt(name: string) {
  if (
    !(await confirm.show(
      tx('应用提示词', 'Apply prompt'),
      tx(
        `将覆盖 ${target.value} 当前全局提示词，原内容会备份。`,
        `Replaces the current ${target.value} global prompt and backs up its content.`,
      ),
      'warning',
    ))
  )
    return
  await run(() => workbench('ApplyPromptPreset', name, target.value), tx('提示词已应用', 'Prompt applied'))
}
void run(async () => {
  await config.loadConfig()
  await load()
  servers.value = await callService('MCPService', 'ListServers')
  skills.value = await callService('SkillService', 'ListSkills')
})
</script>
<template>
  <p v-if="error" role="alert" class="wb-error">{{ error }}</p>
  <p v-if="message" class="wb-hint">{{ message }}</p>
  <section class="wb-card">
    <h2>{{ tx('项目套装', 'Project presets') }}</h2>
    <p class="wb-hint">
      {{
        tx(
          '为不同项目保存环境、模型、MCP、技能与提示词组合。应用会切换工具的全局配置。',
          'Save environment, model, MCP, skills and prompt combinations for projects. Applying switches the tool’s global configuration.',
        )
      }}
    </p>
    <form @submit.prevent="saveProject">
      <div class="wb-grid">
        <label
          >{{ tx('套装名称', 'Preset name') }}<input v-model="form.name" required maxlength="100" /></label
        ><label
          >{{ tx('环境', 'Environment')
          }}<select v-model="selected" required>
            <option value="" disabled>{{ tx('选择环境', 'Select environment') }}</option>
            <option
              v-for="e in config.environments"
              :key="`${e.provider}/${e.name}`"
              :value="`${e.provider}/${e.name}`"
            >
              {{ e.provider }} · {{ e.name }}
            </option>
          </select></label
        ><label
          >{{ tx('模型覆盖（可选）', 'Model override (optional)')
          }}<input
            v-model="form.model"
            :placeholder="tx('沿用环境默认模型', 'Use environment default')" /></label
        ><label
          >{{ tx('提示词模板', 'Prompt preset')
          }}<select v-model="form.prompt">
            <option value="">{{ tx('保留当前提示词', 'Keep current prompt') }}</option>
            <option v-for="p in prompts" :key="p.name">{{ p.name }}</option>
          </select></label
        >
      </div>
      <div class="wb-row mt-4">
        <button type="button" :disabled="busy" @click="pick">
          {{ tx('选择项目目录', 'Choose project folder') }}</button
        ><span class="break-all text-xs text-muted-foreground">{{
          form.directory || tx('尚未选择', 'No folder selected')
        }}</span>
      </div>
      <div class="wb-grid">
        <fieldset class="rounded-lg border border-border p-3">
          <legend class="px-2 text-sm">MCP</legend>
          <label class="wb-check mb-2" v-for="s in servers" :key="s.name"
            ><input type="checkbox" v-model="form.mcp" :value="s.name" />{{ s.name }}</label
          >
          <p v-if="!servers.length" class="wb-hint">
            {{ tx('请先在 MCP 页添加服务', 'Add servers in MCP first') }}
          </p>
        </fieldset>
        <fieldset class="rounded-lg border border-border p-3">
          <legend class="px-2 text-sm">Skills</legend>
          <label class="wb-check mb-2" v-for="s in skills" :key="s.name"
            ><input type="checkbox" v-model="form.skills" :value="s.name" />{{ s.name }}</label
          >
          <p v-if="!skills.length" class="wb-hint">
            {{ tx('请先在 Skills 页安装技能', 'Install skills first') }}
          </p>
        </fieldset>
      </div>
      <button class="primary mt-4" :disabled="busy || !env || !form.directory">
        {{ tx('保存套装', 'Save preset') }}
      </button>
    </form>
    <div class="wb-table mt-5" v-if="projects.length">
      <table>
        <tbody>
          <tr v-for="p in projects" :key="p.name">
            <td>
              <strong>{{ p.name }}</strong>
              <p class="text-xs text-muted-foreground">{{ p.provider }} · {{ p.environment }}</p>
              <p class="break-all text-xs text-muted-foreground">{{ p.directory }}</p>
            </td>
            <td>
              <div class="wb-row mb-0">
                <button
                  :disabled="busy"
                  @click="
                    Object.assign(form, { ...p, mcp: [...p.mcp], skills: [...p.skills] })
                    selected = `${p.provider}/${p.environment}`
                  "
                >
                  {{ tx('编辑', 'Edit') }}</button
                ><button :disabled="busy" @click="applyProject(p)">{{ tx('应用', 'Apply') }}</button
                ><button :disabled="busy" @click="remove('Project', p.name)">
                  {{ tx('删除', 'Delete') }}
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
  <section class="wb-card">
    <h2>{{ tx('提示词库', 'Prompt library') }}</h2>
    <form @submit.prevent="savePrompt">
      <label
        >{{ tx('模板名称', 'Preset name') }}<input v-model="prompt.name" required maxlength="100" /></label
      ><label class="mt-4"
        >{{ tx('提示词内容', 'Prompt content')
        }}<textarea v-model="prompt.content" rows="7" required></textarea></label
      ><button class="primary mt-4" :disabled="busy">{{ tx('保存提示词', 'Save prompt') }}</button>
    </form>
    <label class="mt-5"
      >{{ tx('应用到工具', 'Apply to tool')
      }}<select v-model="target">
        <option value="claude">Claude Code</option>
        <option value="codex">Codex</option>
        <option value="antigravity">Gemini</option>
        <option value="opencode">OpenCode</option>
        <option value="grok">Grok</option>
      </select></label
    >
    <div class="wb-list mt-4">
      <div v-for="p in prompts" :key="p.name" class="wb-row">
        <strong class="mr-auto">{{ p.name }}</strong
        ><button :disabled="busy" @click="Object.assign(prompt, p)">{{ tx('编辑', 'Edit') }}</button
        ><button :disabled="busy" @click="applyPrompt(p.name)">{{ tx('应用', 'Apply') }}</button
        ><button :disabled="busy" @click="remove('Prompt', p.name)">{{ tx('删除', 'Delete') }}</button>
      </div>
    </div>
  </section>
</template>
