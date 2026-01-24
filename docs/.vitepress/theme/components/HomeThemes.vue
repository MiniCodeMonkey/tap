<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

const themes = [
  {
    name: 'Minimal',
    vibe: 'Clean & spacious',
    gradient: 'linear-gradient(135deg, #f8fafc 0%, #e2e8f0 50%, #f1f5f9 100%)',
    accent: '#64748b'
  },
  {
    name: 'Gradient',
    vibe: 'Modern & vibrant',
    gradient: 'linear-gradient(135deg, #6366f1 0%, #ec4899 50%, #f59e0b 100%)',
    accent: '#6366f1'
  },
  {
    name: 'Terminal',
    vibe: 'Hacker aesthetic',
    gradient: 'linear-gradient(135deg, #0f172a 0%, #1e293b 50%, #22c55e 100%)',
    accent: '#22c55e'
  },
  {
    name: 'Brutalist',
    vibe: 'Bold & geometric',
    gradient: 'linear-gradient(135deg, #18181b 0%, #fafafa 50%, #ef4444 100%)',
    accent: '#ef4444'
  },
  {
    name: 'Keynote',
    vibe: 'Professional polish',
    gradient: 'linear-gradient(135deg, #1e3a5f 0%, #2563eb 50%, #60a5fa 100%)',
    accent: '#2563eb'
  }
]

const sectionRef = ref<HTMLElement | null>(null)
const isVisible = ref(false)

const checkVisibility = () => {
  if (!sectionRef.value) return
  const rect = sectionRef.value.getBoundingClientRect()
  const windowHeight = window.innerHeight
  if (rect.top < windowHeight * 0.75) {
    isVisible.value = true
  }
}

onMounted(() => {
  checkVisibility()
  window.addEventListener('scroll', checkVisibility, { passive: true })
})

onUnmounted(() => {
  window.removeEventListener('scroll', checkVisibility)
})
</script>

<template>
  <section ref="sectionRef" class="home-themes" :class="{ 'is-visible': isVisible }">
    <div class="themes-header">
      <span class="themes-label">Themes</span>
      <h2 class="themes-heading">Built-in themes</h2>
      <p class="themes-subheading">Choose a style that fits your presentation</p>
    </div>

    <div class="themes-grid">
      <div
        v-for="(theme, index) in themes"
        :key="theme.name"
        class="theme-card"
        :style="{ '--delay': `${index * 100}ms`, '--accent': theme.accent }"
      >
        <div class="theme-preview-wrapper">
          <div class="theme-preview" :style="{ background: theme.gradient }">
            <div class="theme-preview-content">
              <div class="preview-line preview-line-title"></div>
              <div class="preview-line preview-line-text"></div>
              <div class="preview-line preview-line-text short"></div>
            </div>
          </div>
          <div class="theme-preview-glow" :style="{ background: theme.gradient }"></div>
        </div>
        <div class="theme-info">
          <h3 class="theme-name">{{ theme.name }}</h3>
          <p class="theme-vibe">{{ theme.vibe }}</p>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.home-themes {
  padding: 80px 24px 100px;
  max-width: 1100px;
  margin: 0 auto;
}

/* Header */
.themes-header {
  text-align: center;
  margin-bottom: 56px;
  opacity: 0;
  transform: translateY(20px);
  transition: all 0.8s cubic-bezier(0.16, 1, 0.3, 1);
}

.home-themes.is-visible .themes-header {
  opacity: 1;
  transform: translateY(0);
}

.themes-label {
  display: inline-block;
  padding: 6px 16px;
  background: linear-gradient(135deg, rgba(236, 72, 153, 0.15), rgba(244, 114, 182, 0.15));
  border: 1px solid rgba(236, 72, 153, 0.2);
  border-radius: 100px;
  font-size: 0.8125rem;
  font-weight: 600;
  color: #ec4899;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin-bottom: 20px;
}

.themes-heading {
  font-size: clamp(2rem, 5vw, 2.5rem);
  font-weight: 800;
  color: var(--vp-c-text-1);
  margin: 0 0 12px;
  letter-spacing: -0.02em;
}

.themes-subheading {
  font-size: 1.125rem;
  color: var(--vp-c-text-2);
  margin: 0;
}

/* Grid */
.themes-grid {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 20px;
}

/* Cards */
.theme-card {
  position: relative;
  background: var(--vp-c-bg);
  border: 1px solid var(--vp-c-divider);
  border-radius: 16px;
  overflow: hidden;
  transition:
    transform 0.5s cubic-bezier(0.34, 1.56, 0.64, 1),
    box-shadow 0.4s ease,
    border-color 0.3s ease;
  cursor: pointer;

  /* Animation initial state */
  opacity: 0;
  transform: translateY(30px) scale(0.95);
}

.home-themes.is-visible .theme-card {
  opacity: 1;
  transform: translateY(0) scale(1);
  transition-delay: var(--delay);
}

.theme-card:hover {
  transform: translateY(-8px) scale(1.02);
  border-color: var(--accent);
  box-shadow:
    0 20px 40px rgba(0, 0, 0, 0.15),
    0 0 0 1px rgba(99, 102, 241, 0.1);
}

.dark .theme-card {
  background: var(--vp-c-bg-alt);
}

.dark .theme-card:hover {
  box-shadow:
    0 20px 40px rgba(0, 0, 0, 0.4),
    0 0 0 1px var(--accent);
}

/* Preview */
.theme-preview-wrapper {
  position: relative;
  padding: 12px 12px 0;
}

.theme-preview {
  position: relative;
  height: 120px;
  border-radius: 10px;
  overflow: hidden;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  transition: transform 0.4s ease;
}

.theme-card:hover .theme-preview {
  transform: scale(1.02);
}

/* Mini slide preview */
.theme-preview-content {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  gap: 8px;
  padding: 16px;
}

.preview-line {
  height: 4px;
  border-radius: 2px;
  background: rgba(255, 255, 255, 0.5);
}

.preview-line-title {
  width: 60%;
  height: 6px;
  background: rgba(255, 255, 255, 0.8);
}

.preview-line-text {
  width: 80%;
}

.preview-line-text.short {
  width: 50%;
}

/* Glow effect */
.theme-preview-glow {
  position: absolute;
  bottom: -20px;
  left: 20%;
  right: 20%;
  height: 40px;
  filter: blur(20px);
  opacity: 0;
  transition: opacity 0.4s ease;
  pointer-events: none;
}

.theme-card:hover .theme-preview-glow {
  opacity: 0.5;
}

/* Info */
.theme-info {
  padding: 16px;
  text-align: center;
}

.theme-name {
  font-size: 1rem;
  font-weight: 700;
  color: var(--vp-c-text-1);
  margin: 0 0 4px;
  transition: color 0.3s ease;
}

.theme-card:hover .theme-name {
  color: var(--accent);
}

.theme-vibe {
  font-size: 0.8125rem;
  color: var(--vp-c-text-3);
  margin: 0;
}

/* Responsive */
@media (max-width: 1000px) {
  .themes-grid {
    grid-template-columns: repeat(3, 1fr);
  }
}

@media (max-width: 700px) {
  .themes-grid {
    grid-template-columns: repeat(2, 1fr);
  }

  .home-themes {
    padding: 60px 24px 80px;
  }

  .themes-header {
    margin-bottom: 40px;
  }

  .themes-heading {
    font-size: 1.75rem;
  }

  .themes-subheading {
    font-size: 1rem;
  }
}

@media (max-width: 480px) {
  .themes-grid {
    grid-template-columns: 1fr;
    max-width: 300px;
    margin-inline: auto;
    gap: 16px;
  }

  .home-themes {
    padding: 48px 16px 64px;
  }

  .theme-preview {
    height: 100px;
  }

  .theme-info {
    padding: 14px;
  }

  .theme-name {
    font-size: 0.9375rem;
  }

  .theme-vibe {
    font-size: 0.75rem;
  }
}
</style>
