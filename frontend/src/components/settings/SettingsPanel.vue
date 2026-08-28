<template>
  <AppModal v-model="open" size="xl" :plain="embedded" :close-on-overlay="false">
    <template #header>
      <h1 class="text-[2.5rem] leading-none font-semibold tracking-tight">{{ t('settings.title') }}</h1>
      <p class="mt-2 text-sm text-muted-foreground">{{ t('settings.subtitle') }}</p>
    </template>

    <div class="space-y-5">
      <SegmentedPills
        :model-value="section"
        layout-id="settings-section-pill"
        dense
        :items="[
          { value: 'general', label: t('settings.tabGeneral') },
          { value: 'cli', label: t('settings.tabCli') },
          { value: 'dirs', label: t('settings.tabDirs') },
        ]"
        @update:model-value="onSection"
      />

      <CliUpdatePanel v-if="section === 'cli'" />
      <ConfigDirsPanel v-else-if="section === 'dirs'" />

      <template v-else>
      <Card>
        <CardHeader>
          <CardTitle>{{ t('settings.general') }}</CardTitle>
          <CardDescription>{{ t('settings.languageHint') }}</CardDescription>
        </CardHeader>
        <CardContent class="space-y-5">
          <div class="grid gap-2">
            <Label>{{ t('settings.language') }}</Label>
            <SegmentedPills
              :model-value="settings.language"
              layout-id="settings-lang-pill"
              dense
              :items="[
                { value: 'zh', label: t('settings.zh') },
                { value: 'en', label: t('settings.en') },
              ]"
              @update:model-value="onLanguage"
            />
          </div>
          <div class="grid gap-2">
            <div>
              <Label>{{ t('settings.theme') }}</Label>
              <p class="mt-1 text-xs text-muted-foreground">{{ t('settings.themeHint') }}</p>
            </div>
            <SegmentedPills
              :model-value="settings.theme"
              layout-id="settings-theme-pill"
              dense
              :items="[
                { value: 'system', label: t('settings.themeSystem') },
                { value: 'light', label: t('settings.themeLight') },
                { value: 'dark', label: t('settings.themeDark') },
              ]"
              @update:model-value="onTheme"
            />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{{ t('settings.appearance') }}</CardTitle>
          <CardDescription>{{ t('settings.accentHint') }}</CardDescription>
        </CardHeader>
        <CardContent>
          <Label>{{ t('settings.accent') }}</Label>
          <div class="mt-3 flex flex-wrap gap-2 py-1">
            <button
              v-for="item in ACCENTS"
              :key="item.id"
              type="button"
              class="flex size-9 items-center justify-center rounded-full outline-none transition-transform hover:scale-105"
              :aria-pressed="settings.accent === item.id"
              :aria-label="t(`settings.accent${capitalize(item.id)}`)"
              @click="onAccent(item.id)"
            >
              <span
                class="relative flex size-6 items-center justify-center rounded-full"
                :style="{ background: item.swatch }"
              >
                <Check v-if="settings.accent === item.id" class="size-3 text-white" />
              </span>
            </button>
          </div>
          <p class="mt-2 text-xs text-muted-foreground">{{ accentLabel }}</p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{{ t('settings.behavior') }}</CardTitle>
        </CardHeader>
        <CardContent class="space-y-4">
          <div class="flex items-start justify-between gap-4">
            <div class="min-w-0">
              <p class="text-sm font-medium">{{ t('settings.reducedMotion') }}</p>
              <p class="mt-0.5 text-xs text-muted-foreground">{{ t('settings.reducedMotionHint') }}</p>
            </div>
            <Switch :checked="settings.reducedMotion" @update:checked="onFlag('reducedMotion', $event, 'settings.reducedMotion')" />
          </div>
          <div class="flex items-start justify-between gap-4">
            <div class="min-w-0">
              <p class="text-sm font-medium">{{ t('settings.checkUpdateOnLaunch') }}</p>
              <p class="mt-0.5 text-xs text-muted-foreground">{{ t('settings.checkUpdateOnLaunchHint') }}</p>
            </div>
            <Switch :checked="settings.checkUpdateOnLaunch" @update:checked="onFlag('checkUpdateOnLaunch', $event, 'settings.checkUpdateOnLaunch')" />
          </div>
          <div class="flex items-start justify-between gap-4">
            <div class="min-w-0">
              <p class="text-sm font-medium">{{ t('settings.restoreLastPage') }}</p>
              <p class="mt-0.5 text-xs text-muted-foreground">{{ t('settings.restoreLastPageHint') }}</p>
            </div>
            <Switch :checked="settings.restoreLastPage" @update:checked="onFlag('restoreLastPage', $event, 'settings.restoreLastPage')" />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{{ t('settings.proxy') }}</CardTitle>
          <CardDescription>{{ t('settings.proxyHint') }}</CardDescription>
        </CardHeader>
        <CardContent class="space-y-4">
          <div class="flex items-start justify-between gap-4">
            <div class="min-w-0">
              <p class="text-sm font-medium">{{ t('settings.proxyEnable') }}</p>
              <p class="mt-0.5 text-xs text-muted-foreground">{{ t('settings.proxyEnableHint') }}</p>
            </div>
            <Switch :checked="proxy.enabled" :disabled="proxySaving" @update:checked="onProxyEnabled" />
          </div>
          <div class="grid gap-2">
            <Label>{{ t('settings.proxyType') }}</Label>
            <SegmentedPills
              :model-value="proxyProtocol"
              layout-id="settings-proxy-protocol"
              dense
              :items="[
                { value: 'http', label: t('settings.proxyHttp') },
                { value: 'socks5', label: t('settings.proxySocks') },
              ]"
              @update:model-value="onProxyProtocol"
            />
          </div>
          <AppInput
            v-model="proxy.url"
            :label="t('settings.proxyUrl')"
            :placeholder="proxyProtocol === 'socks5' ? 'socks5://127.0.0.1:7891' : 'http://127.0.0.1:7890'"
            :hint="t('settings.proxyUrlHint')"
            :disabled="proxySaving"
            @blur="() => saveProxy()"
          />
          <div class="flex flex-wrap items-center gap-2">
            <Button variant="outline" size="sm" :disabled="proxyTesting || proxySaving" @click="testProxy">
              <Loader2 v-if="proxyTesting" class="animate-spin" />
              <Wifi v-else />
              {{ proxyTesting ? t('settings.proxyTesting') : t('settings.proxyTest') }}
            </Button>
            <span v-if="proxyTestText" class="text-xs text-muted-foreground">{{ proxyTestText }}</span>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{{ t('settings.data') }}</CardTitle>
          <CardDescription>{{ t('settings.importHint') }}</CardDescription>
        </CardHeader>
        <CardContent class="flex flex-wrap gap-2">
          <Button variant="outline" @click="$emit('export')">
            <Download />
            {{ t('settings.export') }}
          </Button>
          <Button variant="outline" @click="$emit('import')">
            <Upload />
            {{ t('settings.import') }}
          </Button>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{{ t('settings.about') }}</CardTitle>
          <CardDescription>{{ t('settings.shortcut') }}</CardDescription>
        </CardHeader>
        <CardContent class="space-y-4">
          <div class="flex items-center justify-between gap-3">
            <div>
              <p class="text-sm font-medium">{{ t('settings.version') }}</p>
              <p class="mt-0.5 font-mono text-xs text-muted-foreground">v{{ appVersion }}</p>
            </div>
            <Button variant="outline" size="sm" @click="$emit('checkUpdate')">
              <ArrowUpCircle />
              {{ t('settings.checkUpdate') }}
            </Button>
          </div>
          <div class="flex items-center justify-between gap-3">
            <div class="min-w-0">
              <p class="text-sm font-medium">{{ t('settings.github') }}</p>
              <p class="mt-0.5 truncate text-xs text-muted-foreground">{{ t('settings.githubHint') }}</p>
            </div>
            <Button variant="outline" size="sm" @click="openGithub">
              <FolderGit2 />
              {{ t('settings.openGithub') }}
            </Button>
          </div>
        </CardContent>
      </Card>
      </template>
    </div>
  </AppModal>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ArrowUpCircle, Check, Download, FolderGit2, Loader2, Upload, Wifi } from '@lucide/vue'
import { ACCENTS, useSettings, type AccentId, type ThemeMode } from '@/composables/useSettings'
import { useI18n } from '@/composables/useI18n'
import { useToast } from '@/composables/useToast'
import { updateService } from '@/services/updateService'
import { callApp, pickBool, pickNum, pickText } from '@/services/appBridge'
import type { OutboundProxySettings, ProxyTestResult } from '@/types'
import AppModal from '@/components/common/AppModal.vue'
import AppInput from '@/components/common/AppInput.vue'
import SegmentedPills from '@/components/layout/SegmentedPills.vue'
import CliUpdatePanel from '@/components/settings/CliUpdatePanel.vue'
import ConfigDirsPanel from '@/components/settings/ConfigDirsPanel.vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import type { Locale } from '@/i18n'

const props = withDefaults(defineProps<{
  modelValue?: boolean
  embedded?: boolean
}>(), {
  modelValue: true,
  embedded: true,
})

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  checkUpdate: []
  export: []
  import: []
}>()

const open = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value),
})

const { settings } = useSettings()
const { t } = useI18n()
const toast = useToast()
const appVersion = ref('2.3.0')
const section = ref<'general' | 'cli' | 'dirs'>('general')
const proxy = reactive<OutboundProxySettings>({ enabled: false, url: 'http://127.0.0.1:7890' })
const proxySaving = ref(false)
const proxyTesting = ref(false)
const proxyTestText = ref('')

const proxyProtocol = computed(() => proxy.url.trim().toLowerCase().startsWith('socks5') ? 'socks5' : 'http')

const accentLabel = computed(() => t(`settings.accent${capitalize(settings.accent)}`))

function capitalize(id: AccentId) {
  return id.charAt(0).toUpperCase() + id.slice(1)
}

function onSection(value: string) {
  if (value === 'general' || value === 'cli' || value === 'dirs') section.value = value
}

function onLanguage(value: string) {
  if (value !== 'zh' && value !== 'en') return
  if (settings.language === value) return
  settings.language = value as Locale
  toast.success(t('toast.languageSet', { name: t(value === 'zh' ? 'settings.zh' : 'settings.en') }))
}

function onTheme(value: string) {
  if (value !== 'system' && value !== 'light' && value !== 'dark') return
  if (settings.theme === value) return
  settings.theme = value as ThemeMode
  const name = value === 'system'
    ? t('settings.themeSystem')
    : value === 'light'
      ? t('settings.themeLight')
      : t('settings.themeDark')
  toast.success(t('toast.themeSet', { name }))
}

function onAccent(id: AccentId) {
  if (settings.accent === id) return
  settings.accent = id
  toast.success(t('toast.accentSet', { name: t(`settings.accent${capitalize(id)}`) }))
}

function onFlag(key: 'reducedMotion' | 'checkUpdateOnLaunch' | 'restoreLastPage', value: boolean, nameKey: string) {
  settings[key] = value
  toast.success(value ? t('toast.settingOn', { name: t(nameKey) }) : t('toast.settingOff', { name: t(nameKey) }))
}

function rewriteProxyScheme(url: string, protocol: 'http' | 'socks5') {
  const trimmed = url.trim() || (protocol === 'socks5' ? '127.0.0.1:7891' : '127.0.0.1:7890')
  const withoutScheme = trimmed.replace(/^(https?|socks5h?):\/\//i, '')
  return `${protocol}://${withoutScheme}`
}

function onProxyProtocol(value: string) {
  if (value !== 'http' && value !== 'socks5') return
  proxy.url = rewriteProxyScheme(proxy.url, value)
  saveProxy('saved')
}

async function onProxyEnabled(value: boolean) {
  proxy.enabled = value
  await saveProxy('toggle')
}

async function saveProxy(kind: 'silent' | 'saved' | 'toggle' = 'silent') {
  if (proxySaving.value) return
  proxySaving.value = true
  try {
    const payload: OutboundProxySettings = {
      enabled: proxy.enabled,
      url: proxy.url.trim(),
    }
    await callApp('SetOutboundProxy', payload)
    const applied = await callApp<OutboundProxySettings>('GetOutboundProxy')
    proxy.enabled = pickBool(applied, 'enabled', 'Enabled')
    const url = pickText(applied, 'url', 'URL', 'Url')
    if (url) proxy.url = url
    if (kind === 'saved') toast.success(t('settings.proxySaved'))
    if (kind === 'toggle') {
      toast.success(proxy.enabled
        ? t('toast.settingOn', { name: t('settings.proxyEnable') })
        : t('toast.settingOff', { name: t('settings.proxyEnable') }))
    }
  } catch (e: unknown) {
    toast.error(t('settings.proxySaveFailed', { error: e instanceof Error ? e.message : String(e) }))
  } finally {
    proxySaving.value = false
  }
}

async function testProxy() {
  if (proxyTesting.value) return
  proxyTesting.value = true
  proxyTestText.value = ''
  try {
    const result = await callApp<ProxyTestResult>('TestOutboundProxy', {
      enabled: proxy.enabled,
      url: proxy.url.trim(),
    })
    const message = pickText(result, 'message', 'Message')
    const latency = pickNum(result, 'latency', 'Latency')
    if (pickBool(result, 'success', 'Success')) {
      proxyTestText.value = t('settings.proxyTestOk', { latency })
      toast.success(message)
    } else {
      proxyTestText.value = message
      toast.error(t('settings.proxyTestFail', { error: message }))
    }
  } catch (e: unknown) {
    const message = e instanceof Error ? e.message : String(e)
    proxyTestText.value = message
    toast.error(t('settings.proxyTestFail', { error: message }))
  } finally {
    proxyTesting.value = false
  }
}

function openGithub() {
  updateService.openUrl('https://github.com/nsmao-com/claude-code-env-change')
}

onMounted(async () => {
  try {
    appVersion.value = await updateService.version()
  } catch { /* ignore */ }
  try {
    const current = await callApp<OutboundProxySettings>('GetOutboundProxy')
    proxy.enabled = pickBool(current, 'enabled', 'Enabled')
    proxy.url = pickText(current, 'url', 'URL', 'Url') || 'http://127.0.0.1:7890'
  } catch { /* ignore */ }
})
</script>
