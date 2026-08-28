<template>
  <AppModal v-model="isOpen" :title="isEditing ? '编辑配置' : '新建配置'" size="lg">
    <form class="space-y-4" @submit.prevent="handleSubmit">
      <div class="grid grid-cols-2 gap-4">
        <div class="col-span-2 sm:col-span-1">
          <AppInput v-model="form.name" label="配置名称" placeholder="输入配置名称" :tooltip="tips.name" />
        </div>
        <div class="col-span-2 sm:col-span-1">
          <FieldLabel label="图标" :hint="tips.icon" />
          <div class="relative mt-1.5">
            <Button type="button" variant="outline" size="icon" class="text-xl" @click="showEmojiPicker = !showEmojiPicker">
              <ConfigIcon :value="form.icon" class="size-5" />
            </Button>
            <EmojiPicker :show="showEmojiPicker" :current="form.icon" @close="showEmojiPicker = false" @select="selectIcon" />
          </div>
        </div>
        <div class="col-span-2">
          <AppInput v-model="form.description" label="描述" placeholder="可选的配置描述" :tooltip="tips.description" />
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

      <div class="grid gap-1.5">
        <FieldLabel label="上游格式" :hint="tips.upstreamAdvanced" />
        <Select v-model="upstreamSelect">
          <SelectTrigger class="w-full">
            <SelectValue placeholder="选择上游 API 格式" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem v-for="opt in upstreamOptions" :key="opt.value" :value="opt.value">
              {{ opt.label }}
            </SelectItem>
          </SelectContent>
        </Select>
        <p class="text-xs leading-relaxed text-muted-foreground">{{ upstreamHint }}</p>
      </div>

      <div v-if="form.provider === 'claude'" class="space-y-4">
        <AppInput v-model="form.claude.baseUrl" label="Base URL" placeholder="https://api.anthropic.com" :tooltip="tips.baseUrlClaude">
          <template #suffix>
            <Button type="button" variant="ghost" size="icon-xs" :disabled="latencyTesting" @click="testLatency(form.claude.baseUrl)">
              <Loader2 v-if="latencyTesting" class="animate-spin" />
              <Zap v-else />
            </Button>
          </template>
        </AppInput>
        <AppInput v-model="form.claude.authToken" label="Auth Token" placeholder="可选" :tooltip="tips.authToken" />
        <AppInput v-model="form.claude.model" label="Model" placeholder="claude-sonnet-4-20250514" :tooltip="tips.modelClaude" />
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

        <div class="space-y-3 border-t pt-3">
          <p class="text-xs font-medium tracking-wide text-muted-foreground uppercase">Claude Code 环境变量</p>
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
          <AppInput v-model="form.claude.smallFastModel" label="Small Fast Model（压缩上下文）" placeholder="ANTHROPIC_SMALL_FAST_MODEL" :tooltip="tips.smallFastModel" />
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <AppInput v-model="form.claude.defaultHaiku" label="Default Haiku" placeholder="ANTHROPIC_DEFAULT_HAIKU_MODEL" :tooltip="tips.defaultHaiku" />
            <AppInput v-model="form.claude.defaultSonnet" label="Default Sonnet" placeholder="ANTHROPIC_DEFAULT_SONNET_MODEL" :tooltip="tips.defaultSonnet" />
            <AppInput v-model="form.claude.defaultOpus" label="Default Opus" placeholder="ANTHROPIC_DEFAULT_OPUS_MODEL" :tooltip="tips.defaultOpus" />
          </div>
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <AppInput v-model="form.claude.maxOutputTokens" label="最大输出 Tokens" placeholder="CLAUDE_CODE_MAX_OUTPUT_TOKENS" :tooltip="tips.maxOutputClaude" />
            <AppInput v-model="form.claude.autocompactPct" label="自动压缩阈值 %" placeholder="CLAUDE_AUTOCOMPACT_PCT_OVERRIDE" :tooltip="tips.autocompactPct" />
          </div>
          <div class="space-y-3 rounded-xl bg-muted/40 p-3">
            <p class="text-xs font-medium tracking-wide text-muted-foreground uppercase">思维链</p>
            <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div class="grid gap-1.5">
                <FieldLabel label="推理强度" :hint="tips.claudeEffort" />
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
              <AppInput v-model="form.claude.maxThinkingTokens" label="思考 Tokens（旧模型）" placeholder="MAX_THINKING_TOKENS，0 关闭" :tooltip="tips.maxThinking" />
            </div>
            <div class="flex items-center justify-between gap-3">
              <div>
                <FieldLabel label="禁用自适应思考" :hint="tips.disableAdaptiveThinking" />
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
              <FieldLabel label="禁用自动压缩" :hint="tips.disableAutocompact" />
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
        <Button type="button" variant="ghost" size="sm" @click="showMore = !showMore">
          {{ showMore ? '收起更多配置' : '更多配置' }}
        </Button>
        <div v-if="showMore" class="space-y-3 border-t pt-3">
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <AppInput v-model="form.claude.httpProxy" label="HTTP_PROXY" placeholder="http://127.0.0.1:7890" :tooltip="tips.httpProxy" />
            <AppInput v-model="form.claude.httpsProxy" label="HTTPS_PROXY" placeholder="http://127.0.0.1:7890" :tooltip="tips.httpsProxy" />
            <AppInput v-model="form.claude.bashDefaultTimeout" label="Bash 默认超时 ms" placeholder="BASH_DEFAULT_TIMEOUT_MS" :tooltip="tips.bashDefaultTimeout" />
            <AppInput v-model="form.claude.bashMaxTimeout" label="Bash 最大超时 ms" placeholder="BASH_MAX_TIMEOUT_MS" :tooltip="tips.bashMaxTimeout" />
            <AppInput v-model="form.claude.bashMaxOutput" label="Bash 最大输出长度" placeholder="BASH_MAX_OUTPUT_LENGTH" :tooltip="tips.bashMaxOutput" />
            <AppInput v-model="form.claude.maxMcpOutputTokens" label="MCP 最大输出 Tokens" placeholder="MAX_MCP_OUTPUT_TOKENS" :tooltip="tips.maxMcpOutput" />
            <AppInput v-model="form.claude.mcpTimeout" label="MCP 超时 ms" placeholder="MCP_TIMEOUT" :tooltip="tips.mcpTimeout" />
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
              <FieldLabel label="强制发送 effort" :hint="tips.alwaysEnableEffort" />
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
          <AppInput v-model="form.codex.contextWindow" label="上下文窗口" placeholder="model_context_window" :tooltip="tips.contextWindowCodex" />
          <AppInput v-model="form.codex.maxOutputTokens" label="最大输出 Tokens" placeholder="model_max_output_tokens" :tooltip="tips.maxOutputCodex" />
        </div>
        <div class="space-y-3 rounded-xl bg-muted/40 p-3">
          <p class="text-xs font-medium tracking-wide text-muted-foreground uppercase">思维链</p>
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div class="grid gap-1.5">
              <FieldLabel label="推理强度" :hint="tips.reasoningEffort" />
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
              <FieldLabel label="Plan 模式推理" :hint="tips.planReasoningEffort" />
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
              <FieldLabel label="推理摘要" :hint="tips.reasoningSummary" />
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
              <FieldLabel label="回复详细度" :hint="tips.modelVerbosity" />
              <Select v-model="form.codex.verbosity">
                <SelectTrigger class="w-full">
                  <SelectValue placeholder="model_verbosity" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="unset">不设置</SelectItem>
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
            <FieldLabel label="审批策略" :hint="tips.approvalPolicy" />
            <Select v-model="form.codex.approvalPolicy">
              <SelectTrigger class="w-full">
                <SelectValue placeholder="approval_policy" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="unset">不设置</SelectItem>
                <SelectItem value="untrusted">untrusted</SelectItem>
                <SelectItem value="on-failure">on-failure</SelectItem>
                <SelectItem value="on-request">on-request</SelectItem>
                <SelectItem value="never">never</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="grid gap-1.5">
            <FieldLabel label="沙箱" :hint="tips.sandboxCodex" />
            <Select v-model="form.codex.sandboxMode">
              <SelectTrigger class="w-full">
                <SelectValue placeholder="sandbox_mode" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="unset">不设置</SelectItem>
                <SelectItem value="read-only">read-only</SelectItem>
                <SelectItem value="workspace-write">workspace-write</SelectItem>
                <SelectItem value="danger-full-access">danger-full-access</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
        <div class="grid gap-1.5">
          <FieldLabel label="config.toml 模板" :hint="tips.codexToml" />
          <CodeEditor v-model="form.codex.configTemplate" language="toml" placeholder="TOML 配置模板..." class="min-h-32" />
        </div>
        <div class="grid gap-1.5">
          <FieldLabel label="auth.json 模板" :hint="tips.codexAuth" />
          <CodeEditor v-model="form.codex.authTemplate" language="json" placeholder="JSON 认证模板..." class="min-h-24" />
        </div>
      </div>

      <div v-if="form.provider === 'gemini'" class="space-y-4">
        <AppInput v-model="form.gemini.baseUrl" label="Base URL" placeholder="https://generativelanguage.googleapis.com" :tooltip="tips.baseUrlGemini">
          <template #suffix>
            <Button type="button" variant="ghost" size="icon-xs" :disabled="latencyTesting" @click="testLatency(form.gemini.baseUrl)">
              <Loader2 v-if="latencyTesting" class="animate-spin" />
              <Zap v-else />
            </Button>
          </template>
        </AppInput>
        <AppInput
          v-model="form.gemini.apiKey"
          label="API Key"
          :type="showApiKey.gemini ? 'text' : 'password'"
          placeholder="API Key"
          :tooltip="tips.apiKeyGemini"
        >
          <template #suffix>
            <Button type="button" variant="ghost" size="icon-xs" @click="toggleApiKeyVisibility('gemini')">
              <EyeOff v-if="showApiKey.gemini" />
              <Eye v-else />
            </Button>
          </template>
        </AppInput>
        <AppInput v-model="form.gemini.model" label="Model" placeholder="gemini-3.1-pro-preview" :tooltip="tips.modelGemini" />
        <div class="space-y-3 rounded-xl bg-muted/40 p-3">
          <p class="text-xs font-medium tracking-wide text-muted-foreground uppercase">思维链</p>
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div class="grid gap-1.5">
              <FieldLabel label="思维等级（Gemini 3+）" :hint="tips.geminiThinkingLevel" />
              <Select v-model="form.gemini.thinkingLevel">
                <SelectTrigger class="w-full">
                  <SelectValue placeholder="thinkingLevel" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="item in geminiLevelItems" :key="item.value" :value="item.value">{{ item.label }}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <AppInput v-model="form.gemini.thinkingBudget" label="思考预算（Gemini 2.5）" placeholder="-1 动态 / 0 关闭 / token 数" :tooltip="tips.geminiThinkingBudget" />
          </div>
        </div>
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <AppInput v-model="form.gemini.project" label="Google Cloud Project" placeholder="GOOGLE_CLOUD_PROJECT" :tooltip="tips.geminiProject" />
          <AppInput v-model="form.gemini.location" label="Location" placeholder="GOOGLE_CLOUD_LOCATION" :tooltip="tips.geminiLocation" />
          <AppInput v-model="form.gemini.useVertex" label="使用 Vertex AI" placeholder="GOOGLE_GENAI_USE_VERTEXAI，true/false" :tooltip="tips.geminiVertex" />
          <AppInput v-model="form.gemini.sandbox" label="Sandbox" placeholder="GEMINI_SANDBOX" :tooltip="tips.geminiSandbox" />
          <AppInput v-model="form.gemini.maxSessionTurns" label="会话最大轮次" placeholder="maxSessionTurns" :tooltip="tips.geminiTurns" />
          <AppInput v-model="form.gemini.compressionThreshold" label="上下文压缩阈值" placeholder="0.7" :tooltip="tips.geminiCompress" />
        </div>
        <div class="grid gap-1.5">
          <FieldLabel label=".env 模板" :hint="tips.geminiEnv" />
          <CodeEditor v-model="form.gemini.envTemplate" language="env" placeholder="环境变量模板..." class="min-h-24" />
        </div>
        <div class="grid gap-1.5">
          <FieldLabel label="settings.json 模板" :hint="tips.geminiSettings" />
          <CodeEditor v-model="form.gemini.settingsTemplate" language="json" placeholder="JSON 设置模板..." class="min-h-24" />
        </div>
      </div>

      <div v-if="form.provider === 'opencode'" class="space-y-4">
        <div class="rounded-lg border bg-muted/40 p-3">
          <p class="text-xs leading-relaxed text-muted-foreground">
            OpenCode 配置默认写入
            <span class="font-mono">~/.config/opencode/opencode.json</span>，
            并支持 <span class="font-mono">OPENCODE_CONFIG_DIR / OPENCODE_CONFIG</span> 覆盖路径。
            填了 Base URL 时会以 OpenAI 兼容自定义 provider 接入网关。
            OpenCode 比较特殊：多套配置可以同时点「应用」，会合并进同一个 opencode.json，互不覆盖；再点一次「停用」只拿掉这一套。
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
          placeholder="可选"
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
          <p class="text-xs font-medium tracking-wide text-muted-foreground uppercase">思维链</p>
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div class="grid gap-1.5">
              <FieldLabel label="推理强度" :hint="tips.opencodeEffort" />
              <Select v-model="form.opencode.reasoningEffort">
                <SelectTrigger class="w-full">
                  <SelectValue placeholder="reasoningEffort" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="item in openaiEffortItems" :key="'oc-' + item.value" :value="item.value">{{ item.label }}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <AppInput v-model="form.opencode.thinkingBudget" label="思考预算（Anthropic）" placeholder="budgetTokens，如 16000" :tooltip="tips.opencodeThinkingBudget" />
            <div class="grid gap-1.5">
              <FieldLabel label="推理摘要" :hint="tips.opencodeReasoningSummary" />
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
          <AppInput v-model="form.opencode.smallModel" label="Small Model（摘要/压缩）" placeholder="openai/gpt-4.1-nano" :tooltip="tips.opencodeSmall" />
          <AppInput v-model="form.opencode.username" label="Username" placeholder="显示名" :tooltip="tips.opencodeUser" />
          <AppInput v-model="form.opencode.share" label="Share" placeholder="manual / auto / disabled" :tooltip="tips.opencodeShare" />
          <AppInput v-model="form.opencode.autoupdate" label="Autoupdate" placeholder="true / false" :tooltip="tips.opencodeAutoupdate" />
          <AppInput v-model="form.opencode.snapshot" label="Snapshot" placeholder="true / false" :tooltip="tips.opencodeSnapshot" />
        </div>
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <AppInput v-model="form.opencode.configDir" label="OPENCODE_CONFIG_DIR（可选）" placeholder="~/.config/opencode" :tooltip="tips.opencodeDir" />
          <AppInput v-model="form.opencode.configPath" label="OPENCODE_CONFIG（可选）" placeholder="~/.config/opencode/opencode.json" :tooltip="tips.opencodePath" />
        </div>
        <div class="grid gap-1.5">
          <FieldLabel label="opencode.json 模板（可选）" :hint="tips.opencodeJson" />
          <CodeEditor v-model="form.opencode.configTemplate" language="json" placeholder="JSON 模板，支持 {{OPENCODE_MODEL}} / {{OPENCODE_BASE_URL}} / {{OPENCODE_API_KEY}} 占位符..." class="min-h-32" />
        </div>
      </div>

      <div v-if="form.provider === 'grok'" class="space-y-4">
        <div class="rounded-lg border bg-muted/40 p-3">
          <p class="text-xs leading-relaxed text-muted-foreground">
            Grok 配置写入
            <span class="font-mono">~/.grok/config.toml</span>
            （保留已有 MCP / Skills 段）。CLI 读取
            <span class="font-mono">XAI_API_KEY</span>
            和模型的 <span class="font-mono">api_key</span>。
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
          <p class="text-xs font-medium tracking-wide text-muted-foreground uppercase">思维链</p>
          <div class="grid gap-1.5">
            <FieldLabel label="推理强度" :hint="tips.grokEffort" />
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
          <AppInput v-model="form.grok.modelName" label="显示名" placeholder="Grok" :tooltip="tips.grokName" />
          <AppInput v-model="form.grok.contextWindow" label="上下文窗口" placeholder="131072" :tooltip="tips.grokContext" />
          <AppInput v-model="form.grok.maxTokens" label="最大输出 Tokens" placeholder="8192" :tooltip="tips.grokMaxTokens" />
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
        <AppInput v-model="form.grok.homeDir" label="GROK_HOME（可选）" placeholder="~/.grok" :tooltip="tips.grokHome" />
        <div class="grid gap-1.5">
          <FieldLabel label="config.toml 模板（可选）" :hint="tips.grokToml" />
          <CodeEditor v-model="form.grok.configTemplate" language="toml" placeholder="留空则按上面的字段生成" class="min-h-32" />
        </div>
      </div>

    </form>

    <template #footer>
      <Button type="button" variant="secondary" @click="isOpen = false">取消</Button>
      <Button type="button" @click="handleSubmit">{{ isEditing ? '保存' : '创建' }}</Button>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Eye, EyeOff, Loader2, Zap } from '@lucide/vue'
import type { EnvConfig, Provider, UpstreamFormat } from '@/types'
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
type ApiKeyProvider = 'claude' | 'codex' | 'gemini' | 'opencode' | 'grok'
const showApiKey = ref<Record<ApiKeyProvider, boolean>>({
  claude: false,
  codex: false,
  gemini: false,
  opencode: false,
  grok: false,
})

function toggleApiKeyVisibility(provider: ApiKeyProvider) {
  showApiKey.value[provider] = !showApiKey.value[provider]
}

function resetApiKeyVisibility() {
  showApiKey.value.claude = false
  showApiKey.value.codex = false
  showApiKey.value.gemini = false
  showApiKey.value.opencode = false
  showApiKey.value.grok = false
}

function selectIcon(emoji: string) {
  form.value.icon = emoji
}

const providers: { value: Provider; label: string }[] = [
  { value: 'claude', label: 'Claude' },
  { value: 'codex', label: 'Codex' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'opencode', label: 'OpenCode' },
  { value: 'grok', label: 'Grok' },
]
const grokBackends = [
  { value: 'responses', label: 'Responses' },
  { value: 'chat_completions', label: 'Chat' },
  { value: 'messages', label: 'Messages' },
]

const tips = {
  name: '本软件里的显示名称，不能和其他配置重名。不会写入 CLI。',
  icon: '列表里显示的图标。可用滑块切换 Emoji 或 Lucide，再按分类挑选。',
  description: '可选备注，只在本软件显示，不写入 CLI。',
  baseUrlClaude: 'API 根地址。官方是 https://api.anthropic.com。中转/聚合填对方给的地址，一般不要再拼 /v1/messages。右侧闪电图标可测延迟。写入 ANTHROPIC_BASE_URL。',
  upstreamAdvanced: '中转站实际协议。和 CLI 原生一致选「原生直连」。Claude 接 Codex/GPT 选 Responses；Codex 接 Claude 选 Anthropic Messages。改完后打开左上角对应模型商的路由开关。',
  authToken: '部分中转用 Token 而不是 API Key。对应 ANTHROPIC_AUTH_TOKEN。通常与 API Key 二选一即可。',
  modelClaude: '主模型 ID，例如 claude-sonnet-4-20250514 或中转文档里的名称。写入 ANTHROPIC_MODEL。',
  apiKeyClaude: '密钥。官方以 sk-ant- 开头，中转按对方格式。写入 ANTHROPIC_API_KEY。',
  attributionHeader: '是否发送 Claude Code 归因头。1 开启，0 关闭。选「不设置」则沿用 CLI 默认。',
  disableNonessential: '1 会禁止遥测等非必要网络请求。选「不设置」则不改这项。',
  smallFastModel: '用来压缩长上下文的小模型 ID。空则用 CLI 默认。对应 ANTHROPIC_SMALL_FAST_MODEL。',
  defaultHaiku: 'Haiku 档默认模型 ID。对应 ANTHROPIC_DEFAULT_HAIKU_MODEL。',
  defaultSonnet: 'Sonnet 档默认模型 ID。对应 ANTHROPIC_DEFAULT_SONNET_MODEL。',
  defaultOpus: 'Opus 档默认模型 ID。对应 ANTHROPIC_DEFAULT_OPUS_MODEL。',
  maxOutputClaude: '单次回复最大输出 token。对应 CLAUDE_CODE_MAX_OUTPUT_TOKENS。',
  claudeEffort: '新模型（Opus 4.6 / Sonnet 4.6 / Claude 5 等）用自适应思考，靠 effort 控制深浅：low / medium / high / xhigh / max。auto 用模型默认。对应 CLAUDE_CODE_EFFORT_LEVEL。',
  maxThinking: '旧模型（Opus/Sonnet 4.5 及更早）的固定思考预算。填 0 可关掉思考。新模型默认忽略此项；要在 4.6 上继续用预算，请打开「禁用自适应思考」。对应 MAX_THINKING_TOKENS。',
  disableAdaptiveThinking: '仅对 Opus 4.6 / Sonnet 4.6 有效：设为 1 后改回 MAX_THINKING_TOKENS 固定预算。Opus 4.7、Claude 5 等始终走自适应思考，此项无效。',
  alwaysEnableEffort: '中转/自定义模型 ID 不被识别时，设为 1 仍在请求里带上 effort。官方模型一般不用改。对应 CLAUDE_CODE_ALWAYS_ENABLE_EFFORT。',
  autocompactPct: '上下文占用到这个百分比时自动压缩，填 0–100 的数字。对应 CLAUDE_AUTOCOMPACT_PCT_OVERRIDE。',
  disableAutocompact: '1 关闭自动压缩上下文。0 开启。不设置则沿用默认。',
  httpProxy: 'Claude Code 进程的 HTTP 代理，例如 http://127.0.0.1:7890。',
  httpsProxy: 'Claude Code 进程的 HTTPS 代理，例如 http://127.0.0.1:7890。',
  bashDefaultTimeout: 'Bash 工具默认超时，单位毫秒。对应 BASH_DEFAULT_TIMEOUT_MS。',
  bashMaxTimeout: 'Bash 工具允许的最大超时，单位毫秒。对应 BASH_MAX_TIMEOUT_MS。',
  bashMaxOutput: 'Bash 捕获输出的最大字符数。对应 BASH_MAX_OUTPUT_LENGTH。',
  maxMcpOutput: 'MCP 工具返回内容的最大 token。对应 MAX_MCP_OUTPUT_TOKENS。',
  mcpTimeout: 'MCP 调用超时，单位毫秒。对应 MCP_TIMEOUT。',
  disableTelemetry: '1 关闭遥测上报。不设置则沿用 CLI 默认。',
  disableErrorReporting: '1 关闭错误上报。不设置则沿用 CLI 默认。',
  baseUrlCodex: 'OpenAI 兼容 API 根地址，官方是 https://api.openai.com/v1。写入 config.toml 的 base_url。',
  apiKeyCodex: '写入 auth.json 的 OPENAI_API_KEY。官方以 sk- 开头，中转按对方格式。',
  modelCodex: '模型 ID，例如 gpt-5 或中转文档里的名称。写入 config.toml 的 model。',
  contextWindowCodex: '模型上下文窗口大小，填数字。对应 model_context_window。',
  maxOutputCodex: '最大输出 token，填数字。对应 model_max_output_tokens。',
  reasoningEffort: 'GPT-5 / Codex 推理强度。新模型支持 xhigh / max / ultra，旧模型常用 minimal / low / medium / high。越高越慢、越费 token。对应 model_reasoning_effort。',
  planReasoningEffort: 'Plan 模式单独的推理强度，可与默认不同。对应 plan_mode_reasoning_effort。',
  approvalPolicy: '命令执行要不要确认。never 最省事，on-request 每次问，untrusted 更严。',
  sandboxCodex: '文件访问范围。read-only 最安全，workspace-write 可改当前项目，danger-full-access 不限制。',
  reasoningSummary: '推理摘要：auto / concise / detailed / none。对应 model_reasoning_summary。',
  modelVerbosity: 'GPT-5 文本详细度：low / medium / high。对应 model_verbosity。',
  codexToml: '写入 ~/.codex/config.toml。可用 {{model}}、{{base_url}} 占位符，应用时替换成上面的值。',
  codexAuth: '写入 ~/.codex/auth.json。可用 {{OPENAI_API_KEY}} 占位符。',
  baseUrlGemini: 'Gemini API 根地址。官方是 https://generativelanguage.googleapis.com。写入 GOOGLE_GEMINI_BASE_URL。',
  apiKeyGemini: 'Gemini API Key，写入 GEMINI_API_KEY。',
  modelGemini: '模型 ID，例如 gemini-3.1-pro-preview 或 gemini-2.5-pro。写入 GEMINI_MODEL。',
  geminiThinkingLevel: 'Gemini 3 及以后用思维等级：minimal / low / medium / high。不要和思考预算同时填。写入 settings.json 的 thinkingLevel。',
  geminiThinkingBudget: 'Gemini 2.5 用 token 预算。-1 动态，0 关闭，或填具体数字（如 8192）。3.x 建议改用左侧等级。写入 thinkingBudget。',
  geminiProject: '走 Vertex AI 时需要的 GCP 项目 ID。对应 GOOGLE_CLOUD_PROJECT。',
  geminiLocation: 'Vertex 区域，例如 us-central1。对应 GOOGLE_CLOUD_LOCATION。',
  geminiVertex: '填 true 使用 Vertex AI，false 使用 AI Studio。对应 GOOGLE_GENAI_USE_VERTEXAI。',
  geminiSandbox: '是否启用沙箱。按 Gemini CLI 文档填。对应 GEMINI_SANDBOX。',
  geminiTurns: '单次会话最多轮数，填数字。对应 maxSessionTurns。',
  geminiCompress: '上下文占用到该比例时压缩，填 0–1，例如 0.7。',
  geminiEnv: '写入 ~/.gemini/.env。可用 {{GOOGLE_GEMINI_BASE_URL}}、{{GEMINI_API_KEY}}、{{GEMINI_MODEL}}。',
  geminiSettings: '写入 ~/.gemini/settings.json 的额外 JSON。留空则只用上面的字段。',
  baseUrlOpencode: 'OpenAI 兼容网关地址，例如 https://xxx/v1。填了会作为自定义 provider 写入 opencode.json。',
  apiKeyOpencode: '网关密钥。对应 OPENCODE_API_KEY，模板里可用 {{OPENCODE_API_KEY}}。',
  modelOpencode: '完整模型名，格式 provider/model，例如 anthropic/claude-sonnet-4。',
  opencodeEffort: '写入当前 provider 模型的 options.reasoningEffort。OpenAI 系用 none～ultra，Google 常用 low/high。',
  opencodeThinkingBudget: 'Anthropic 旧模型的 thinking.budgetTokens。新 Claude 5 请用左侧推理强度，不要只填预算。',
  opencodeReasoningSummary: 'OpenAI 系推理摘要：auto / concise / detailed / none。',
  opencodeSmall: '摘要/压缩用的小模型，同样用 provider/model。',
  opencodeUser: 'OpenCode 界面显示名。',
  opencodeShare: '会话分享：manual 手动、auto 自动、disabled 关闭。',
  opencodeAutoupdate: '是否自动更新 CLI，填 true 或 false。',
  opencodeSnapshot: '是否启用快照，填 true 或 false。',
  opencodeDir: '配置目录，默认 ~/.config/opencode。对应 OPENCODE_CONFIG_DIR。',
  opencodePath: '配置文件完整路径。对应 OPENCODE_CONFIG。一般只改其中一个。',
  opencodeJson: '可选。覆盖生成的 opencode.json。支持 {{OPENCODE_MODEL}} / {{OPENCODE_BASE_URL}} / {{OPENCODE_API_KEY}}。',
  baseUrlGrok: 'xAI API 地址，官方是 https://api.x.ai/v1。写入 config.toml。',
  apiKeyGrok: '密钥，一般以 xai- 开头。同时写入 XAI_API_KEY 和模型的 api_key。',
  modelGrok: '模型 ID，例如 grok-4.6。',
  grokName: '在 Grok CLI 里显示的名称，可不填。',
  grokContext: '上下文窗口，填数字，例如 131072。',
  grokMaxTokens: '单次最大输出 token，填数字。',
  grokTemp: '采样温度 0–2，越大越随机。例如 0.7。',
  grokEffort: 'Grok 4.5 支持 low / medium / high；4.6 另加 xhigh。默认 high，不能关掉思考。写入 [models] default_reasoning_effort。',
  grokBackend: 'Grok CLI 自己发出的协议。官方用 Responses。这和上面的「上游格式」不是一回事：上游格式管中转站，需要转换时才改。',
  grokHome: '配置目录，默认 ~/.grok。对应 GROK_HOME。',
  grokToml: '可选。覆盖生成的 ~/.grok/config.toml。留空则按上面的字段写入。',
}

function onProvider(value: unknown) {
  if (value === 'claude' || value === 'codex' || value === 'gemini' || value === 'opencode' || value === 'grok') {
    form.value.provider = value
    form.value.upstreamFormat = ''
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
    return `${name} 会直连你填的 Base URL，不经过本机路由。中转站协议和 CLI 不一致时再改这一项。`
  }
  const conv = conversionTagLabel(form.value.provider, form.value.upstreamFormat)
  return `卡片会标「需路由 · ${conv}」。打开左上角「${name}」路由开关后，请求先到本机网关再转到上游。`
})

function triValue(value: string) {
  return value === '' ? 'unset' : value
}

function fromTri(value: unknown) {
  if (value === '0' || value === '1') return value
  return ''
}

const triItems = [
  { value: 'unset', label: '不设置' },
  { value: '0', label: '0' },
  { value: '1', label: '1' },
]

const effortUnset = { value: 'unset', label: '不设置' }
const claudeEffortItems = [
  effortUnset,
  { value: 'auto', label: 'auto（模型默认）' },
  { value: 'low', label: 'low' },
  { value: 'medium', label: 'medium' },
  { value: 'high', label: 'high' },
  { value: 'xhigh', label: 'xhigh' },
  { value: 'max', label: 'max' },
]
const openaiEffortItems = [
  effortUnset,
  { value: 'none', label: 'none' },
  { value: 'minimal', label: 'minimal' },
  { value: 'low', label: 'low' },
  { value: 'medium', label: 'medium' },
  { value: 'high', label: 'high' },
  { value: 'xhigh', label: 'xhigh' },
  { value: 'max', label: 'max' },
  { value: 'ultra', label: 'ultra' },
]
const grokEffortItems = [
  effortUnset,
  { value: 'none', label: 'none' },
  { value: 'minimal', label: 'minimal' },
  { value: 'low', label: 'low' },
  { value: 'medium', label: 'medium' },
  { value: 'high', label: 'high' },
  { value: 'xhigh', label: 'xhigh' },
  { value: 'max', label: 'max' },
]
const geminiLevelItems = [
  effortUnset,
  { value: 'minimal', label: 'minimal' },
  { value: 'low', label: 'low' },
  { value: 'medium', label: 'medium' },
  { value: 'high', label: 'high' },
]
const reasoningSummaryItems = [
  effortUnset,
  { value: 'auto', label: 'auto' },
  { value: 'concise', label: 'concise' },
  { value: 'detailed', label: 'detailed' },
  { value: 'none', label: 'none' },
]

function selectOrUnset(value: string) {
  return value === 'unset' ? '' : value
}

function unsetOr(value: string, fallback = 'unset') {
  return value || fallback
}

function providerFromFilter(): Provider {
  const filter = configStore.currentFilter
  if (filter === 'codex' || filter === 'gemini' || filter === 'opencode' || filter === 'grok') return filter
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
network_access = "enabled"
disable_response_storage = true

[model_providers.duckcoding]
name = "duckcoding"
base_url = "{{base_url}}"
wire_api = "responses"
requires_openai_auth = true`,
    authTemplate: `{
  "OPENAI_API_KEY": "{{OPENAI_API_KEY}}"
}`
  },
  gemini: {
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

watch(() => props.editConfig, (config) => {
  if (config) {
    originalName.value = config.name
    form.value.name = config.name
    form.value.description = config.description || ''
    form.value.icon = config.icon || '📦'
    form.value.provider = config.provider
    form.value.upstreamFormat = (config.upstream_format || '') as UpstreamFormat

    if (config.provider === 'claude') {
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
    } else if (config.provider === 'gemini') {
      form.value.gemini.baseUrl = config.variables.GOOGLE_GEMINI_BASE_URL || ''
      form.value.gemini.apiKey = config.variables.GEMINI_API_KEY || ''
      form.value.gemini.model = config.variables.GEMINI_MODEL || ''
      form.value.gemini.project = config.variables.GOOGLE_CLOUD_PROJECT || ''
      form.value.gemini.location = config.variables.GOOGLE_CLOUD_LOCATION || ''
      form.value.gemini.useVertex = config.variables.GOOGLE_GENAI_USE_VERTEXAI || ''
      form.value.gemini.sandbox = config.variables.GEMINI_SANDBOX || ''
      form.value.gemini.maxSessionTurns = config.variables.GEMINI_MAX_SESSION_TURNS || ''
      form.value.gemini.compressionThreshold = config.variables.GEMINI_COMPRESSION_THRESHOLD || ''
      form.value.gemini.thinkingLevel = unsetOr(config.variables.GEMINI_THINKING_LEVEL)
      form.value.gemini.thinkingBudget = config.variables.GEMINI_THINKING_BUDGET || ''
      form.value.gemini.envTemplate = config.templates?.['.env'] || form.value.gemini.envTemplate
      form.value.gemini.settingsTemplate = config.templates?.['settings.json'] || form.value.gemini.settingsTemplate
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
  }
}, { immediate: true })

function resetBlankForm() {
  form.value = defaultForm()
  originalName.value = ''
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
    toast.error('Base URL 为空')
    return
  }
  latencyTesting.value = true
  try {
    const ms = await configStore.testLatency(target)
    const label = formatLatency(ms)
    if (ms > 1000) toast.error(`延迟 ${label}`)
    else if (ms > 300) toast.info(`延迟 ${label}`)
    else toast.success(`延迟 ${label}`)
  } catch (e: unknown) {
    toast.error('测速失败: ' + errorMessage(e))
  } finally {
    latencyTesting.value = false
  }
}

async function handleSubmit() {
  if (!form.value.name.trim()) {
    toast.error('请输入配置名称')
    return
  }

  const exists = configStore.environments.some(
    c => c.name === form.value.name && c.name !== originalName.value
  )
  if (exists) {
    toast.error('配置名称已存在')
    return
  }

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
  } else if (form.value.provider === 'gemini') {
    variables = {
      GOOGLE_GEMINI_BASE_URL: form.value.gemini.baseUrl,
      GEMINI_API_KEY: form.value.gemini.apiKey,
      GEMINI_MODEL: form.value.gemini.model,
      GOOGLE_CLOUD_PROJECT: form.value.gemini.project,
      GOOGLE_CLOUD_LOCATION: form.value.gemini.location,
      GOOGLE_GENAI_USE_VERTEXAI: form.value.gemini.useVertex,
      GEMINI_SANDBOX: form.value.gemini.sandbox,
      GEMINI_MAX_SESSION_TURNS: form.value.gemini.maxSessionTurns,
      GEMINI_COMPRESSION_THRESHOLD: form.value.gemini.compressionThreshold,
      GEMINI_THINKING_LEVEL: selectOrUnset(form.value.gemini.thinkingLevel),
      GEMINI_THINKING_BUDGET: form.value.gemini.thinkingBudget,
    }
    if (form.value.gemini.envTemplate) {
      templates['.env'] = form.value.gemini.envTemplate
    }
    if (form.value.gemini.settingsTemplate) {
      templates['settings.json'] = form.value.gemini.settingsTemplate
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

  const configData: EnvConfig = {
    name: form.value.name.trim(),
    description: form.value.description.trim(),
    provider: form.value.provider,
    variables,
    templates,
    icon: form.value.icon,
    upstream_format: form.value.upstreamFormat,
    attribution_header: form.value.provider === 'claude' ? form.value.claude.attributionHeader : '',
    disable_nonessential_traffic: form.value.provider === 'claude' ? form.value.claude.disableNonessentialTraffic : ''
  }

  try {
    if (isEditing.value) {
      await configStore.updateEnv(originalName.value, configData)
    } else {
      await configStore.addEnv(configData)
    }
    toast.success('配置已保存')
    isOpen.value = false
    emit('saved')
  } catch (e: any) {
    toast.error('保存失败: ' + e.message)
  }
}
</script>
