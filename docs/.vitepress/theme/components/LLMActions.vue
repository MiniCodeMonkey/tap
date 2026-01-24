<template>
  <div class="llm-actions" ref="dropdownRef">
    <button
      class="llm-trigger"
      @click="toggleMenu"
      :aria-expanded="isOpen"
      aria-haspopup="true"
      aria-label="Copy page menu"
    >
      <!-- Copy/Document icon -->
      <svg
        class="trigger-icon"
        width="16"
        height="16"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
        <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
      </svg>
      <span class="trigger-text">Copy page</span>
      <svg
        class="caret-icon"
        :class="{ 'is-open': isOpen }"
        width="12"
        height="12"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2.5"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <polyline points="6 9 12 15 18 9" />
      </svg>
    </button>

    <Transition name="dropdown">
      <div v-if="isOpen" class="llm-menu" role="menu">
        <!-- Copy as Markdown -->
        <button
          class="llm-menu-item"
          @click="copyAsMarkdown"
          role="menuitem"
        >
          <span class="menu-icon-wrapper">
            <!-- Markdown icon -->
            <svg
              class="menu-icon"
              width="18"
              height="18"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <rect x="2" y="4" width="20" height="16" rx="2" />
              <path d="M6 8v8" />
              <path d="M6 12l3-4 3 4" />
              <path d="M18 8v8" />
              <path d="M15 12l3 4" />
            </svg>
          </span>
          <span class="menu-item-content">
            <span class="menu-item-title">{{ copied ? 'Copied!' : 'Copy as Markdown' }}</span>
            <span class="menu-item-description">Copy this page in Markdown</span>
          </span>
        </button>

        <!-- Open in ChatGPT -->
        <button
          class="llm-menu-item"
          @click="openInChatGPT"
          role="menuitem"
        >
          <span class="menu-icon-wrapper">
            <!-- ChatGPT icon -->
            <svg
              class="menu-icon"
              width="18"
              height="18"
              viewBox="0 0 24 24"
              fill="currentColor"
            >
              <path d="M22.282 9.821a5.985 5.985 0 0 0-.516-4.91 6.046 6.046 0 0 0-6.51-2.9A6.065 6.065 0 0 0 4.981 4.18a5.985 5.985 0 0 0-3.998 2.9 6.046 6.046 0 0 0 .743 7.097 5.98 5.98 0 0 0 .51 4.911 6.051 6.051 0 0 0 6.515 2.9A5.985 5.985 0 0 0 13.26 24a6.056 6.056 0 0 0 5.772-4.206 5.99 5.99 0 0 0 3.997-2.9 6.056 6.056 0 0 0-.747-7.073zM13.26 22.43a4.476 4.476 0 0 1-2.876-1.04l.141-.081 4.779-2.758a.795.795 0 0 0 .392-.681v-6.737l2.02 1.168a.071.071 0 0 1 .038.052v5.583a4.504 4.504 0 0 1-4.494 4.494zM3.6 18.304a4.47 4.47 0 0 1-.535-3.014l.142.085 4.783 2.759a.771.771 0 0 0 .78 0l5.843-3.369v2.332a.08.08 0 0 1-.033.062L9.74 19.95a4.5 4.5 0 0 1-6.14-1.646zM2.34 7.896a4.485 4.485 0 0 1 2.366-1.973V11.6a.766.766 0 0 0 .388.676l5.815 3.355-2.02 1.168a.076.076 0 0 1-.071 0l-4.83-2.786A4.504 4.504 0 0 1 2.34 7.896zm16.597 3.855l-5.833-3.387L15.119 7.2a.076.076 0 0 1 .071 0l4.83 2.791a4.494 4.494 0 0 1-.676 8.105v-5.678a.79.79 0 0 0-.407-.667zm2.01-3.023l-.141-.085-4.774-2.782a.776.776 0 0 0-.785 0L9.409 9.23V6.897a.066.066 0 0 1 .028-.061l4.83-2.787a4.5 4.5 0 0 1 6.68 4.66zm-12.64 4.135l-2.02-1.164a.08.08 0 0 1-.038-.057V6.075a4.5 4.5 0 0 1 7.375-3.453l-.142.08L8.704 5.46a.795.795 0 0 0-.393.681zm1.097-2.365l2.602-1.5 2.607 1.5v2.999l-2.597 1.5-2.607-1.5z" />
            </svg>
          </span>
          <span class="menu-item-content">
            <span class="menu-item-title">Open in ChatGPT</span>
            <span class="menu-item-description">Ask questions about this page</span>
          </span>
          <svg
            class="external-icon"
            width="12"
            height="12"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="M7 17L17 7" />
            <path d="M7 7h10v10" />
          </svg>
        </button>

        <!-- Open in Claude -->
        <button
          class="llm-menu-item"
          @click="openInClaude"
          role="menuitem"
        >
          <span class="menu-icon-wrapper">
            <!-- Claude icon -->
            <svg
              class="menu-icon"
              width="18"
              height="18"
              viewBox="0 0 24 24"
              fill="currentColor"
            >
              <path d="M4.709 15.955l4.72-2.647.08-.08 2.726-1.609-2.646-1.529-2.807 1.529-.08.08-4.72 2.647c-.479.32-.879.8-.879 1.449s.4 1.129.879 1.449l8.156 4.716c.479.32 1.038.32 1.598.32s1.119 0 1.598-.32l8.156-4.716c.479-.32.879-.8.879-1.449H21.37l-4.72 2.647-.08.08-2.726 1.609 2.646 1.529 2.807-1.529.08-.08 4.72-2.647c.479-.32.879-.8.879-1.449s-.4-1.129-.879-1.449l-8.156-4.716c-.479-.32-1.038-.32-1.598-.32s-1.119 0-1.598.32L4.59 14.506c-.479.32-.879.8-.879 1.449h-.001z" />
            </svg>
          </span>
          <span class="menu-item-content">
            <span class="menu-item-title">Open in Claude</span>
            <span class="menu-item-description">Ask questions about this page</span>
          </span>
          <svg
            class="external-icon"
            width="12"
            height="12"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="M7 17L17 7" />
            <path d="M7 7h10v10" />
          </svg>
        </button>
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useData } from 'vitepress'

const { page } = useData()

const isOpen = ref(false)
const copied = ref(false)
const dropdownRef = ref<HTMLElement | null>(null)

function toggleMenu() {
  isOpen.value = !isOpen.value
}

function closeMenu() {
  isOpen.value = false
}

function handleClickOutside(event: MouseEvent) {
  if (dropdownRef.value && !dropdownRef.value.contains(event.target as Node)) {
    closeMenu()
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})

async function copyAsMarkdown() {
  const rawMarkdown = (window as any).__DOC_RAW

  if (!rawMarkdown) {
    console.error('Raw markdown not available')
    return
  }

  try {
    await navigator.clipboard.writeText(rawMarkdown)
    copied.value = true
    setTimeout(() => {
      copied.value = false
      closeMenu()
    }, 1500)
  } catch (err) {
    console.error('Failed to copy:', err)
  }
}

function getPrompt() {
  const pageUrl = typeof window !== 'undefined'
    ? window.location.href
    : `https://tap.sh${page.value.relativePath.replace(/\.md$/, '.html')}`

  return `Please read the documentation from ${pageUrl} and help me understand it.`
}

function openInChatGPT() {
  const prompt = encodeURIComponent(getPrompt())
  window.open(`https://chatgpt.com/?hints=search&q=${prompt}`, '_blank')
  closeMenu()
}

function openInClaude() {
  const prompt = encodeURIComponent(getPrompt())
  window.open(`https://claude.ai/new?q=${prompt}`, '_blank')
  closeMenu()
}
</script>

<style scoped>
.llm-actions {
  position: relative;
  display: inline-flex;
  margin-bottom: 16px;
}

.llm-trigger {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  background: var(--vp-c-bg);
  border: 1px solid var(--vp-c-divider);
  border-radius: 8px;
  color: var(--vp-c-text-1);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
}

.llm-trigger:hover {
  border-color: var(--vp-c-divider-dark);
  background: var(--vp-c-bg-soft);
}

.trigger-icon {
  flex-shrink: 0;
  opacity: 0.7;
}

.trigger-text {
  font-weight: 500;
}

.caret-icon {
  flex-shrink: 0;
  opacity: 0.5;
  transition: transform 0.2s ease;
}

.caret-icon.is-open {
  transform: rotate(180deg);
}

.llm-menu {
  position: absolute;
  top: calc(100% + 8px);
  left: 0;
  z-index: 100;
  min-width: 280px;
  padding: 8px;
  background: var(--vp-c-bg-elv);
  border: 1px solid var(--vp-c-divider);
  border-radius: 12px;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.12);
}

.llm-menu-item {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  width: 100%;
  padding: 10px 12px;
  background: transparent;
  border: none;
  border-radius: 8px;
  color: var(--vp-c-text-1);
  text-align: left;
  cursor: pointer;
  transition: all 0.15s ease;
}

.llm-menu-item:hover {
  background: var(--vp-c-bg-soft);
}

.llm-menu-item:active {
  transform: scale(0.99);
}

.menu-icon-wrapper {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  flex-shrink: 0;
  border-radius: 6px;
  background: var(--vp-c-bg-soft);
  margin-top: 2px;
}

.menu-icon {
  flex-shrink: 0;
  opacity: 0.8;
}

.llm-menu-item:hover .menu-icon {
  opacity: 1;
}

.menu-item-content {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
}

.menu-item-title {
  font-size: 14px;
  font-weight: 500;
  color: var(--vp-c-text-1);
  line-height: 1.4;
}

.menu-item-description {
  font-size: 12px;
  font-weight: 400;
  color: var(--vp-c-text-3);
  line-height: 1.4;
}

.external-icon {
  flex-shrink: 0;
  opacity: 0.4;
  margin-top: 6px;
}

.llm-menu-item:hover .external-icon {
  opacity: 0.6;
}

/* Dropdown animation */
.dropdown-enter-active,
.dropdown-leave-active {
  transition: opacity 0.15s ease, transform 0.15s ease;
}

.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: translateY(-8px) scale(0.95);
}

.dropdown-enter-to,
.dropdown-leave-from {
  opacity: 1;
  transform: translateY(0) scale(1);
}
</style>
