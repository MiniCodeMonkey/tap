<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

const principles = [
  {
    icon: 'sparkles',
    title: 'Beautiful by default',
    description: 'Professional-looking slides without design skills. Every theme is polished and presentation-ready.',
    color: '#f472b6'
  },
  {
    icon: 'document',
    title: 'Markdown-first',
    description: '100% markdown for basic presentations. Write in the format you already know and love.',
    color: '#6366f1'
  },
  {
    icon: 'layers',
    title: 'Simple yet powerful',
    description: 'Simple for common cases, powerful for advanced needs. Progressive enhancement when you need it.',
    color: '#8b5cf6'
  },
  {
    icon: 'terminal',
    title: 'Developer-first',
    description: 'Built for technical presentations. Syntax highlighting, live code execution, and terminal integration.',
    color: '#10b981'
  },
  {
    icon: 'bolt',
    title: 'Zero config',
    description: 'Single binary, no runtime dependencies. Install once and start presenting immediately.',
    color: '#f59e0b'
  },
  {
    icon: 'git',
    title: 'Version control friendly',
    description: 'Plain text files that work with git. Review changes, collaborate, and track history.',
    color: '#06b6d4'
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
  <section ref="sectionRef" class="home-philosophy" :class="{ 'is-visible': isVisible }">
    <div class="philosophy-header">
      <span class="philosophy-label">Philosophy</span>
      <h2 class="philosophy-heading">The principles that guide Tap</h2>
      <p class="philosophy-subheading">
        Every decision we make is grounded in these core beliefs
      </p>
    </div>

    <div class="philosophy-grid">
      <div
        v-for="(principle, index) in principles"
        :key="principle.title"
        class="principle-card"
        :style="{ '--delay': `${index * 80}ms`, '--accent': principle.color }"
      >
        <div class="principle-icon">
          <!-- Sparkles -->
          <svg v-if="principle.icon === 'sparkles'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <path d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707"/>
            <circle cx="12" cy="12" r="4"/>
          </svg>
          <!-- Document -->
          <svg v-if="principle.icon === 'document'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <path d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/>
          </svg>
          <!-- Layers -->
          <svg v-if="principle.icon === 'layers'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5"/>
          </svg>
          <!-- Terminal -->
          <svg v-if="principle.icon === 'terminal'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <path d="M4 17l6-6-6-6m8 14h8"/>
          </svg>
          <!-- Bolt -->
          <svg v-if="principle.icon === 'bolt'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"/>
          </svg>
          <!-- Git -->
          <svg v-if="principle.icon === 'git'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <circle cx="18" cy="18" r="3"/>
            <circle cx="6" cy="6" r="3"/>
            <path d="M6 21V9a9 9 0 009 9"/>
          </svg>
        </div>

        <h3 class="principle-title">{{ principle.title }}</h3>
        <p class="principle-description">{{ principle.description }}</p>

        <div class="principle-glow"></div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.home-philosophy {
  padding: 100px 24px;
  max-width: 1200px;
  margin: 0 auto;
  position: relative;
}

/* Section background gradient */
.home-philosophy::before {
  content: '';
  position: absolute;
  inset: 0;
  background: radial-gradient(ellipse 70% 50% at 50% 100%, rgba(99, 102, 241, 0.08), transparent);
  pointer-events: none;
}

.dark .home-philosophy::before {
  background: radial-gradient(ellipse 70% 50% at 50% 100%, rgba(99, 102, 241, 0.12), transparent);
}

/* Header */
.philosophy-header {
  text-align: center;
  margin-bottom: 64px;
  position: relative;
  opacity: 0;
  transform: translateY(20px);
  transition: all 0.8s cubic-bezier(0.16, 1, 0.3, 1);
}

.home-philosophy.is-visible .philosophy-header {
  opacity: 1;
  transform: translateY(0);
}

.philosophy-label {
  display: inline-block;
  padding: 6px 16px;
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.15), rgba(139, 92, 246, 0.15));
  border: 1px solid rgba(99, 102, 241, 0.2);
  border-radius: 100px;
  font-size: 0.8125rem;
  font-weight: 600;
  color: var(--vp-c-brand-1);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin-bottom: 20px;
}

.philosophy-heading {
  font-size: clamp(2rem, 5vw, 2.75rem);
  font-weight: 800;
  color: var(--vp-c-text-1);
  margin: 0 0 16px;
  letter-spacing: -0.02em;
}

.philosophy-subheading {
  font-size: 1.125rem;
  color: var(--vp-c-text-2);
  margin: 0;
  max-width: 500px;
  margin-inline: auto;
}

/* Grid */
.philosophy-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 24px;
}

/* Cards */
.principle-card {
  position: relative;
  padding: 32px;
  background: var(--vp-c-bg);
  border: 1px solid var(--vp-c-divider);
  border-radius: 16px;
  text-align: left;
  overflow: hidden;
  transition:
    transform 0.4s cubic-bezier(0.34, 1.56, 0.64, 1),
    border-color 0.3s ease,
    box-shadow 0.4s ease;

  /* Animation initial state */
  opacity: 0;
  transform: translateY(30px);
}

.home-philosophy.is-visible .principle-card {
  opacity: 1;
  transform: translateY(0);
  transition-delay: var(--delay);
}

.principle-card:hover {
  transform: translateY(-4px);
  border-color: var(--accent);
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.08);
}

.dark .principle-card {
  background: var(--vp-c-bg-alt);
}

.dark .principle-card:hover {
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.3);
}

/* Glow effect */
.principle-glow {
  position: absolute;
  top: -50%;
  left: -50%;
  width: 200%;
  height: 200%;
  background: radial-gradient(circle at center, var(--accent), transparent 70%);
  opacity: 0;
  transition: opacity 0.4s ease;
  pointer-events: none;
  z-index: 0;
}

.principle-card:hover .principle-glow {
  opacity: 0.05;
}

.dark .principle-card:hover .principle-glow {
  opacity: 0.08;
}

/* Icon */
.principle-icon {
  position: relative;
  z-index: 1;
  width: 44px;
  height: 44px;
  margin-bottom: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 12px;
  background: linear-gradient(135deg, color-mix(in srgb, var(--accent) 15%, transparent), color-mix(in srgb, var(--accent) 5%, transparent));
  transition: transform 0.3s ease;
}

.principle-card:hover .principle-icon {
  transform: scale(1.1) rotate(-3deg);
}

.principle-icon svg {
  width: 22px;
  height: 22px;
  color: var(--accent);
}

/* Title & Description */
.principle-title {
  position: relative;
  z-index: 1;
  font-size: 1.125rem;
  font-weight: 700;
  color: var(--vp-c-text-1);
  margin: 0 0 10px;
  letter-spacing: -0.01em;
}

.principle-description {
  position: relative;
  z-index: 1;
  font-size: 0.9375rem;
  color: var(--vp-c-text-2);
  line-height: 1.65;
  margin: 0;
}

/* Responsive */
@media (max-width: 900px) {
  .philosophy-grid {
    grid-template-columns: repeat(2, 1fr);
  }

  .home-philosophy {
    padding: 80px 24px;
  }

  .philosophy-header {
    margin-bottom: 48px;
  }
}

@media (max-width: 600px) {
  .philosophy-grid {
    grid-template-columns: 1fr;
    max-width: 400px;
    margin-inline: auto;
  }

  .home-philosophy {
    padding: 60px 16px;
  }

  .philosophy-heading {
    font-size: 1.75rem;
  }

  .philosophy-subheading {
    font-size: 1rem;
  }

  .principle-card {
    padding: 24px;
  }

  .principle-icon {
    width: 40px;
    height: 40px;
    margin-bottom: 16px;
  }

  .principle-icon svg {
    width: 20px;
    height: 20px;
  }

  .principle-title {
    font-size: 1rem;
  }

  .principle-description {
    font-size: 0.875rem;
  }
}
</style>
