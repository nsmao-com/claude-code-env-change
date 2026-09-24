<template>
  <AppModal v-model="isOpen" :title="isEditing ? t('envForm.titleEdit') : t('envForm.titleNew')" size="lg" :close-on-overlay="false">
    <form class="space-y-4" @submit.prevent="handleSubmit">
      <div class="grid grid-cols-2 gap-4">
        <div class="col-span-2 sm:col-span-1">
          <AppInput v-model="form.name" :label="t('envForm.name')" :placeholder="t('envForm.namePlaceholder')" :tooltip="tips.name" />
        </div>
        <div class="col-span-2 sm:col-span-1">
          <FieldLabel :label="t('envForm.icon')" :hint="tips.icon" />
          <div class="relative mt-1.5">
            <Button type="button" variant="outline" size="icon" class="text-xl" @click="showEmojiPicker = !showEmojiPicker">
              <ConfigIcon :value="form.icon" class="size-5" />
            </Button>
            <EmojiPicker :show="showEmojiPicker" :current="form.icon" @close="showEmojiPicker = false" @select="selectIcon" />
          </div>
        </div>
        <div class="col-span-2">
          <AppInput v-model="form.description" :label="t('envForm.description')" :placeholder="t('envForm.descriptionPlaceholder')" :tooltip="tips.description" />
        </div>
      </div>

      <SegmentedPills
        :model-value="form.provider"
        layout-id="config-provider-pill"
        full
        dense
        :items="providers.map(p => ({ value: p.value, label: p.label }))"
        @update:model-value="onProvider"
      >
        <template #default="{ item }">
          <BrandIcon :provider="item.value" class="size-3.5" />
          {{ item.label }}
        </template>
      </SegmentedPills>

      <div
        v-if="officialLogin"
        class="rounded-xl border border-emerald-500/30 bg-emerald-500/10 px-3 py-2 text-xs leading-relaxed text-emerald-700 dark:text-emerald-400"
      >
        {{ t('envForm.officialNote.before') }}<b>{{ t('envForm.officialNote.bold') }}</b>{{ t('envForm.officialNote.after') }}
      </div>

      <div class="flex flex-wrap items-center gap-2 rounded-xl bg-muted/40 px-3 py-2">
        <span class="text-xs text-muted-foreground">{{ t('envForm.quickFill') }}</span>
        <Button v-for="preset in providerPresets" :key="preset.label" type="button" size="sm" variant="outline" @click="applyPreset(preset)">
          {{ preset.label }}
        </Button>
      </div>

      <div class="grid gap-1.5">
        <FieldLabel :label="t('envForm.upstreamFormat')" :hint="tips.upstreamAdvanced" />
        <Select v-model="upstreamSelect">
          <SelectTrigger class="w-full">
            <SelectValue :placeholder="t('envForm.upstreamPlaceholder')" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem v-for="opt in upstreamOptions" :key="opt.value" :value="opt.value">
              {{ opt.label }}
            </SelectItem>
          </SelectContent>
        </Select>
        <p class="text-xs leading-relaxed text-muted-foreground">{{ upstreamHint }}</p>
      </div>

      <div v-if="form.provider === 'claude' || form.provider === 'claude_desktop'" class="space-y-4">
        <div v-if="!isEditing && vendorPresets.length" class="space-y-1.5">
          <p class="text-xs font-medium tracking-wide text-muted-foreground uppercase">{{ t('envForm.vendorPresets') }}</p>
          <div class="flex flex-wrap gap-1.5">
            <Button
              v-for="preset in vendorPresets"
              :key="preset.id"
              type="button"
              variant="outline"
              size="sm"
              class="h-7 rounded-full px-2.5 text-xs"
              :title="preset.description"
              @click="applyVendorPreset(preset)"
            >
              <span>{{ preset.icon }}</span>
              <span>{{ preset.name }}</span>
            </Button>
          </div>
        </div>
        <AppInput v-model="form.claude.baseUrl" label="Base URL" placeholder="https://api.anthropic.com" :tooltip="tips.baseUrlClaude">
          <template #suffix>
            <Button type="button" variant="ghost" size="icon-xs" :disabled="latencyTesting" @click="testLatency(form.claude.baseUrl)">
              <Loader2 v-if="latencyTesting" class="animate-spin" />
              <Zap v-else />
            </Button>
          </template>
        </AppInput>
        <AppInput v-model="form.claude.authToken" label="Auth Token" :placeholder="t('envForm.optional')" :tooltip="tips.authToken" />
        <AppInput v-model="form.claude.model" label="Model" placeholder="claude-sonnet-5" :tooltip="tips.modelClaude" />
        <AppInput
          v-model="form.claude.apiKey"
          label="API Key"
          :type="showApiKey.claude ? 'text' : 'password'"
          placeholder="sk-ant-..."
          :tooltip="tips.apiKeyClaude"
        >
          <template #suffix>
            <Button type="button" variant="ghost" size="icon-xs" @click="toggleApiKeyVisibility('claude')">
              <EyeOff v-if="showApiKey.claude" />
              <Eye v-else />
            </Button>
          </template>
        </AppInput>

        <div v-if="form.provider === 'claude'" class="space-y-3 border-t pt-3">
          <p class="text-xs font-medium tracking-wide text-muted-foreground uppercase">{{ t('envForm.claudeEnvVars') }}</p>
          <div class="flex items-center justify-between gap-3">
            <div>
              <FieldLabel label="Attribution Header" :hint="tips.attributionHeader" />
              <div class="font-mono text-[11px] text-muted-foreground">CLAUDE_CODE_ATTRIBUTION_HEADER</div>
            </div>
            <SegmentedPills
              :model-value="triValue(form.claude.attributionHeader)"
              layout-id="cfg-attr-header"
              dense
              :items="triItems"
              @update:model-value="v => form.claude.attributionHeader = fromTri(v)"
            />
          </div>
          <div class="flex items-center justify-between gap-3">
            <div>
              <FieldLabel label="Disable Nonessential Traffic" :hint="tips.disableNonessential" />
              <div class="font-mono text-[11px] text-muted-foreground">CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC</div>
            </div>
            <SegmentedPills
              :model-value="triValue(form.claude.disableNonessentialTraffic)"
              layout-id="cfg-nonessential"
              dense
              :items="triItems"
              @update:model-value="v => form.claude.disableNonessentialTraffic = fromTri(v)"
            />
          </div>
          <AppInput v-model="form.claude.smallFastModel" :label="t('envForm.smallFastModel')" placeholder="ANTHROPIC_SMALL_FAST_MODEL" :tooltip="tips.smallFastModel" />
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <AppInput v-model="form.claude.defaultHaiku" label="Default Haiku" placeholder="ANTHROPIC_DEFAULT_HAIKU_MODEL" :tooltip="tips.defaultHaiku" />
            <AppInput v-model="form.claude.defaultSonnet" label="Default Sonnet" placeholder="ANTHROPIC_DEFAULT_SONNET_MODEL" :tooltip="tips.defaultSonnet" />
            <AppInput v-model="form.claude.defaultOpus" label="Default Opus" placeholder="ANTHROPIC_DEFAULT_OPUS_MODEL" :tooltip="tips.defaultOpus" />
          </div>
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <AppInput v-model="form.claude.maxOutputTokens" :label="t('envForm.maxOutputTokens')" placeholder="CLAUDE_CODE_MAX_OUTPUT_TOKENS" :tooltip="tips.maxOutputClaude" />
            <AppInput v-model="form.claude.autocompactPct" :label="t('envForm.autocompactPct')" placeholder="CLAUDE_AUTOCOMPACT_PCT_OVERRIDE" :tooltip="tips.autocompactPct" />
          </div>
          <div class="space-y-3 rounded-xl bg-muted/40 p-3">
            <p class="text-xs font-medium tracking-wide text-muted-foreground uppercase">{{ t('envForm.thinking') }}</p>
            <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div class="grid gap-1.5">
                <FieldLabel :label="t('envForm.effort')" :hint="tips.claudeEffort" />
                <Select v-model="form.claude.effortLevel">
                  <SelectTrigger class="w-full">
                    <SelectValue placeholder="CLAUDE_CODE_EFFORT_LEVEL" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem v-for="item in claudeEffortItems" :key="item.value" :value="item.value">{{ item.label }}</SelectItem>
                  </SelectContent>
                </Select>
                <p class="font-mono text-[11px] text-muted-foreground">CLAUDE_CODE_EFFORT_LEVEL</p>
              </div>
              <AppInput v-model="form.claude.maxThinkingTokens" :label="t('envForm.maxThinkingTokens')" :placeholder="t('envForm.maxThinkingPlaceholder')" :tooltip="tips.maxThinking" />
            </div>
            <div class="flex items-center justify-between gap-3">
              <div>
                <FieldLabel :label="t('envForm.disableAdaptiveThinking')" :hint="tips.disableAdaptiveThinking" />
                <div class="font-mono text-[11px] text-muted-foreground">CLAUDE_CODE_DISABLE_ADAPTIVE_THINKING</div>
              </div>
              <SegmentedPills
                :model-value="triValue(form.claude.disableAdaptiveThinking)"
                layout-id="cfg-adaptive-thinking"
                dense
                :items="triItems"
                @update:model-value="v => form.claude.disableAdaptiveThinking = fromTri(v)"
              />
            </div>
          </div>
          <div class="flex items-center justify-between gap-3">
            <div>
              <FieldLabel :label="t('envForm.disableAutocompact')" :hint="tips.disableAutocompact" />
              <div class="font-mono text-[11px] text-muted-foreground">DISABLE_AUTOCOMPACT</div>
            </div>
            <SegmentedPills
              :model-value="triValue(form.claude.disableAutocompact)"
              layout-id="cfg-autocompact"
              dense
              :items="triItems"
              @update:model-value="v => form.claude.disableAutocompact = fromTri(v)"
            />
          </div>
        </div>
        <div v-if="form.provider === 'claude_desktop'" class="space-y-3 rounded-xl border border-border/70 bg-muted/30 p-3">
          <p class="text-sm font-medium">{{ t('envForm.desktopTitle') }}</p>
          <p class="text-xs leading-relaxed text-muted-foreground">
            {{ t('envForm.desktopNote') }}
          </p>
          <div class="grid gap-1.5">
            <FieldLabel :label="t('envForm.desktopTemplate')" :hint="t('envForm.desktopTemplateHint')" />
            <CodeEditor v-if="form.provider === 'claude_desktop'" v-model="form.claude.desktopTemplate" language="json" :placeholder="t('envForm.desktopTemplatePlaceholder')" class="min-h-32" />
          </div>
        </div>

        <Button v-if="form.provider === 'claude'" type="button" variant="ghost" size="sm" @click="showMore = !showMore">
          {{ showMore ? t('envForm.moreCollapse') : t('envForm.more') }}
        </Button>
        <div v-if="form.provider === 'claude' && showMore" class="space-y-3 border-t pt-3">
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <AppInput v-model="form.claude.httpProxy" label="HTTP_PROXY" placeholder="http://127.0.0.1:7890" :tooltip="tips.httpProxy" />
            <AppInput v-model="form.claude.httpsProxy" label="HTTPS_PROXY" placeholder="http://127.0.0.1:7890" :tooltip="tips.httpsProxy" />
            <AppInput v-model="form.claude.bashDefaultTimeout" :label="t('envForm.bashDefaultTimeout')" placeholder="BASH_DEFAULT_TIMEOUT_MS" :tooltip="tips.bashDefaultTimeout" />
            <AppInput v-model="form.claude.bashMaxTimeout" :label="t('envForm.bashMaxTimeout')" placeholder="BASH_MAX_TIMEOUT_MS" :tooltip="tips.bashMaxTimeout" />
            <AppInput v-model="form.claude.bashMaxOutput" :label="t('envForm.bashMaxOutput')" placeholder="BASH_MAX_OUTPUT_LENGTH" :tooltip="tips.bashMaxOutput" />
            <AppInput v-model="form.claude.maxMcpOutputTokens" :label="t('envForm.maxMcpOutput')" placeholder="MAX_MCP_OUTPUT_TOKENS" :tooltip="tips.maxMcpOutput" />
            <AppInput v-model="form.claude.mcpTimeout" :label="t('envForm.mcpTimeout')" placeholder="MCP_TIMEOUT" :tooltip="tips.mcpTimeout" />
          </div>
          <div class="flex items-center justify-between gap-3">
            <div>
              <FieldLabel label="Disable Telemetry" :hint="tips.disableTelemetry" />
              <div class="font-mono text-[11px] text-muted-foreground">DISABLE_TELEMETRY</div>
            </div>
            <SegmentedPills
              :model-value="triValue(form.claude.disableTelemetry)"
              layout-id="cfg-telemetry"
              dense
              :items="triItems"
              @update:model-value="v => form.claude.disableTelemetry = fromTri(v)"
            />
          </div>
          <div class="flex items-center justify-between gap-3">
            <div>
              <FieldLabel label="Disable Error Reporting" :hint="tips.disableErrorReporting" />
              <div class="font-mono text-[11px] text-muted-foreground">DISABLE_ERROR_REPORTING</div>
            </div>
            <SegmentedPills
              :model-value="triValue(form.claude.disableErrorReporting)"
              layout-id="cfg-error-reporting"
              dense
              :items="triItems"
              @update:model-value="v => form.claude.disableErrorReporting = fromTri(v)"
            />
          </div>
          <div class="flex items-center justify-between gap-3">
            <div>
              <FieldLabel :label="t('envForm.alwaysEnableEffort')" :hint="tips.alwaysEnableEffort" />
              <div class="font-mono text-[11px] text-muted-foreground">CLAUDE_CODE_ALWAYS_ENABLE_EFFORT</div>
            </div>
            <SegmentedPills
              :model-value="triValue(form.claude.alwaysEnableEffort)"
              layout-id="cfg-always-effort"
              dense
              :items="triItems"
              @update:model-value="v => form.claude.alwaysEnableEffort = fromTri(v)"
            />
          </div>
        </div>
      </div>

      <div v-if="form.provider === 'codex'" class="space-y-4">
        <AppInput v-model="form.codex.baseUrl" label="Base URL" placeholder="https://api.openai.com/v1" :tooltip="tips.baseUrlCodex">
          <template #suffix>
            <Button type="button" variant="ghost" size="icon-xs" :disabled="latencyTesting" @click="testLatency(form.codex.baseUrl)">
              <Loader2 v-if="latencyTesting" class="animate-spin" />
              <Zap v-else />
            </Button>
          </template>
        </AppInput>
        <AppInput
          v-model="form.codex.apiKey"
          label="API Key"
          :type="showApiKey.codex ? 'text' : 'password'"
          placeholder="sk-..."
          :tooltip="tips.apiKeyCodex"
        >
          <template #suffix>
            <Button type="button" variant="ghost" size="icon-xs" @click="toggleApiKeyVisibility('codex')">
              <EyeOff v-if="showApiKey.codex" />
              <Eye v-else />
            </Button>
          </template>
        </AppInput>
        <AppInput v-model="form.codex.model" label="Model" placeholder="gpt-5.4" :tooltip="tips.modelCodex" />
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <AppInput v-model="form.codex.contextWindow" :label="t('envForm.contextWindow')" placeholder="model_context_window" :tooltip="tips.contextWindowCodex" />
          <AppInput v-model="form.codex.maxOutputTokens" :label="t('envForm.maxOutputTokens')" placeholder="model_max_output_tokens" :tooltip="tips.maxOutputCodex" />
        </div>
        <div class="space-y-3 rounded-xl bg-muted/40 p-3">
          <p class="text-xs font-medium tracking-wide text-muted-foreground uppercase">{{ t('envForm.thinking') }}</p>
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div class="grid gap-1.5">
              <FieldLabel :label="t('envForm.effort')" :hint="tips.reasoningEffort" />
              <Select v-model="form.codex.reasoningEffort">
                <SelectTrigger class="w-full">
                  <SelectValue placeholder="model_reasoning_effort" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="item in openaiEffortItems" :key="item.value" :value="item.value">{{ item.label }}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div class="grid gap-1.5">
              <FieldLabel :label="t('envForm.planEffort')" :hint="tips.planReasoningEffort" />
              <Select v-model="form.codex.planReasoningEffort">
                <SelectTrigger class="w-full">
                  <SelectValue placeholder="plan_mode_reasoning_effort" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="item in openaiEffortItems" :key="'plan-' + item.value" :value="item.value">{{ item.label }}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div class="grid gap-1.5">
              <FieldLabel :label="t('envForm.reasoningSummary')" :hint="tips.reasoningSummary" />
              <Select v-model="form.codex.reasoningSummary">
                <SelectTrigger class="w-full">
                  <SelectValue placeholder="model_reasoning_summary" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="item in reasoningSummaryItems" :key="item.value" :value="item.value">{{ item.label }}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div class="grid gap-1.5">
              <FieldLabel :label="t('envForm.verbosity')" :hint="tips.modelVerbosity" />
              <Select v-model="form.codex.verbosity">
                <SelectTrigger class="w-full">
                  <SelectValue placeholder="model_verbosity" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="unset">{{ t('envForm.unset') }}</SelectItem>
                  <SelectItem value="low">low</SelectItem>
                  <SelectItem value="medium">medium</SelectItem>
                  <SelectItem value="high">high</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        </div>
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div class="grid gap-1.5">
            <FieldLabel :label="t('envForm.approvalPolicy')" :hint="tips.approvalPolicy" />
            <Select v-model="form.codex.approvalPolicy">
              <SelectTrigger class="w-full">
                <SelectValue placeholder="approval_policy" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="unset">{{ t('envForm.unset') }}</SelectItem>
                <SelectItem value="untrusted">untrusted</SelectItem>
                <SelectItem value="on-failure">on-failure</SelectItem>
                <SelectItem value="on-request">on-request</SelectItem>
                <SelectItem value="never">never</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="grid gap-1.5">
            <FieldLabel :label="t('envForm.sandbox')" :hint="tips.sandboxCodex" />
            <Select v-model="form.codex.sandboxMode">
              <SelectTrigger class="w-full">
                <SelectValue placeholder="sandbox_mode" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="unset">{{ t('envForm.unset') }}</SelectItem>
                <SelectItem value="read-only">read-only</SelectItem>
                <SelectItem value="workspace-write">workspace-write</SelectItem>
                <SelectItem value="danger-full-access">danger-full-access</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
        <div class="grid gap-1.5">
          <FieldLabel :label="t('envForm.configTomlTemplate')" :hint="tips.codexToml" />
          <CodeEditor v-model="form.codex.configTemplate" language="toml" :placeholder="t('envForm.tomlPlaceholder')" class="min-h-32" />
        </div>
        <div class="grid gap-1.5">
          <FieldLabel :label="t('envForm.authJsonTemplate')" :hint="tips.codexAuth" />
          <CodeEditor v-model="form.codex.authTemplate" language="json" :placeholder="t('envForm.authPlaceholder')" class="min-h-24" />
        </div>
      </div>

      <div v-if="form.provider === 'antigravity'" class="space-y-4">
        <AppInput v-model="form.antigravity.baseUrl" label="Base URL" placeholder="https://generativelanguage.googleapis.com" :tooltip="tips.baseUrlGemini">
          <template #suffix>
            <Button type="button" variant="ghost" size="icon-xs" :disabled="latencyTesting" @click="testLatency(form.antigravity.baseUrl)">
              <Loader2 v-if="latencyTesting" class="animate-spin" />
              <Zap v-else />
            </Button>
          </template>
        </AppInput>
        <AppInput
          v-model="form.antigravity.apiKey"
          label="API Key"
          :type="showApiKey.antigravity ? 'text' : 'password'"
          placeholder="API Key"
          :tooltip="tips.apiKeyGemini"
        >
          <template #suffix>
            <Button type="button" variant="ghost" size="icon-xs" @click="toggleApiKeyVisibility('antigravity')">
              <EyeOff v-if="showApiKey.antigravity" />
              <Eye v-else />
            </Button>
          </template>
        </AppInput>
        <AppInput v-model="form.antigravity.model" label="Model" placeholder="gemini-3.1-pro-preview" :tooltip="tips.modelGemini" />
        <div class="space-y-3 rounded-xl bg-muted/40 p-3">
          <p class="text-xs font-medium tracking-wide text-muted-foreground uppercase">{{ t('envForm.thinking') }}</p>
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div class="grid gap-1.5">
              <FieldLabel :label="t('envForm.geminiLevel')" :hint="tips.geminiThinkingLevel" />
              <Select v-model="form.antigravity.thinkingLevel">
                <SelectTrigger class="w-full">
                  <SelectValue placeholder="thinkingLevel" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="item in geminiLevelItems" :key="item.value" :value="item.value">{{ item.label }}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <AppInput v-model="form.antigravity.thinkingBudget" :label="t('envForm.geminiBudget')" :placeholder="t('envForm.geminiBudgetPlaceholder')" :tooltip="tips.geminiThinkingBudget" />
          </div>
        </div>
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <AppInput v-model="form.antigravity.project" label="Google Cloud Project" placeholder="GOOGLE_CLOUD_PROJECT" :tooltip="tips.geminiProject" />
          <AppInput v-model="form.antigravity.location" label="Location" placeholder="GOOGLE_CLOUD_LOCATION" :tooltip="tips.geminiLocation" />
          <AppInput v-model="form.antigravity.useVertex" :label="t('envForm.useVertex')" :placeholder="t('envForm.useVertexPlaceholder')" :tooltip="tips.geminiVertex" />
          <AppInput v-model="form.antigravity.sandbox" label="Sandbox" placeholder="GEMINI_SANDBOX" :tooltip="tips.geminiSandbox" />
          <AppInput v-model="form.antigravity.maxSessionTurns" :label="t('envForm.maxSessionTurns')" placeholder="maxSessionTurns" :tooltip="tips.geminiTurns" />
          <AppInput v-model="form.antigravity.compressionThreshold" :label="t('envForm.compressionThreshold')" placeholder="0.7" :tooltip="tips.geminiCompress" />
        </div>
        <div class="grid gap-1.5">
          <FieldLabel :label="t('envForm.envTemplate')" :hint="tips.geminiEnv" />
          <CodeEditor v-model="form.antigravity.envTemplate" language="env" :placeholder="t('envForm.envPlaceholder')" class="min-h-24" />
        </div>
        <div class="grid gap-1.5">
          <FieldLabel :label="t('envForm.settingsTemplate')" :hint="tips.geminiSettings" />
          <CodeEditor v-model="form.antigravity.settingsTemplate" language="json" :placeholder="t('envForm.settingsPlaceholder')" class="min-h-24" />
        </div>
      </div>

      <div v-if="form.provider === 'opencode'" class="space-y-4">
        <div class="rounded-lg border bg-muted/40 p-3">
          <p class="text-xs leading-relaxed text-muted-foreground">
            {{ t('envForm.opencodeNote.before') }}
            <span class="font-mono">~/.config/opencode/opencode.json</span>{{ t('envForm.opencodeNote.mid') }}
            <span class="font-mono">OPENCODE_CONFIG_DIR / OPENCODE_CONFIG</span>{{ t('envForm.opencodeNote.after') }}
          </p>
        </div>
        <AppInput v-model="form.opencode.baseUrl" label="Base URL" placeholder="https://your-gateway/v1" :tooltip="tips.baseUrlOpencode">
          <template #suffix>
            <Button type="button" variant="ghost" size="icon-xs" :disabled="latencyTesting" @click="testLatency(form.opencode.baseUrl)">
              <Loader2 v-if="latencyTesting" class="animate-spin" />
              <Zap v-else />
            </Button>
          </template>
        </AppInput>
        <AppInput
          v-model="form.opencode.apiKey"
          label="API Key"
          :type="showApiKey.opencode ? 'text' : 'password'"
          :placeholder="t('envForm.optional')"
          :tooltip="tips.apiKeyOpencode"
        >
          <template #suffix>
            <Button type="button" variant="ghost" size="icon-xs" @click="toggleApiKeyVisibility('opencode')">
              <EyeOff v-if="showApiKey.opencode" />
              <Eye v-else />
            </Button>
          </template>
        </AppInput>
        <AppInput v-model="form.opencode.model" label="Model" placeholder="anthropic/claude-sonnet-4" :tooltip="tips.modelOpencode" />
        <div class="space-y-3 rounded-xl bg-muted/40 p-3">
          <p class="text-xs font-medium tracking-wide text-muted-foreground uppercase">{{ t('envForm.thinking') }}</p>
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div class="grid gap-1.5">
              <FieldLabel :label="t('envForm.effort')" :hint="tips.opencodeEffort" />
              <Select v-model="form.opencode.reasoningEffort">
                <SelectTrigger class="w-full">
                  <SelectValue placeholder="reasoningEffort" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="item in openaiEffortItems" :key="'oc-' + item.value" :value="item.value">{{ item.label }}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <AppInput v-model="form.opencode.thinkingBudget" :label="t('envForm.anthropicBudget')" :placeholder="t('envForm.anthropicBudgetPlaceholder')" :tooltip="tips.opencodeThinkingBudget" />
            <div class="grid gap-1.5">
              <FieldLabel :label="t('envForm.reasoningSummary')" :hint="tips.opencodeReasoningSummary" />
              <Select v-model="form.opencode.reasoningSummary">
                <SelectTrigger class="w-full">
                  <SelectValue placeholder="reasoningSummary" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="item in reasoningSummaryItems" :key="'oc-sum-' + item.value" :value="item.value">{{ item.label }}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        </div>
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <AppInput v-model="form.opencode.smallModel" :label="t('envForm.smallModel')" placeholder="openai/gpt-4.1-nano" :tooltip="tips.opencodeSmall" />
          <AppInput v-model="form.opencode.username" label="Username" :placeholder="t('envForm.displayName')" :tooltip="tips.opencodeUser" />
          <AppInput v-model="form.opencode.share" label="Share" placeholder="manual / auto / disabled" :tooltip="tips.opencodeShare" />
          <AppInput v-model="form.opencode.autoupdate" label="Autoupdate" placeholder="true / false" :tooltip="tips.opencodeAutoupdate" />
          <AppInput v-model="form.opencode.snapshot" label="Snapshot" placeholder="true / false" :tooltip="tips.opencodeSnapshot" />
        </div>
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <AppInput v-model="form.opencode.configDir" :label="t('envForm.configDirOptional')" placeholder="~/.config/opencode" :tooltip="tips.opencodeDir" />
          <AppInput v-model="form.opencode.configPath" :label="t('envForm.configPathOptional')" placeholder="~/.config/opencode/opencode.json" :tooltip="tips.opencodePath" />
        </div>
        <div class="grid gap-1.5">
          <FieldLabel :label="t('envForm.opencodeJsonOptional')" :hint="tips.opencodeJson" />
          <CodeEditor v-model="form.opencode.configTemplate" language="json" :placeholder="t('envForm.opencodeJsonPlaceholder')" class="min-h-32" />
        </div>
      </div>

      <div v-if="form.provider === 'grok'" class="space-y-4">
        <div class="rounded-lg border bg-muted/40 p-3">
          <p class="text-xs leading-relaxed text-muted-foreground">
            {{ t('envForm.grokNote.a') }}
            <span class="font-mono">~/.grok/config.toml</span>
            {{ t('envForm.grokNote.b') }}
            <span class="font-mono">XAI_API_KEY</span>
            {{ t('envForm.grokNote.c') }} <span class="font-mono">api_key</span>{{ t('envForm.grokNote.d') }}
          </p>
        </div>
        <AppInput v-model="form.grok.baseUrl" label="Base URL" placeholder="https://api.x.ai/v1" :tooltip="tips.baseUrlGrok">
          <template #suffix>
            <Button type="button" variant="ghost" size="icon-xs" :disabled="latencyTesting" @click="testLatency(form.grok.baseUrl)">
              <Loader2 v-if="latencyTesting" class="animate-spin" />
              <Zap v-else />
            </Button>
          </template>
        </AppInput>
        <AppInput
          v-model="form.grok.apiKey"
          label="API Key"
          :type="showApiKey.grok ? 'text' : 'password'"
          placeholder="xai-..."
          :tooltip="tips.apiKeyGrok"
        >
          <template #suffix>
            <Button type="button" variant="ghost" size="icon-xs" @click="toggleApiKeyVisibility('grok')">
              <EyeOff v-if="showApiKey.grok" />
              <Eye v-else />
            </Button>
          </template>
        </AppInput>
        <AppInput v-model="form.grok.model" label="Model" placeholder="grok-4.6" :tooltip="tips.modelGrok" />
        <div class="space-y-3 rounded-xl bg-muted/40 p-3">
          <p class="text-xs font-medium tracking-wide text-muted-foreground uppercase">{{ t('envForm.thinking') }}</p>
          <div class="grid gap-1.5">
            <FieldLabel :label="t('envForm.effort')" :hint="tips.grokEffort" />
            <Select v-model="form.grok.reasoningEffort">
              <SelectTrigger class="w-full">
                <SelectValue placeholder="reasoning_effort" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="item in grokEffortItems" :key="item.value" :value="item.value">{{ item.label }}</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <AppInput v-model="form.grok.modelName" :label="t('envForm.displayName')" placeholder="Grok" :tooltip="tips.grokName" />
          <AppInput v-model="form.grok.contextWindow" :label="t('envForm.contextWindow')" placeholder="131072" :tooltip="tips.grokContext" />
          <AppInput v-model="form.grok.maxTokens" :label="t('envForm.maxOutputTokens')" placeholder="8192" :tooltip="tips.grokMaxTokens" />
          <AppInput v-model="form.grok.temperature" label="Temperature" placeholder="0.7" :tooltip="tips.grokTemp" />
        </div>
        <div class="grid gap-1.5">
          <FieldLabel label="API Backend" :hint="tips.grokBackend" />
          <SegmentedPills
            :model-value="form.grok.apiBackend"
            layout-id="grok-backend-pill"
            full
            dense
            :items="grokBackends"
            @update:model-value="v => { if (v === 'responses' || v === 'chat_completions' || v === 'messages') form.grok.apiBackend = v }"
          />
        </div>
        <AppInput v-model="form.grok.homeDir" :label="t('envForm.grokHomeOptional')" placeholder="~/.grok" :tooltip="tips.grokHome" />
        <div class="grid gap-1.5">
          <FieldLabel :label="t('envForm.grokTomlOptional')" :hint="tips.grokToml" />
          <CodeEditor v-model="form.grok.configTemplate" language="toml" :placeholder="t('envForm.grokTomlPlaceholder')" class="min-h-32" />
        </div>
      </div>

    </form>

    <template #footer>
      <Button
        type="button"
        variant="ghost"
        :disabled="submitting"
        :title="t('envForm.copyJsonTip')"
        @click="copyAsJSON"
      >
        {{ copied ? t('envForm.copied') : t('envForm.copyJson') }}
      </Button>
      <Button type="button" variant="secondary" :disabled="submitting" @click="isOpen = false">{{ t('common.cancel') }}</Button>
      <Button type="button" :disabled="submitting" @click="handleSubmit">{{ submitting ? t('envForm.saving') : (isEditing ? t('common.save') : t('envForm.create')) }}</Button>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { useI18n } from '@/composables/useI18n'
import { ref, computed, watch, onMounted } from 'vue'
import { Eye, EyeOff, Loader2, Zap } from '@lucide/vue'
import type { EnvConfig, Provider, UpstreamFormat, ProviderPreset } from '@/types'
import { configService } from '@/services/configService'
import { useConfigStore } from '@/stores/configStore'
import { useToast } from '@/composables/useToast'
import AppModal from '@/components/common/AppModal.vue'
import AppInput from '@/components/common/AppInput.vue'
import FieldLabel from '@/components/common/FieldLabel.vue'
import BrandIcon from '@/components/common/BrandIcon.vue'
import ConfigIcon from '@/components/common/ConfigIcon.vue'
import EmojiPicker from '@/components/common/EmojiPicker.vue'
import { Button } from '@/components/ui/button'
import CodeEditor from '@/components/common/CodeEditor.vue'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import SegmentedPills from '@/components/layout/SegmentedPills.vue'
import { errorMessage, formatLatency, withDefaultBaseUrl } from '@/lib/configUrl'
import { asUpstreamFormat, conversionTagLabel, upstreamFormatOptions } from '@/lib/upstreamFormat'
import { zhEnvForm } from '@/i18n/locales/zh/envForm'

const { t } = useI18n()

interface Props {
  modelValue: boolean
  editConfig?: EnvConfig | null
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  saved: []
}>()

const configStore = useConfigStore()
const toast = useToast()

const isOpen = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value)
})

const isEditing = computed(() => !!props.editConfig)
const showEmojiPicker = ref(false)
const showMore = ref(false)
type ApiKeyProvider = 'claude' | 'codex' | 'antigravity' | 'opencode' | 'grok'
const showApiKey = ref<Record<ApiKeyProvider, boolean>>({
  claude: false,
  codex: false,
  antigravity: false,
  opencode: false,
  grok: false,
})

function toggleApiKeyVisibility(provider: ApiKeyProvider) {
  showApiKey.value[provider] = !showApiKey.value[provider]
}

function resetApiKeyVisibility() {
  showApiKey.value.claude = false
  showApiKey.value.codex = false
  showApiKey.value.antigravity = false
  showApiKey.value.opencode = false
  showApiKey.value.grok = false
}

function selectIcon(emoji: string) {
  form.value.icon = emoji
}

const providers: { value: Provider; label: string }[] = [
  { value: 'claude', label: 'Claude Code' },
  { value: 'claude_desktop', label: 'Claude Desktop' },
  { value: 'codex', label: 'Codex' },
  { value: 'antigravity', label: 'Antigravity' },
  { value: 'opencode', label: 'OpenCode' },
  { value: 'grok', label: 'Grok' },
]
const providerPresets = computed(() => {
  const common = [{ label: t('envForm.official'), url: officialUrl(form.value.provider) }, { label: 'AIHubo', url: 'https://www.aihubo.com/api/v1' }]
  return common
})
function officialUrl(provider: Provider) {
  if (provider === 'claude' || provider === 'claude_desktop') return 'https://api.anthropic.com'
  if (provider === 'codex' || provider === 'opencode') return 'https://api.openai.com/v1'
  if (provider === 'antigravity') return 'https://generativelanguage.googleapis.com'
  return 'https://api.x.ai/v1'
}
function applyPreset(preset: { url: string }) {
  const target = form.value.provider
  if (target === 'claude' || target === 'claude_desktop') form.value.claude.baseUrl = preset.url
  else if (target === 'codex') form.value.codex.baseUrl = preset.url
  else if (target === 'antigravity') form.value.antigravity.baseUrl = preset.url
  else if (target === 'opencode') form.value.opencode.baseUrl = preset.url
  else form.value.grok.baseUrl = preset.url
}
// 供应商预设：从后端目录拉取，一键填充官方兼容端点与推荐模型（Key 仍由用户填写）
const vendorPresets = ref<ProviderPreset[]>([])
onMounted(() => {
  configService.getProviderPresets().then(list => {
    vendorPresets.value = list.filter(p => p.provider === 'claude')
  }).catch(() => {})
})

function applyVendorPreset(preset: ProviderPreset) {
  form.value.claude.baseUrl = preset.variables['ANTHROPIC_BASE_URL'] || ''
  if (preset.variables['ANTHROPIC_MODEL']) {
    form.value.claude.model = preset.variables['ANTHROPIC_MODEL']
  }
}

const grokBackends = [
  { value: 'responses', label: 'Responses' },
  { value: 'chat_completions', label: 'Chat' },
  { value: 'messages', label: 'Messages' },
]

// 字段提示随界面语言切换；键与语言包 envForm.tips 一一对应
const TIP_KEYS = Object.keys(zhEnvForm.tips) as (keyof typeof zhEnvForm.tips)[]
const tips = computed(() => Object.fromEntries(
  TIP_KEYS.map(key => [key, t(`envForm.tips.${key}`)]),
) as Record<keyof typeof zhEnvForm.tips, string>)

function onProvider(value: unknown) {
  if (value === 'claude' || value === 'claude_desktop' || value === 'codex' || value === 'antigravity' || value === 'opencode' || value === 'grok') {
    const prevFormat = form.value.upstreamFormat
    form.value.provider = value
    // 新平台支持同一格式时保留用户的「上游格式」选择，误点其它平台再点回来不再丢配置
    const stillSupported = upstreamFormatOptions(value).some(item => item.value === prevFormat)
    form.value.upstreamFormat = stillSupported ? prevFormat : ''
  }
}

const upstreamOptions = computed(() => upstreamFormatOptions(form.value.provider))

// Select 不接受空字符串值，用 native 哨兵值桥接
const upstreamSelect = computed({
  get: () => form.value.upstreamFormat || 'native',
  set: (value: string) => {
    form.value.upstreamFormat = asUpstreamFormat(value)
  },
})

const upstreamHint = computed(() => {
  const name = providers.find(item => item.value === form.value.provider)?.label || form.value.provider
  if (!form.value.upstreamFormat) {
    return t('envForm.upstreamDirect', { name })
  }
  const conv = conversionTagLabel(form.value.provider, form.value.upstreamFormat)
  return t('envForm.upstreamRouted', { name, conv })
})

function triValue(value: string) {
  return value === '' ? 'unset' : value
}

function fromTri(value: unknown) {
  if (value === '0' || value === '1') return value
  return ''
}

const effortUnset = computed(() => ({ value: 'unset', label: t('envForm.unset') }))
const triItems = computed(() => [
  effortUnset.value,
  { value: '0', label: '0' },
  { value: '1', label: '1' },
])

const claudeEffortItems = computed(() => [
  effortUnset.value,
  { value: 'auto', label: t('envForm.autoDefault') },
  { value: 'low', label: 'low' },
  { value: 'medium', label: 'medium' },
  { value: 'high', label: 'high' },
  { value: 'xhigh', label: 'xhigh' },
  { value: 'max', label: 'max' },
])
const openaiEffortItems = computed(() => [
  effortUnset.value,
  { value: 'none', label: 'none' },
  { value: 'minimal', label: 'minimal' },
  { value: 'low', label: 'low' },
  { value: 'medium', label: 'medium' },
  { value: 'high', label: 'high' },
  { value: 'xhigh', label: 'xhigh' },
  { value: 'max', label: 'max' },
  { value: 'ultra', label: 'ultra' },
])
const grokEffortItems = computed(() => [
  effortUnset.value,
  { value: 'none', label: 'none' },
  { value: 'minimal', label: 'minimal' },
  { value: 'low', label: 'low' },
  { value: 'medium', label: 'medium' },
  { value: 'high', label: 'high' },
  { value: 'xhigh', label: 'xhigh' },
  { value: 'max', label: 'max' },
])
const geminiLevelItems = computed(() => [
  effortUnset.value,
  { value: 'minimal', label: 'minimal' },
  { value: 'low', label: 'low' },
  { value: 'medium', label: 'medium' },
  { value: 'high', label: 'high' },
])
const reasoningSummaryItems = computed(() => [
  effortUnset.value,
  { value: 'auto', label: 'auto' },
  { value: 'concise', label: 'concise' },
  { value: 'detailed', label: 'detailed' },
  { value: 'none', label: 'none' },
])

function selectOrUnset(value: string) {
  return value === 'unset' ? '' : value
}

function unsetOr(value: string, fallback = 'unset') {
  return value || fallback
}

function providerFromFilter(): Provider {
  const filter = configStore.currentFilter
  if (filter === 'claude_desktop' || filter === 'codex' || filter === 'antigravity' || filter === 'opencode' || filter === 'grok') return filter
  return 'claude'
}

const defaultForm = () => ({
  name: '',
  description: '',
  icon: '📦',
  provider: providerFromFilter(),
  upstreamFormat: '' as UpstreamFormat,
  claude: {
    baseUrl: '',
    authToken: '',
    model: '',
    apiKey: '',
    attributionHeader: '',
    disableNonessentialTraffic: '',
    desktopTemplate: '',
    smallFastModel: '',
    defaultHaiku: '',
    defaultSonnet: '',
    defaultOpus: '',
    maxOutputTokens: '',
    maxThinkingTokens: '',
    effortLevel: 'unset',
    disableAdaptiveThinking: '',
    alwaysEnableEffort: '',
    autocompactPct: '',
    disableAutocompact: '',
    disableTelemetry: '',
    disableErrorReporting: '',
    bashDefaultTimeout: '',
    bashMaxTimeout: '',
    bashMaxOutput: '',
    httpProxy: '',
    httpsProxy: '',
    maxMcpOutputTokens: '',
    mcpTimeout: '',
  },
  codex: {
    baseUrl: '',
    apiKey: '',
    model: '',
    contextWindow: '',
    maxOutputTokens: '',
    reasoningEffort: 'high',
    planReasoningEffort: 'unset',
    reasoningSummary: 'unset',
    verbosity: 'unset',
    approvalPolicy: 'unset',
    sandboxMode: 'unset',
    configTemplate: `model_provider = "duckcoding"
model = "{{model}}"
model_reasoning_effort = "high"

[model_providers.duckcoding]
name = "duckcoding"
base_url = "{{base_url}}"
wire_api = "responses"
requires_openai_auth = true`,
    authTemplate: `{
  "OPENAI_API_KEY": "{{OPENAI_API_KEY}}"
}`
  },
  antigravity: {
    baseUrl: '',
    apiKey: '',
    model: '',
    project: '',
    location: '',
    useVertex: '',
    sandbox: '',
    maxSessionTurns: '',
    compressionThreshold: '',
    thinkingLevel: 'unset',
    thinkingBudget: '',
    envTemplate: `GOOGLE_GEMINI_BASE_URL={{GOOGLE_GEMINI_BASE_URL}}
GEMINI_API_KEY={{GEMINI_API_KEY}}
GEMINI_MODEL={{GEMINI_MODEL}}`,
    settingsTemplate: `{
  "ide": {
    "enabled": true
  },
  "security": {
    "auth": {
      "selectedType": "gemini-api-key"
    }
  }
}`
  },
  opencode: {
    baseUrl: '',
    apiKey: '',
    model: '',
    reasoningEffort: 'unset',
    thinkingBudget: '',
    reasoningSummary: 'unset',
    smallModel: '',
    username: '',
    share: '',
    autoupdate: '',
    snapshot: '',
    configDir: '',
    configPath: '',
    configTemplate: ''
  },
  grok: {
    baseUrl: 'https://api.x.ai/v1',
    apiKey: '',
    model: 'grok-4.6',
    modelName: '',
    contextWindow: '',
    maxTokens: '',
    temperature: '',
    reasoningEffort: 'unset',
    apiBackend: 'responses',
    homeDir: '',
    configTemplate: '',
  }
})

const form = ref(defaultForm())
const originalName = ref('')
// 官方登录配置本身没有 base_url / api_key。编辑时要把这个标记带回去，
// 否则保存一次就退化成普通空配置，再应用会把第三方默认模板写进本机。
const officialLogin = ref(false)

const upstreamValueKeys = [
  'ANTHROPIC_BASE_URL', 'ANTHROPIC_AUTH_TOKEN', 'ANTHROPIC_API_KEY',
  'base_url', 'OPENAI_API_KEY',
  'GOOGLE_GEMINI_BASE_URL', 'GEMINI_API_KEY',
  'OPENCODE_BASE_URL', 'OPENCODE_API_KEY',
  'XAI_BASE_URL', 'XAI_API_KEY',
]

// 填了上游地址或密钥，就说明改成第三方接入了，官方登录标记自动摘掉
function hasUpstreamValue(variables: Record<string, string>) {
  return upstreamValueKeys.some(key => String(variables[key] || '').trim() !== '')
}

watch(() => props.editConfig, (config) => {
  if (config) {
    originalName.value = config.name
    form.value.name = config.name
    form.value.description = config.description || ''
    form.value.icon = config.icon || '📦'
    form.value.provider = config.provider
    form.value.upstreamFormat = (config.upstream_format || '') as UpstreamFormat
    officialLogin.value = Boolean(config.official_login)

    if (config.provider === 'claude' || config.provider === 'claude_desktop') {
      form.value.claude.baseUrl = config.variables.ANTHROPIC_BASE_URL || ''
      form.value.claude.authToken = config.variables.ANTHROPIC_AUTH_TOKEN || ''
      form.value.claude.model = config.variables.ANTHROPIC_MODEL || ''
      form.value.claude.apiKey = config.variables.ANTHROPIC_API_KEY || ''
      form.value.claude.attributionHeader = config.attribution_header || ''
      form.value.claude.disableNonessentialTraffic = config.disable_nonessential_traffic || ''
      form.value.claude.smallFastModel = config.variables.ANTHROPIC_SMALL_FAST_MODEL || ''
      form.value.claude.defaultHaiku = config.variables.ANTHROPIC_DEFAULT_HAIKU_MODEL || ''
      form.value.claude.defaultSonnet = config.variables.ANTHROPIC_DEFAULT_SONNET_MODEL || ''
      form.value.claude.defaultOpus = config.variables.ANTHROPIC_DEFAULT_OPUS_MODEL || ''
      form.value.claude.maxOutputTokens = config.variables.CLAUDE_CODE_MAX_OUTPUT_TOKENS || ''
      form.value.claude.maxThinkingTokens = config.variables.MAX_THINKING_TOKENS || ''
      form.value.claude.effortLevel = unsetOr(config.variables.CLAUDE_CODE_EFFORT_LEVEL)
      form.value.claude.disableAdaptiveThinking = config.variables.CLAUDE_CODE_DISABLE_ADAPTIVE_THINKING || ''
      form.value.claude.alwaysEnableEffort = config.variables.CLAUDE_CODE_ALWAYS_ENABLE_EFFORT || ''
      form.value.claude.autocompactPct = config.variables.CLAUDE_AUTOCOMPACT_PCT_OVERRIDE || ''
      form.value.claude.disableAutocompact = config.variables.DISABLE_AUTOCOMPACT || ''
      form.value.claude.disableTelemetry = config.variables.DISABLE_TELEMETRY || ''
      form.value.claude.disableErrorReporting = config.variables.DISABLE_ERROR_REPORTING || ''
      form.value.claude.bashDefaultTimeout = config.variables.BASH_DEFAULT_TIMEOUT_MS || ''
      form.value.claude.bashMaxTimeout = config.variables.BASH_MAX_TIMEOUT_MS || ''
      form.value.claude.bashMaxOutput = config.variables.BASH_MAX_OUTPUT_LENGTH || ''
      form.value.claude.httpProxy = config.variables.HTTP_PROXY || ''
      form.value.claude.httpsProxy = config.variables.HTTPS_PROXY || ''
      form.value.claude.maxMcpOutputTokens = config.variables.MAX_MCP_OUTPUT_TOKENS || ''
      form.value.claude.mcpTimeout = config.variables.MCP_TIMEOUT || ''
      form.value.claude.desktopTemplate = config.templates?.['claude_desktop_config.json'] || ''
    } else if (config.provider === 'codex') {
      form.value.codex.baseUrl = config.variables.base_url || ''
      form.value.codex.apiKey = config.variables.OPENAI_API_KEY || ''
      form.value.codex.model = config.variables.model || ''
      form.value.codex.contextWindow = config.variables.model_context_window || ''
      form.value.codex.maxOutputTokens = config.variables.model_max_output_tokens || ''
      form.value.codex.reasoningEffort = unsetOr(config.variables.model_reasoning_effort, 'high')
      form.value.codex.planReasoningEffort = unsetOr(config.variables.plan_mode_reasoning_effort)
      form.value.codex.reasoningSummary = unsetOr(config.variables.model_reasoning_summary)
      form.value.codex.verbosity = unsetOr(config.variables.model_verbosity)
      form.value.codex.approvalPolicy = config.variables.approval_policy || 'unset'
      form.value.codex.sandboxMode = config.variables.sandbox_mode || 'unset'
      form.value.codex.configTemplate = config.templates?.['config.toml'] || form.value.codex.configTemplate
      form.value.codex.authTemplate = config.templates?.['auth.json'] || form.value.codex.authTemplate
    } else if (config.provider === 'antigravity') {
      form.value.antigravity.baseUrl = config.variables.GOOGLE_GEMINI_BASE_URL || ''
      form.value.antigravity.apiKey = config.variables.GEMINI_API_KEY || ''
      form.value.antigravity.model = config.variables.GEMINI_MODEL || ''
      form.value.antigravity.project = config.variables.GOOGLE_CLOUD_PROJECT || ''
      form.value.antigravity.location = config.variables.GOOGLE_CLOUD_LOCATION || ''
      form.value.antigravity.useVertex = config.variables.GOOGLE_GENAI_USE_VERTEXAI || ''
      form.value.antigravity.sandbox = config.variables.GEMINI_SANDBOX || ''
      form.value.antigravity.maxSessionTurns = config.variables.GEMINI_MAX_SESSION_TURNS || ''
      form.value.antigravity.compressionThreshold = config.variables.GEMINI_COMPRESSION_THRESHOLD || ''
      form.value.antigravity.thinkingLevel = unsetOr(config.variables.GEMINI_THINKING_LEVEL)
      form.value.antigravity.thinkingBudget = config.variables.GEMINI_THINKING_BUDGET || ''
      form.value.antigravity.envTemplate = config.templates?.['.env'] || form.value.antigravity.envTemplate
      form.value.antigravity.settingsTemplate = config.templates?.['settings.json'] || form.value.antigravity.settingsTemplate
    } else if (config.provider === 'opencode') {
      form.value.opencode.baseUrl = config.variables.OPENCODE_BASE_URL || ''
      form.value.opencode.apiKey = config.variables.OPENCODE_API_KEY || ''
      form.value.opencode.model = config.variables.OPENCODE_MODEL || ''
      form.value.opencode.reasoningEffort = unsetOr(config.variables.OPENCODE_REASONING_EFFORT)
      form.value.opencode.thinkingBudget = config.variables.OPENCODE_THINKING_BUDGET || ''
      form.value.opencode.reasoningSummary = unsetOr(config.variables.OPENCODE_REASONING_SUMMARY)
      form.value.opencode.smallModel = config.variables.OPENCODE_SMALL_MODEL || ''
      form.value.opencode.username = config.variables.OPENCODE_USERNAME || ''
      form.value.opencode.share = config.variables.OPENCODE_SHARE || ''
      form.value.opencode.autoupdate = config.variables.OPENCODE_AUTOUPDATE || ''
      form.value.opencode.snapshot = config.variables.OPENCODE_SNAPSHOT || ''
      form.value.opencode.configDir = config.variables.OPENCODE_CONFIG_DIR || ''
      form.value.opencode.configPath = config.variables.OPENCODE_CONFIG || ''
      form.value.opencode.configTemplate = config.templates?.['opencode.json'] || ''
    } else if (config.provider === 'grok') {
      form.value.grok.baseUrl = config.variables.XAI_BASE_URL || 'https://api.x.ai/v1'
      form.value.grok.apiKey = config.variables.XAI_API_KEY || ''
      form.value.grok.model = config.variables.XAI_MODEL || 'grok-4.6'
      form.value.grok.modelName = config.variables.XAI_MODEL_NAME || ''
      form.value.grok.contextWindow = config.variables.XAI_CONTEXT_WINDOW || ''
      form.value.grok.maxTokens = config.variables.XAI_MAX_TOKENS || ''
      form.value.grok.temperature = config.variables.XAI_TEMPERATURE || ''
      form.value.grok.reasoningEffort = unsetOr(config.variables.XAI_REASONING_EFFORT)
      form.value.grok.apiBackend = config.variables.XAI_API_BACKEND || 'responses'
      form.value.grok.homeDir = config.variables.GROK_HOME || ''
      form.value.grok.configTemplate = config.templates?.['config.toml'] || ''
    }
  } else {
    form.value = defaultForm()
    originalName.value = ''
    officialLogin.value = false
  }
}, { immediate: true })

function resetBlankForm() {
  form.value = defaultForm()
  originalName.value = ''
  officialLogin.value = false
  showMore.value = false
  resetApiKeyVisibility()
}

watch(isOpen, (open) => {
  if (open) {
    if (!props.editConfig) resetBlankForm()
    return
  }
  resetBlankForm()
})

const latencyTesting = ref(false)

async function testLatency(url: string) {
  if (latencyTesting.value) return
  const target = withDefaultBaseUrl(form.value.provider, url)
  if (!target) {
    toast.error(t('envForm.emptyBaseUrl'))
    return
  }
  latencyTesting.value = true
  try {
    const ms = await configStore.testLatency(target)
    const label = formatLatency(ms)
    if (ms > 1000) toast.error(t('envForm.latency', { value: label }))
    else if (ms > 300) toast.info(t('envForm.latency', { value: label }))
    else toast.success(t('envForm.latency', { value: label }))
  } catch (e: unknown) {
    toast.error(t('envForm.latencyFailed', { error: errorMessage(e) }))
  } finally {
    latencyTesting.value = false
  }
}

const submitting = ref(false)

async function handleSubmit() {
  if (submitting.value) return
  const name = form.value.name.trim()
  if (!name) {
    toast.error(t('envForm.nameRequired'))
    return
  }

  // 名称只需在同一服务商内唯一；不同服务商允许同名。
  // 校验用 trim 后的名字，与保存一致，避免 "foo " 绕过重名检查
  const exists = configStore.environments.some(
    c => c.name === name
      && c.provider === form.value.provider
      && !(isEditing.value && c.name === originalName.value && c.provider === props.editConfig?.provider)
  )
  if (exists) {
    toast.error(t('envForm.nameExists'))
    return
  }

  const { variables, templates } = collectVariablesAndTemplates()
  const configData = buildConfigData(variables, templates)

  submitting.value = true
  try {
    if (isEditing.value) {
      await configStore.updateEnv(originalName.value, props.editConfig?.provider || configData.provider, configData)
    } else {
      await configStore.addEnv(configData)
    }
    toast.success(t('envForm.saved'))
    isOpen.value = false
    emit('saved')
  } catch (e: any) {
    toast.error(t('envForm.saveFailed', { error: e?.message ?? String(e) }))
  } finally {
    submitting.value = false
  }
}

function collectVariablesAndTemplates(): { variables: Record<string, string>, templates: Record<string, string> } {
  let variables: Record<string, string> = {}
  let templates: Record<string, string> = {}

  if (form.value.provider === 'claude') {
    variables = {
      ANTHROPIC_BASE_URL: form.value.claude.baseUrl,
      ANTHROPIC_AUTH_TOKEN: form.value.claude.authToken,
      ANTHROPIC_MODEL: form.value.claude.model,
      ANTHROPIC_API_KEY: form.value.claude.apiKey,
      ANTHROPIC_SMALL_FAST_MODEL: form.value.claude.smallFastModel,
      ANTHROPIC_DEFAULT_HAIKU_MODEL: form.value.claude.defaultHaiku,
      ANTHROPIC_DEFAULT_SONNET_MODEL: form.value.claude.defaultSonnet,
      ANTHROPIC_DEFAULT_OPUS_MODEL: form.value.claude.defaultOpus,
      CLAUDE_CODE_MAX_OUTPUT_TOKENS: form.value.claude.maxOutputTokens,
      MAX_THINKING_TOKENS: form.value.claude.maxThinkingTokens,
      CLAUDE_CODE_EFFORT_LEVEL: selectOrUnset(form.value.claude.effortLevel),
      CLAUDE_CODE_DISABLE_ADAPTIVE_THINKING: form.value.claude.disableAdaptiveThinking,
      CLAUDE_CODE_ALWAYS_ENABLE_EFFORT: form.value.claude.alwaysEnableEffort,
      CLAUDE_AUTOCOMPACT_PCT_OVERRIDE: form.value.claude.autocompactPct,
      DISABLE_AUTOCOMPACT: form.value.claude.disableAutocompact,
      DISABLE_TELEMETRY: form.value.claude.disableTelemetry,
      DISABLE_ERROR_REPORTING: form.value.claude.disableErrorReporting,
      BASH_DEFAULT_TIMEOUT_MS: form.value.claude.bashDefaultTimeout,
      BASH_MAX_TIMEOUT_MS: form.value.claude.bashMaxTimeout,
      BASH_MAX_OUTPUT_LENGTH: form.value.claude.bashMaxOutput,
      HTTP_PROXY: form.value.claude.httpProxy,
      HTTPS_PROXY: form.value.claude.httpsProxy,
      MAX_MCP_OUTPUT_TOKENS: form.value.claude.maxMcpOutputTokens,
      MCP_TIMEOUT: form.value.claude.mcpTimeout,
    }
  } else if (form.value.provider === 'claude_desktop') {
    variables = {
      ANTHROPIC_BASE_URL: form.value.claude.baseUrl,
      ANTHROPIC_AUTH_TOKEN: form.value.claude.authToken,
      ANTHROPIC_MODEL: form.value.claude.model,
      ANTHROPIC_API_KEY: form.value.claude.apiKey,
    }
    if (form.value.provider === 'claude_desktop' && form.value.claude.desktopTemplate.trim()) {
      templates['claude_desktop_config.json'] = form.value.claude.desktopTemplate
    }
  } else if (form.value.provider === 'codex') {
    variables = {
      base_url: form.value.codex.baseUrl,
      OPENAI_API_KEY: form.value.codex.apiKey,
      model: form.value.codex.model,
      model_context_window: form.value.codex.contextWindow,
      model_max_output_tokens: form.value.codex.maxOutputTokens,
      model_reasoning_effort: selectOrUnset(form.value.codex.reasoningEffort),
      plan_mode_reasoning_effort: selectOrUnset(form.value.codex.planReasoningEffort),
      model_reasoning_summary: selectOrUnset(form.value.codex.reasoningSummary),
      model_verbosity: selectOrUnset(form.value.codex.verbosity),
      approval_policy: form.value.codex.approvalPolicy === 'unset' ? '' : form.value.codex.approvalPolicy,
      sandbox_mode: form.value.codex.sandboxMode === 'unset' ? '' : form.value.codex.sandboxMode,
    }
    if (form.value.codex.configTemplate) {
      templates['config.toml'] = form.value.codex.configTemplate
    }
    if (form.value.codex.authTemplate) {
      templates['auth.json'] = form.value.codex.authTemplate
    }
  } else if (form.value.provider === 'antigravity') {
    variables = {
      GOOGLE_GEMINI_BASE_URL: form.value.antigravity.baseUrl,
      GEMINI_API_KEY: form.value.antigravity.apiKey,
      GEMINI_MODEL: form.value.antigravity.model,
      GOOGLE_CLOUD_PROJECT: form.value.antigravity.project,
      GOOGLE_CLOUD_LOCATION: form.value.antigravity.location,
      GOOGLE_GENAI_USE_VERTEXAI: form.value.antigravity.useVertex,
      GEMINI_SANDBOX: form.value.antigravity.sandbox,
      GEMINI_MAX_SESSION_TURNS: form.value.antigravity.maxSessionTurns,
      GEMINI_COMPRESSION_THRESHOLD: form.value.antigravity.compressionThreshold,
      GEMINI_THINKING_LEVEL: selectOrUnset(form.value.antigravity.thinkingLevel),
      GEMINI_THINKING_BUDGET: form.value.antigravity.thinkingBudget,
    }
    if (form.value.antigravity.envTemplate) {
      templates['.env'] = form.value.antigravity.envTemplate
    }
    if (form.value.antigravity.settingsTemplate) {
      templates['settings.json'] = form.value.antigravity.settingsTemplate
    }
  } else if (form.value.provider === 'opencode') {
    variables = {
      OPENCODE_BASE_URL: form.value.opencode.baseUrl,
      OPENCODE_API_KEY: form.value.opencode.apiKey,
      OPENCODE_MODEL: form.value.opencode.model,
      OPENCODE_REASONING_EFFORT: selectOrUnset(form.value.opencode.reasoningEffort),
      OPENCODE_THINKING_BUDGET: form.value.opencode.thinkingBudget,
      OPENCODE_REASONING_SUMMARY: selectOrUnset(form.value.opencode.reasoningSummary),
      OPENCODE_SMALL_MODEL: form.value.opencode.smallModel,
      OPENCODE_USERNAME: form.value.opencode.username,
      OPENCODE_SHARE: form.value.opencode.share,
      OPENCODE_AUTOUPDATE: form.value.opencode.autoupdate,
      OPENCODE_SNAPSHOT: form.value.opencode.snapshot,
      OPENCODE_CONFIG_DIR: form.value.opencode.configDir,
      OPENCODE_CONFIG: form.value.opencode.configPath
    }
    if (isEditing.value && props.editConfig?.variables) {
      for (const key of ['OPENCODE_PROVIDER_ID', 'OPENCODE_PROVIDER_NAME', 'OPENCODE_NPM', 'OPENCODE_MODELS']) {
        if (props.editConfig.variables[key]) variables[key] = props.editConfig.variables[key]
      }
    }
    if (form.value.opencode.configTemplate) {
      templates['opencode.json'] = form.value.opencode.configTemplate
    }
  } else if (form.value.provider === 'grok') {
    variables = {
      XAI_BASE_URL: form.value.grok.baseUrl || 'https://api.x.ai/v1',
      XAI_API_KEY: form.value.grok.apiKey,
      XAI_MODEL: form.value.grok.model || 'grok-4.6',
      XAI_MODEL_NAME: form.value.grok.modelName,
      XAI_CONTEXT_WINDOW: form.value.grok.contextWindow,
      XAI_MAX_TOKENS: form.value.grok.maxTokens,
      XAI_TEMPERATURE: form.value.grok.temperature,
      XAI_REASONING_EFFORT: selectOrUnset(form.value.grok.reasoningEffort),
      XAI_API_BACKEND: form.value.grok.apiBackend || 'responses',
      GROK_HOME: form.value.grok.homeDir,
    }
    if (form.value.grok.configTemplate) {
      templates['config.toml'] = form.value.grok.configTemplate
    }
  }
  return { variables, templates }
}

function buildConfigData(variables: Record<string, string>, templates: Record<string, string>): EnvConfig {
  return {
    name: form.value.name.trim(),
    description: form.value.description.trim(),
    provider: form.value.provider,
    variables,
    templates,
    icon: form.value.icon,
    upstream_format: form.value.upstreamFormat,
    official_login: officialLogin.value && !hasUpstreamValue(variables),
    attribution_header: form.value.provider === 'claude' ? form.value.claude.attributionHeader : '',
    disable_nonessential_traffic: form.value.provider === 'claude' ? form.value.claude.disableNonessentialTraffic : ''
  }
}

// 复制当前表单为 JSON：与"拖拽 JSON 导入"构成分享闭环
const copied = ref(false)

async function copyAsJSON() {
  const { variables, templates } = collectVariablesAndTemplates()
  const payload = buildConfigData(variables, templates)
  try {
    await navigator.clipboard.writeText(JSON.stringify(payload, null, 2))
    copied.value = true
    toast.success(t('envForm.copiedWithSecrets'))
    setTimeout(() => { copied.value = false }, 2000)
  } catch (e: any) {
    toast.error(t('envForm.copyFailed', { error: e?.message ?? String(e) }))
  }
}
</script>
