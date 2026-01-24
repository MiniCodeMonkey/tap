<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'

const codeLines = [
  { text: '---', type: 'frontmatter' },
  { text: 'theme: phosphor', type: 'frontmatter' },
  { text: '---', type: 'frontmatter' },
  { text: '', type: 'empty' },
  { text: '# Database Demo', type: 'heading' },
  { text: '', type: 'empty' },
  { text: '```sql {driver: \'sqlite\'}', type: 'code-start' },
  { text: 'SELECT name, population', type: 'keyword' },
  { text: 'FROM cities', type: 'keyword' },
  { text: 'WHERE country = \'Japan\'', type: 'string' },
  { text: 'ORDER BY population DESC', type: 'keyword' },
  { text: 'LIMIT 5;', type: 'keyword' },
  { text: '```', type: 'code-end' },
  { text: '', type: 'empty' },
  { text: 'Results appear live as you present.', type: 'text' }
]

const sectionRef = ref<HTMLElement | null>(null)
const isVisible = ref(false)
const typedLines = ref(0)
const isTyping = ref(false)

const checkVisibility = () => {
  if (!sectionRef.value || isTyping.value) return
  const rect = sectionRef.value.getBoundingClientRect()
  const windowHeight = window.innerHeight
  if (rect.top < windowHeight * 0.75 && !isTyping.value) {
    isVisible.value = true
    startTyping()
  }
}

const startTyping = () => {
  if (isTyping.value) return
  isTyping.value = true
  typedLines.value = 0

  const typeNextLine = () => {
    if (typedLines.value < codeLines.length) {
      typedLines.value++
      const delay = codeLines[typedLines.value - 1]?.text === '' ? 50 : 80
      setTimeout(typeNextLine, delay)
    }
  }

  setTimeout(typeNextLine, 500)
}

const visibleLines = computed(() => codeLines.slice(0, typedLines.value))

onMounted(() => {
  checkVisibility()
  window.addEventListener('scroll', checkVisibility, { passive: true })
})

onUnmounted(() => {
  window.removeEventListener('scroll', checkVisibility)
})
</script>

<template>
  <section ref="sectionRef" class="home-code" :class="{ 'is-visible': isVisible }">
    <div class="code-header-text">
      <h2 class="code-heading">Write slides like code</h2>
      <p class="code-subheading">Simple markdown. Powerful features.</p>
    </div>

    <div class="code-window">
      <!-- Glow effect behind window -->
      <div class="code-glow"></div>

      <div class="code-window-inner">
        <div class="code-titlebar">
          <div class="code-dots">
            <span class="code-dot code-dot-red"></span>
            <span class="code-dot code-dot-yellow"></span>
            <span class="code-dot code-dot-green"></span>
          </div>
          <span class="code-filename">slides.md</span>
          <div class="code-actions">
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M18 15l-6-6-6 6"/>
            </svg>
          </div>
        </div>

        <div class="code-content">
          <div class="code-lines">
            <div
              v-for="(line, index) in visibleLines"
              :key="index"
              class="code-line"
              :class="`code-line-${line.type}`"
              :style="{ '--index': index }"
            >
              <span class="line-number">{{ index + 1 }}</span>
              <span class="line-content">{{ line.text }}</span>
            </div>
            <div v-if="typedLines < codeLines.length" class="code-cursor"></div>
          </div>
        </div>
      </div>

      <!-- Reflection effect -->
      <div class="code-reflection"></div>
    </div>
  </section>
</template>

<style scoped>
.home-code {
  padding: 80px 24px;
  max-width: 900px;
  margin: 0 auto;
}

.code-header-text {
  text-align: center;
  margin-bottom: 48px;
  opacity: 0;
  transform: translateY(20px);
  transition: all 0.8s cubic-bezier(0.16, 1, 0.3, 1);
}

.home-code.is-visible .code-header-text {
  opacity: 1;
  transform: translateY(0);
}

.code-heading {
  font-size: clamp(1.75rem, 4vw, 2.25rem);
  font-weight: 700;
  color: var(--vp-c-text-1);
  margin: 0 0 12px;
  letter-spacing: -0.02em;
}

.code-subheading {
  font-size: 1.125rem;
  color: var(--vp-c-text-2);
  margin: 0;
}

/* Code window container */
.code-window {
  position: relative;
  opacity: 0;
  transform: translateY(30px) scale(0.98);
  transition: all 0.8s cubic-bezier(0.16, 1, 0.3, 1);
  transition-delay: 0.2s;
}

.home-code.is-visible .code-window {
  opacity: 1;
  transform: translateY(0) scale(1);
}

/* Glow effect */
.code-glow {
  position: absolute;
  inset: -20px;
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.2), rgba(139, 92, 246, 0.2), rgba(236, 72, 153, 0.1));
  filter: blur(40px);
  border-radius: 30px;
  z-index: -1;
  opacity: 0;
  transition: opacity 0.5s ease;
}

.home-code.is-visible .code-glow {
  opacity: 1;
}

.code-window-inner {
  background: #0d0d12;
  border-radius: 16px;
  overflow: hidden;
  box-shadow:
    0 25px 50px -12px rgba(0, 0, 0, 0.5),
    0 0 0 1px rgba(255, 255, 255, 0.05);
}

/* Title bar */
.code-titlebar {
  display: flex;
  align-items: center;
  padding: 14px 18px;
  background: linear-gradient(180deg, #1a1a24 0%, #14141c 100%);
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.code-dots {
  display: flex;
  gap: 8px;
}

.code-dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  transition: transform 0.2s ease;
}

.code-dot:hover {
  transform: scale(1.2);
}

.code-dot-red {
  background: linear-gradient(135deg, #ff5f57, #ff3b30);
  box-shadow: 0 0 8px rgba(255, 95, 87, 0.4);
}

.code-dot-yellow {
  background: linear-gradient(135deg, #febc2e, #ff9500);
  box-shadow: 0 0 8px rgba(254, 188, 46, 0.4);
}

.code-dot-green {
  background: linear-gradient(135deg, #28c840, #34c759);
  box-shadow: 0 0 8px rgba(40, 200, 64, 0.4);
}

.code-filename {
  flex: 1;
  text-align: center;
  font-size: 0.8125rem;
  color: #6b7280;
  font-family: var(--vp-font-family-mono);
}

.code-actions {
  color: #6b7280;
  opacity: 0.5;
}

/* Code content */
.code-content {
  padding: 24px 0;
  min-height: 380px;
}

.code-lines {
  font-family: var(--vp-font-family-mono);
  font-size: 0.875rem;
  line-height: 1.7;
}

.code-line {
  display: flex;
  padding: 2px 24px;
  animation: fadeInLine 0.3s ease forwards;
  animation-delay: calc(var(--index) * 30ms);
}

@keyframes fadeInLine {
  from {
    opacity: 0;
    transform: translateX(-10px);
  }
  to {
    opacity: 1;
    transform: translateX(0);
  }
}

.line-number {
  width: 32px;
  text-align: right;
  margin-right: 24px;
  color: #4b5563;
  user-select: none;
  flex-shrink: 0;
}

.line-content {
  flex: 1;
  white-space: pre;
}

/* Syntax highlighting */
.code-line-frontmatter .line-content {
  color: #9ca3af;
}

.code-line-heading .line-content {
  color: #f472b6;
  font-weight: 600;
}

.code-line-code-start .line-content,
.code-line-code-end .line-content {
  color: #6366f1;
}

.code-line-keyword .line-content {
  color: #60a5fa;
}

.code-line-string .line-content {
  color: #34d399;
}

.code-line-text .line-content {
  color: #d1d5db;
}

.code-line-empty {
  height: 1.7em;
}

/* Blinking cursor */
.code-cursor {
  display: inline-block;
  width: 2px;
  height: 1.2em;
  background: #6366f1;
  margin-left: 80px;
  animation: blink 1s step-end infinite;
  box-shadow: 0 0 8px rgba(99, 102, 241, 0.8);
}

@keyframes blink {
  0%, 50% { opacity: 1; }
  51%, 100% { opacity: 0; }
}

/* Reflection effect */
.code-reflection {
  position: absolute;
  bottom: -60px;
  left: 10%;
  right: 10%;
  height: 60px;
  background: linear-gradient(180deg, rgba(13, 13, 18, 0.3), transparent);
  border-radius: 0 0 16px 16px;
  transform: scaleY(-1);
  filter: blur(4px);
  opacity: 0.3;
  pointer-events: none;
}

/* Responsive */
@media (max-width: 640px) {
  .home-code {
    padding: 60px 16px;
  }

  .code-header-text {
    margin-bottom: 32px;
  }

  .code-heading {
    font-size: 1.5rem;
  }

  .code-subheading {
    font-size: 1rem;
  }

  .code-content {
    min-height: 320px;
    padding: 16px 0;
  }

  .code-lines {
    font-size: 0.75rem;
  }

  .code-line {
    padding: 2px 16px;
  }

  .line-number {
    width: 24px;
    margin-right: 16px;
  }

  .code-titlebar {
    padding: 12px 14px;
  }

  .code-dot {
    width: 10px;
    height: 10px;
  }

  .code-filename {
    font-size: 0.75rem;
  }

  .code-cursor {
    margin-left: 56px;
  }

  .code-reflection {
    display: none;
  }
}
</style>
