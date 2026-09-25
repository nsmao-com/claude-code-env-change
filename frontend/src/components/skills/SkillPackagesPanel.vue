<script setup lang="ts">
import { ref } from 'vue'
import { useWorkbench, workbench } from '@/composables/useWorkbench'
import { callService } from '@/services/appBridge'
import { useConfirm } from '@/composables/useConfirm'
import AppModal from '@/components/common/AppModal.vue'
import type { FileChange } from '@/types/workbench'
import type { Skill } from '@/types'
const open = defineModel<boolean>({ default: false })
const emit = defineEmits<{ saved: [] }>()
const { tx, busy, error, run } = useWorkbench(),
  confirm = useConfirm()
const repo = ref(''),
  directory = ref(''),
  revision = ref('main'),
  preview = ref<Skill | null>(null),
  installed = ref<Skill[]>([]),
  keepLocal = ref(true)
const update = ref<{
  name: string
  token: string
  revision: string
  changes: string[]
  conflicts: string[]
  platform_conflicts: string[]
  details: FileChange[]
} | null>(null)
async function load() {
  installed.value = await callService('SkillService', 'ListSkills')
}
async function github() {
  await run(async () => {
    preview.value = await callService(
      'SkillService',
      'ImportSkillPackage',
      repo.value,
      directory.value,
      revision.value,
    )
  })
}
async function zip() {
  await run(async () => {
    const path = await workbench<string>('PickImportFile', 'zip')
    if (path) preview.value = await callService('SkillService', 'ImportSkillZip', path, directory.value)
  })
}
async function local() {
  await run(async () => {
    const path = await workbench<string>('PickProjectDirectory')
    if (path) preview.value = await callService('SkillService', 'ImportSkillDirectory', path)
  })
}
async function install() {
  if (!preview.value) return
  if (
    installed.value.some((s) => s.name === preview.value!.name) &&
    !(await confirm.show(tx('覆盖同名技能', 'Replace existing skill'), preview.value.name, 'warning'))
  )
    return
  await run(
    async () => {
      await callService('SkillService', 'SaveSkill', preview.value)
      preview.value = null
      await load()
      emit('saved')
    },
    tx('完整技能包已安装', 'Full skill package installed'),
  )
}
async function check(s: Skill) {
  await run(async () => {
    update.value = await callService('SkillService', 'PreviewSkillUpdate', s.name)
  })
}
async function apply() {
  if (!update.value) return
  await run(
    async () => {
      await callService(
        'SkillService',
        'ApplySkillUpdate',
        update.value!.name,
        update.value!.token,
        keepLocal.value,
      )
      update.value = null
      await load()
      emit('saved')
    },
    tx(
      '技能已更新，原版本可在配置历史恢复',
      'Skill updated. Restore the previous version from configuration history.',
    ),
  )
}
void run(load)
</script>
<template>
  <AppModal v-model="open" :title="tx('完整技能包', 'Skill packages')" size="xl"
    ><div class="workbench">
      <p v-if="error" class="wb-error" role="alert">{{ error }}</p>
      <section class="wb-card">
        <h2>{{ tx('安装技能及附件', 'Install skills and their files') }}</h2>
        <p class="wb-hint">
          {{
            tx(
              '保留 scripts、references、assets 等目录。安装前请确认技能来源；导入过程不会执行脚本。',
              'Keeps scripts, references and assets. Verify the source before installing. Importing does not execute scripts.',
            )
          }}
        </p>
        <form @submit.prevent="github">
          <div class="wb-grid">
            <label
              >{{ tx('GitHub 仓库', 'GitHub repository')
              }}<input v-model="repo" placeholder="owner/repository" required /></label
            ><label
              >{{ tx('技能目录（多技能仓库必填）', 'Skill directory (required for multi-skill repos)')
              }}<input v-model="directory" placeholder="skills/my-skill" /></label
            ><label
              >{{ tx('分支 / 标签 / Commit', 'Branch / tag / commit') }}<input v-model="revision" required
            /></label>
          </div>
          <div class="wb-row mt-4">
            <button :disabled="busy">{{ tx('获取 GitHub 技能', 'Fetch GitHub skill') }}</button
            ><button type="button" :disabled="busy" @click="zip">{{ tx('选择 ZIP', 'Choose ZIP') }}</button
            ><button type="button" :disabled="busy" @click="local">
              {{ tx('选择本地目录', 'Choose local folder') }}
            </button>
          </div>
        </form>
        <div v-if="preview">
          <h3>
            {{ preview.name }} · {{ Object.keys(preview.files || {}).length }} {{ tx('个附件', 'files') }}
          </h3>
          <p class="wb-hint">{{ preview.source?.repo }} {{ preview.source?.revision.slice(0, 12) }}</p>
          <details>
            <summary>{{ tx('查看文件与内容', 'View files and content') }}</summary>
            <pre>{{ Object.keys(preview.files || {}).join('\n') }}</pre>
            <pre class="mt-3">{{ preview.content }}</pre>
          </details>
          <fieldset class="mt-4">
            <legend class="mb-2 text-sm">{{ tx('安装到', 'Install to') }}</legend>
            <div class="wb-row">
              <label
                class="wb-check"
                v-for="p in ['claude-code', 'codex', 'antigravity', 'opencode', 'grok']"
                :key="p"
                ><input type="checkbox" v-model="preview.enable_platform" :value="p" />{{ p }}</label
              >
            </div>
          </fieldset>
          <button class="primary" :disabled="busy || !preview.enable_platform.length" @click="install">
            {{ tx('安装完整包', 'Install package') }}
          </button>
        </div>
      </section>
      <section class="wb-card">
        <div class="wb-row justify-between">
          <h2>{{ tx('来源与更新', 'Sources & updates') }}</h2>
          <button :disabled="busy" @click="run(load)">{{ tx('刷新', 'Refresh') }}</button>
        </div>
        <div class="wb-empty" v-if="!installed.length">{{ tx('还没有安装技能', 'No installed skills') }}</div>
        <div class="wb-row" v-for="s in installed" :key="s.name">
          <div class="mr-auto">
            <strong>{{ s.name }}</strong>
            <p class="text-xs text-muted-foreground">
              {{ s.source?.repo || tx('本地技能', 'Local skill') }} · {{ s.source?.ref }} ·
              {{ s.source?.revision.slice(0, 10) }} · {{ Object.keys(s.files || {}).length }}
              {{ tx('个附件', 'files') }}
            </p>
          </div>
          <button v-if="s.source" :disabled="busy" @click="check(s)">
            {{ tx('检查更新', 'Check updates') }}
          </button>
        </div>
        <div v-if="update" class="mt-5">
          <h3>{{ update.name }} → {{ update.revision.slice(0, 12) }}</h3>
          <pre>{{ update.changes.join('\n') || tx('没有变化', 'No changes') }}</pre>
          <p v-if="update.conflicts.length" class="wb-error mt-3">
            {{ tx('以下文件在本地修改过：', 'Locally modified files:') }} {{ update.conflicts.join(', ') }}
          </p>
          <p v-if="update.platform_conflicts?.length" class="wb-error">
            {{
              tx(
                '平台目录存在手动修改，将保留这些附件：',
                'Platform files were edited locally; these assets will be preserved:',
              )
            }}
            {{ update.platform_conflicts.join(', ') }}
          </p>
          <details v-for="change in update.details" :key="change.path" class="mt-3">
            <summary>{{ change.path }}</summary>
            <div class="wb-grid mt-3">
              <div>
                <h3>{{ tx('当前版本', 'Current') }}</h3>
                <pre>{{ change.before }}</pre>
              </div>
              <div>
                <h3>{{ tx('更新版本', 'Updated') }}</h3>
                <pre>{{ change.after }}</pre>
              </div>
            </div>
          </details>
          <label class="wb-check mt-3"
            ><input type="checkbox" v-model="keepLocal" />{{
              tx('保留本地修改', 'Preserve local changes')
            }}</label
          ><button class="primary mt-3" :disabled="busy || !update.changes.length" @click="apply">
            {{ tx('应用更新', 'Apply update') }}
          </button>
        </div>
      </section>
    </div></AppModal
  >
</template>
