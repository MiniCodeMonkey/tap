<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

const features = [
  {
    icon: 'markdown',
    title: 'Markdown-first',
    description: 'Write slides in the format you already know. Version control friendly, editor agnostic.',
    gradient: 'linear-gradient(135deg, #6366f1, #8b5cf6)'
  },
  {
    icon: 'code',
    title: 'Live code execution',
    description: 'Run SQL, shell, or any language. Results render directly on your slides.',
    gradient: 'linear-gradient(135deg, #10b981, #34d399)'
  },
  {
    icon: 'package',
    title: 'Single binary',
    description: 'No runtime dependencies. Install once, works everywhere.',
    gradient: 'linear-gradient(135deg, #f59e0b, #fbbf24)'
  }
]

const sectionRef = ref<HTMLElement | null>(null)
const isVisible = ref(false)

const checkVisibility = () => {
  if (!sectionRef.value) return
  const rect = sectionRef.value.getBoundingClientRect()
  const windowHeight = window.innerHeight
  if (rect.top < windowHeight * 0.8) {
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
  <section ref="sectionRef" class="home-features" :class="{ 'is-visible': isVisible }">
    <div class="features-grid">
      <div
        v-for="(feature, index) in features"
        :key="feature.title"
        class="feature-card"
        :style="{ '--delay': `${index * 100}ms` }"
      >
        <div class="feature-icon-wrapper">
          <div class="feature-icon-bg" :style="{ background: feature.gradient }"></div>
          <div class="feature-icon">
            <!-- Markdown icon -->
            <svg v-if="feature.icon === 'markdown'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
              <path d="M4 5h16a1 1 0 0 1 1 1v12a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V6a1 1 0 0 1 1-1z"/>
              <path d="M7 15V9l2.5 3L12 9v6"/>
              <path d="M17 12l-2 2m0-4l2 2"/>
            </svg>
            <!-- Code icon -->
            <svg v-if="feature.icon === 'code'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
              <path d="M16 18l6-6-6-6"/>
              <path d="M8 6l-6 6 6 6"/>
              <path d="M14.5 4l-5 16"/>
            </svg>
            <!-- Package icon -->
            <svg v-if="feature.icon === 'package'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
              <path d="M12.89 1.45l8 4A2 2 0 0 1 22 7.24v9.53a2 2 0 0 1-1.11 1.79l-8 4a2 2 0 0 1-1.79 0l-8-4a2 2 0 0 1-1.1-1.8V7.24a2 2 0 0 1 1.11-1.79l8-4a2 2 0 0 1 1.78 0z"/>
              <path d="M2.32 6.16L12 11l9.68-4.84"/>
              <path d="M12 22V11"/>
            </svg>
          </div>
        </div>
        <h3 class="feature-title">{{ feature.title }}</h3>
        <p class="feature-description">{{ feature.description }}</p>
        <div class="feature-shine"></div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.home-features {
  padding: 80px 24px;
  max-width: 1100px;
  margin: 0 auto;
}

.features-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 24px;
}

.feature-card {
  position: relative;
  padding: 36px 32px;
  background: var(--tap-glass-bg);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border: 1px solid var(--tap-glass-border);
  border-radius: 20px;
  text-align: left;
  overflow: hidden;
  transition:
    transform 0.5s cubic-bezier(0.34, 1.56, 0.64, 1),
    box-shadow 0.4s ease,
    border-color 0.3s ease;

  /* Animation initial state */
  opacity: 0;
  transform: translateY(40px);
}

.home-features.is-visible .feature-card {
  opacity: 1;
  transform: translateY(0);
  transition-delay: var(--delay);
}

.feature-card:hover {
  transform: translateY(-8px) scale(1.02);
  border-color: var(--vp-c-brand-1);
  box-shadow:
    0 20px 40px rgba(0, 0, 0, 0.1),
    0 0 0 1px rgba(99, 102, 241, 0.1);
}

.dark .feature-card:hover {
  box-shadow:
    0 20px 40px rgba(0, 0, 0, 0.4),
    0 0 0 1px rgba(99, 102, 241, 0.2);
}

/* Shine effect on hover */
.feature-shine {
  position: absolute;
  top: 0;
  left: -100%;
  width: 100%;
  height: 100%;
  background: linear-gradient(
    90deg,
    transparent,
    rgba(255, 255, 255, 0.1),
    transparent
  );
  transition: left 0.5s ease;
  pointer-events: none;
}

.feature-card:hover .feature-shine {
  left: 100%;
}

/* Icon */
.feature-icon-wrapper {
  position: relative;
  width: 56px;
  height: 56px;
  margin-bottom: 24px;
}

.feature-icon-bg {
  position: absolute;
  inset: 0;
  border-radius: 14px;
  opacity: 0.15;
  transition: opacity 0.3s ease, transform 0.3s ease;
}

.feature-card:hover .feature-icon-bg {
  opacity: 0.25;
  transform: scale(1.1);
}

.feature-icon {
  position: relative;
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}

.feature-icon svg {
  width: 28px;
  height: 28px;
  color: var(--vp-c-brand-1);
  transition: transform 0.3s ease;
}

.feature-card:hover .feature-icon svg {
  transform: scale(1.1);
}

/* Title & Description */
.feature-title {
  font-size: 1.25rem;
  font-weight: 700;
  color: var(--vp-c-text-1);
  margin: 0 0 12px;
  letter-spacing: -0.01em;
}

.feature-description {
  font-size: 0.9375rem;
  color: var(--vp-c-text-2);
  line-height: 1.7;
  margin: 0;
}

/* Responsive */
@media (max-width: 900px) {
  .features-grid {
    grid-template-columns: 1fr;
    max-width: 500px;
    margin: 0 auto;
  }

  .home-features {
    padding: 60px 24px;
  }
}

@media (max-width: 480px) {
  .home-features {
    padding: 48px 16px;
  }

  .feature-card {
    padding: 28px 24px;
    border-radius: 16px;
  }

  .feature-icon-wrapper {
    width: 48px;
    height: 48px;
    margin-bottom: 20px;
  }

  .feature-icon svg {
    width: 24px;
    height: 24px;
  }

  .feature-title {
    font-size: 1.125rem;
  }

  .feature-description {
    font-size: 0.875rem;
  }
}
</style>
