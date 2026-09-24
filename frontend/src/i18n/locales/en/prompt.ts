import type { DeepString } from '../../types'
import type { zhPrompt } from '../zh/prompt'

export const enPrompt: DeepString<typeof zhPrompt> = {
  panelHint: "Edit the custom prompts of five platforms; saving overwrites the local file directly. Claude Desktop uses its own config file",
  desktopNote: "Claude Desktop has no separate global prompt file. Edit its configLibrary JSON under Environments; the files shown here belong to Claude Code, Codex, Antigravity, OpenCode and Grok.",
  exists: "Exists",
  notCreated: "Not created",
  delete: "Delete",
  restartHint: "Restart the CLI for changes to take effect",
  loadFailed: "Load failed: {error}",
  notLoaded: "Prompt files haven't finished loading; try again shortly",
  verifyFailed: "The content read back after saving doesn't match what was edited",
  saveFailed: "Save failed: {error}",
  deleteTitle: "Delete prompt",
  deleteMsg: "Delete the {name} prompt file?",
  deleteFailed: "Delete failed: {error}",
  placeholder: {
    claude: "# CLAUDE.md example\n\n## Project rules\n- Write code in TypeScript\n- Follow the ESLint rules\n- Don't create test files\n\n## Code style\n- Prefer a functional style\n- Write comments in English",
    codex: "# AGENTS.md example\n\n## Agent instructions\n- Prefer functional programming patterns\n- Write comments in English\n- Follow the project's code style",
    antigravity: "# GEMINI.md example\n\n## Gemini instructions\n- Reply in English\n- Follow the Google Style Guide\n- Keep answers concise",
    grok: "# GROK.md example\n\n## Grok instructions\n- Reply in English\n- Read the existing structure before changing code\n- Don't add unrelated dependencies",
    opencode: "# AGENTS.md example\n\n## OpenCode instructions\n- Reply in English\n- Read the existing structure before changing code\n- Follow the project's existing code style",
  },
}
