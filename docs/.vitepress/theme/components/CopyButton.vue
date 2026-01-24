<template>
  <button
    class="copy-button"
    :class="{ 'is-copied': copied }"
    @click="copyToClipboard"
    :aria-label="copied ? 'Copied!' : 'Copy to clipboard'"
    :title="copied ? 'Copied!' : 'Copy to clipboard'"
  >
    <span class="copy-icon-wrapper">
      <svg
        v-if="!copied"
        class="copy-icon"
        width="16"
        height="16"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
        <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
      </svg>
      <svg
        v-else
        class="check-icon"
        width="16"
        height="16"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2.5"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <polyline points="20 6 9 17 4 12"></polyline>
      </svg>
    </span>
  </button>
</template>

<script setup lang="ts">
import { ref } from 'vue'

const props = defineProps<{
  text: string
}>()

const copied = ref(false)

async function copyToClipboard() {
  try {
    await navigator.clipboard.writeText(props.text)
    copied.value = true
    setTimeout(() => {
      copied.value = false
    }, 2000)
  } catch (err) {
    console.error('Failed to copy text:', err)
  }
}
</script>

<style scoped>
.copy-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  padding: 0;
  background: transparent;
  border: 1px solid transparent;
  border-radius: 8px;
  color: var(--vp-c-text-3);
  cursor: pointer;
  transition: all 0.25s ease;
  position: relative;
  overflow: hidden;
}

.copy-button::before {
  content: '';
  position: absolute;
  inset: 0;
  background: var(--vp-c-brand-1);
  opacity: 0;
  transition: opacity 0.25s ease;
  border-radius: 7px;
}

.copy-button:hover {
  color: var(--vp-c-brand-1);
  border-color: rgba(99, 102, 241, 0.3);
  background: rgba(99, 102, 241, 0.08);
}

.copy-button:active {
  transform: scale(0.95);
}

.copy-button.is-copied {
  color: #ffffff;
  border-color: transparent;
}

.copy-button.is-copied::before {
  opacity: 1;
}

.copy-icon-wrapper {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}

.copy-icon,
.check-icon {
  display: block;
  transition: transform 0.25s ease;
}

.copy-button:hover .copy-icon {
  transform: scale(1.1);
}

.check-icon {
  animation: checkPop 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
}

@keyframes checkPop {
  0% {
    transform: scale(0);
    opacity: 0;
  }
  50% {
    transform: scale(1.2);
  }
  100% {
    transform: scale(1);
    opacity: 1;
  }
}
</style>
