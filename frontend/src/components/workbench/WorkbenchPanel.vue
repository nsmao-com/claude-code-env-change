<script setup lang="ts">
import { ref } from 'vue'
import { useWorkbench } from '@/composables/useWorkbench'
import SessionsWorkspace from './SessionsWorkspace.vue'
import ModelsWorkspace from './ModelsWorkspace.vue'
import ToolsWorkspace from './ToolsWorkspace.vue'
import PresetsWorkspace from './PresetsWorkspace.vue'
import ImportWorkspace from './ImportWorkspace.vue'
import CostsWorkspace from './CostsWorkspace.vue'
const { tx } = useWorkbench()
const tab = ref('sessions')
const tabs = [
  { id: 'sessions', zh: '会话', en: 'Sessions', component: SessionsWorkspace },
  { id: 'models', zh: '模型', en: 'Models', component: ModelsWorkspace },
  { id: 'tools', zh: '诊断与历史', en: 'Diagnostics & history', component: ToolsWorkspace },
  { id: 'presets', zh: '项目与提示词', en: 'Projects & prompts', component: PresetsWorkspace },
  { id: 'import', zh: '供应商导入', en: 'Providers', component: ImportWorkspace },
  { id: 'costs', zh: '费用与预算', en: 'Costs & budgets', component: CostsWorkspace },
]
</script>
<template>
  <div class="workbench h-full overflow-y-auto px-6 pb-8 pt-4">
    <header class="mb-6">
      <h1 class="text-[2.5rem] font-semibold tracking-tight">{{ tx('工作台', 'Workbench') }}</h1>
      <p class="text-sm text-muted-foreground">
        {{
          tx(
            '会话、模型、项目套装与配置恢复，集中处理日常维护。',
            'Sessions, models, project presets and configuration recovery in one place.',
          )
        }}
      </p>
    </header>
    <nav class="wb-tabs" :aria-label="tx('工作台工具', 'Workbench tools')">
      <button
        v-for="item in tabs"
        :key="item.id"
        :aria-current="tab === item.id ? 'page' : undefined"
        :class="{ active: tab === item.id }"
        @click="tab = item.id"
      >
        {{ tx(item.zh, item.en) }}
      </button>
    </nav>
    <KeepAlive><component :is="tabs.find((item) => item.id === tab)?.component" /></KeepAlive>
  </div>
</template>
<style>
.workbench .wb-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 24px;
  border-bottom: 1px solid var(--border);
  padding-bottom: 12px;
}
.workbench button {
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 7px 12px;
  font-size: 13px;
  cursor: pointer;
  background: var(--card);
  transition: background 0.15s;
}
.workbench button:hover {
  background: var(--muted);
}
.workbench button.active,
.workbench button.primary {
  background: var(--foreground);
  color: var(--background);
  border-color: var(--foreground);
}
.workbench button:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}
.workbench :focus-visible {
  outline: 2px solid var(--ring);
  outline-offset: 3px;
}
.workbench .wb-card {
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 20px;
  background: var(--card);
  margin-bottom: 16px;
}
.workbench h2 {
  font-weight: 600;
  font-size: 18px;
  margin-bottom: 8px;
}
.workbench h3 {
  font-weight: 600;
  margin-bottom: 8px;
}
.workbench .wb-hint {
  color: var(--muted-foreground);
  font-size: 13px;
  line-height: 1.6;
  margin-bottom: 14px;
}
.workbench .wb-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  margin-bottom: 14px;
}
.workbench .wb-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(230px, 1fr));
  gap: 16px;
}
.workbench label {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 13px;
}
.workbench label.wb-check {
  flex-direction: row;
  align-items: center;
}
.workbench input:not([type='checkbox']),
.workbench select,
.workbench textarea {
  width: 100%;
  min-height: 36px;
  border: 1px solid var(--border);
  border-radius: 7px;
  background: var(--background);
  padding: 8px 10px;
  color: var(--foreground);
  font-size: 13px;
}
.workbench textarea {
  resize: vertical;
  min-height: 130px;
}
.workbench input[type='checkbox'] {
  accent-color: var(--foreground);
}
.workbench .wb-error {
  border: 1px solid var(--destructive);
  color: var(--destructive);
  padding: 12px;
  border-radius: 8px;
  margin-bottom: 16px;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.workbench .wb-empty {
  padding: 40px 16px;
  text-align: center;
  color: var(--muted-foreground);
  border: 1px dashed var(--border);
  border-radius: 10px;
}
.workbench table {
  width: 100%;
  border-collapse: collapse;
  text-align: left;
  font-size: 13px;
}
.workbench td,
.workbench th {
  padding: 10px 12px;
  border-bottom: 1px solid var(--border);
}
.workbench th {
  font-weight: 500;
  color: var(--muted-foreground);
}
.workbench pre {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  font-size: 12px;
  line-height: 1.6;
  background: var(--muted);
  padding: 14px;
  border-radius: 8px;
  max-height: 400px;
  overflow: auto;
}
.workbench .wb-table {
  overflow-x: auto;
}
.workbench .wb-list {
  display: grid;
  gap: 8px;
}
.workbench .wb-list > button {
  text-align: left;
  display: grid;
  gap: 4px;
}
.workbench .wb-split {
  display: grid;
  grid-template-columns: minmax(240px, 1fr) minmax(320px, 1.8fr);
  gap: 20px;
}
@media (max-width: 850px) {
  .workbench .wb-split {
    grid-template-columns: 1fr;
  }
}
</style>
