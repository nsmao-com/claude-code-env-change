<template>
  <div
    ref="host"
    :class="cn(
      'overflow-hidden rounded-xl border border-input bg-transparent dark:bg-input/30',
      focused && 'border-foreground/30 dark:border-white/10',
      props.class,
    )"
    :style="{ '--cm-max-height': maxHeight }"
  />
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { EditorState, Compartment } from '@codemirror/state'
import {
  EditorView,
  keymap,
  lineNumbers,
  highlightActiveLine,
  highlightActiveLineGutter,
  highlightSpecialChars,
  drawSelection,
  dropCursor,
  placeholder as cmPlaceholder,
} from '@codemirror/view'
import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands'
import {
  HighlightStyle,
  StreamLanguage,
  bracketMatching,
  indentOnInput,
  syntaxHighlighting,
} from '@codemirror/language'
import { json } from '@codemirror/lang-json'
import { linter } from '@codemirror/lint'
import { toml } from '@codemirror/legacy-modes/mode/toml'
import { properties } from '@codemirror/legacy-modes/mode/properties'
import { tags } from '@lezer/highlight'
import { cn } from '@/lib/utils'

export type CodeLanguage = 'json' | 'toml' | 'env' | 'text'

const props = withDefaults(defineProps<{
  modelValue?: string
  language?: CodeLanguage
  placeholder?: string
  class?: string
  maxHeight?: string
}>(), {
  modelValue: '',
  language: 'json',
  placeholder: '',
  maxHeight: '20rem',
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const host = ref<HTMLElement | null>(null)
const focused = ref(false)
let view: EditorView | null = null
const langCompartment = new Compartment()
const lintCompartment = new Compartment()
const placeholderCompartment = new Compartment()

const highlight = HighlightStyle.define([
  { tag: tags.propertyName, color: 'var(--brand)' },
  { tag: tags.definition(tags.variableName), color: 'var(--brand)' },
  { tag: tags.variableName, color: 'var(--foreground)' },
  { tag: tags.string, color: 'var(--chart-2)' },
  { tag: tags.number, color: 'var(--chart-4)' },
  { tag: tags.bool, color: 'var(--chart-3)' },
  { tag: tags.null, color: 'var(--muted-foreground)' },
  { tag: tags.atom, color: 'var(--chart-3)' },
  { tag: tags.keyword, color: 'var(--chart-5)' },
  { tag: tags.comment, color: 'var(--muted-foreground)', fontStyle: 'italic' },
  { tag: tags.operator, color: 'var(--muted-foreground)' },
  { tag: tags.punctuation, color: 'var(--muted-foreground)' },
  { tag: tags.invalid, color: 'var(--destructive)' },
])

const editorTheme = EditorView.theme({
  '&': {
    height: '100%',
    color: 'var(--foreground)',
    fontSize: '12px',
    backgroundColor: 'transparent',
  },
  '&.cm-focused': { outline: 'none' },
  '.cm-scroller': {
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
    overflow: 'auto',
    maxHeight: 'var(--cm-max-height, 20rem)',
  },
  '.cm-content': {
    caretColor: 'var(--foreground)',
    padding: '8px 0',
    minHeight: '6rem',
  },
  '.cm-gutters': {
    backgroundColor: 'transparent',
    color: 'var(--muted-foreground)',
    border: 'none',
  },
  '.cm-activeLine': { backgroundColor: 'color-mix(in oklab, var(--muted) 55%, transparent)' },
  '.cm-activeLineGutter': { backgroundColor: 'transparent' },
  '.cm-cursor, .cm-dropCursor': { borderLeftColor: 'var(--foreground)' },
  '&.cm-focused .cm-selectionBackground, .cm-selectionBackground': {
    backgroundColor: 'color-mix(in oklab, var(--brand) 28%, transparent) !important',
  },
  '.cm-placeholder': { color: 'var(--muted-foreground)' },
  '.cm-diagnostic-error': { borderBottom: '1px dashed var(--destructive)' },
})

function languageExtension(language: CodeLanguage) {
  if (language === 'json') return json()
  if (language === 'toml') return StreamLanguage.define(toml)
  if (language === 'env') return StreamLanguage.define(properties)
  return []
}

function jsonTemplateLinter() {
  return linter((cm) => {
    const text = cm.state.doc.toString()
    if (!text.trim()) return []
    const sanitized = text.replace(/\{\{[\s\S]*?\}\}/g, 'null')
    try {
      JSON.parse(sanitized)
      return []
    } catch (err) {
      const message = err instanceof Error ? err.message : 'JSON 无效'
      const match = message.match(/position\s+(\d+)/i)
      const pos = match ? Math.min(Number(match[1]), Math.max(0, text.length - 1)) : 0
      return [{ from: pos, to: Math.min(pos + 1, text.length), severity: 'error', message }]
    }
  })
}

function lintExtension(language: CodeLanguage) {
  return language === 'json' ? jsonTemplateLinter() : []
}

function createState(doc: string) {
  return EditorState.create({
    doc,
    extensions: [
      lineNumbers(),
      highlightActiveLine(),
      highlightActiveLineGutter(),
      highlightSpecialChars(),
      history(),
      drawSelection(),
      dropCursor(),
      indentOnInput(),
      bracketMatching(),
      EditorView.lineWrapping,
      keymap.of([...defaultKeymap, ...historyKeymap, indentWithTab]),
      syntaxHighlighting(highlight),
      editorTheme,
      langCompartment.of(languageExtension(props.language)),
      lintCompartment.of(lintExtension(props.language)),
      placeholderCompartment.of(props.placeholder ? cmPlaceholder(props.placeholder) : []),
      EditorView.updateListener.of((update) => {
        if (update.docChanged) emit('update:modelValue', update.state.doc.toString())
        if (update.focusChanged) focused.value = update.view.hasFocus
      }),
    ],
  })
}

onMounted(async () => {
  if (!host.value) return
  view = new EditorView({
    state: createState(props.modelValue || ''),
    parent: host.value,
  })
  await nextTick()
  view.requestMeasure()
})

onBeforeUnmount(() => {
  view?.destroy()
  view = null
})

watch(() => props.modelValue, (value) => {
  if (!view) return
  const next = value ?? ''
  if (view.state.doc.toString() === next) return
  view.dispatch({
    changes: { from: 0, to: view.state.doc.length, insert: next },
  })
})

watch(() => props.language, (language) => {
  view?.dispatch({
    effects: [
      langCompartment.reconfigure(languageExtension(language)),
      lintCompartment.reconfigure(lintExtension(language)),
    ],
  })
})

watch(() => props.placeholder, (text) => {
  view?.dispatch({
    effects: placeholderCompartment.reconfigure(text ? cmPlaceholder(text) : []),
  })
})
</script>
