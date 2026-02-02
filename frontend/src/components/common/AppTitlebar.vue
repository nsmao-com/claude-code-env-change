<template>
  <div class="custom-titlebar" style="--wails-draggable: drag">
    <!-- 窗口控制按钮 -->
    <div class="titlebar-controls" style="--wails-draggable: nodrag">
      <button class="titlebar-btn close-btn" @click="closeWindow">
        <span class="btn-dot"></span>
      </button>
      <button class="titlebar-btn minimize-btn" @click="minimizeWindow">
        <span class="btn-dot"></span>
      </button>
      <button class="titlebar-btn maximize-btn" @click="toggleMaximize">
        <span class="btn-dot"></span>
      </button>
    </div>

    <!-- 标题 -->
    <div class="titlebar-title">
      <i class="fas fa-cog"></i>
      <span>Claude Code 环境管理器</span>
    </div>

    <!-- 占位符平衡布局 -->
    <div class="titlebar-spacer"></div>
  </div>
</template>

<script setup lang="ts">
function closeWindow() {
  window.runtime?.Quit()
}

function minimizeWindow() {
  window.runtime?.WindowMinimise()
}

function toggleMaximize() {
  window.runtime?.WindowToggleMaximise()
}
</script>
