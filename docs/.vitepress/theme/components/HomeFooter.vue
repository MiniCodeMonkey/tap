<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

const sectionRef = ref<HTMLElement | null>(null)
const isVisible = ref(false)

const checkVisibility = () => {
  if (!sectionRef.value) return
  const rect = sectionRef.value.getBoundingClientRect()
  const windowHeight = window.innerHeight
  if (rect.top < windowHeight * 0.9) {
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
  <footer ref="sectionRef" class="home-footer" :class="{ 'is-visible': isVisible }">
    <!-- Gradient divider -->
    <div class="footer-divider">
      <div class="footer-divider-line"></div>
    </div>

    <div class="footer-content">
      <div class="footer-links">
        <a href="https://github.com/tap-slides/tap" class="footer-link" target="_blank" rel="noopener">
          <svg class="footer-icon" viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
            <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
          </svg>
          <span>GitHub</span>
        </a>
        <span class="footer-dot"></span>
        <a href="/getting-started" class="footer-link">
          <svg class="footer-icon" viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253"/>
          </svg>
          <span>Documentation</span>
        </a>
        <span class="footer-dot"></span>
        <span class="footer-license">MIT License</span>
      </div>

      <p class="footer-tagline">
        <span class="tagline-text">Made for developers who present</span>
        <span class="tagline-heart">
          <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
            <path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/>
          </svg>
        </span>
      </p>
    </div>
  </footer>
</template>

<style scoped>
.home-footer {
  padding: 0 24px 80px;
  max-width: 1000px;
  margin: 0 auto;
  text-align: center;
}

/* Gradient divider */
.footer-divider {
  margin-bottom: 48px;
  opacity: 0;
  transition: opacity 0.8s ease;
}

.home-footer.is-visible .footer-divider {
  opacity: 1;
}

.footer-divider-line {
  height: 1px;
  background: linear-gradient(90deg,
    transparent,
    var(--vp-c-divider) 20%,
    var(--vp-c-brand-1) 50%,
    var(--vp-c-divider) 80%,
    transparent
  );
}

/* Content */
.footer-content {
  opacity: 0;
  transform: translateY(20px);
  transition: all 0.8s cubic-bezier(0.16, 1, 0.3, 1);
  transition-delay: 0.2s;
}

.home-footer.is-visible .footer-content {
  opacity: 1;
  transform: translateY(0);
}

/* Links */
.footer-links {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 16px;
  flex-wrap: wrap;
  margin-bottom: 24px;
}

.footer-link {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  border-radius: 100px;
  color: var(--vp-c-text-2);
  text-decoration: none;
  font-size: 0.9rem;
  font-weight: 500;
  transition: all 0.3s ease;
  background: transparent;
  border: 1px solid transparent;
}

.footer-link:hover {
  color: var(--vp-c-brand-1);
  background: rgba(99, 102, 241, 0.08);
  border-color: rgba(99, 102, 241, 0.2);
}

.footer-icon {
  flex-shrink: 0;
  transition: transform 0.3s ease;
}

.footer-link:hover .footer-icon {
  transform: scale(1.1);
}

.footer-dot {
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: var(--vp-c-divider);
}

.footer-license {
  color: var(--vp-c-text-3);
  font-size: 0.875rem;
  font-weight: 500;
}

/* Tagline */
.footer-tagline {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  margin: 0;
  color: var(--vp-c-text-3);
  font-size: 0.9375rem;
}

.tagline-text {
  font-style: italic;
}

.tagline-heart {
  display: inline-flex;
  color: #ec4899;
  animation: heartbeat 1.5s ease-in-out infinite;
}

@keyframes heartbeat {
  0%, 100% {
    transform: scale(1);
  }
  14% {
    transform: scale(1.15);
  }
  28% {
    transform: scale(1);
  }
  42% {
    transform: scale(1.15);
  }
  70% {
    transform: scale(1);
  }
}

/* Responsive */
@media (max-width: 600px) {
  .home-footer {
    padding: 0 16px 60px;
  }

  .footer-divider {
    margin-bottom: 36px;
  }

  .footer-links {
    gap: 12px;
  }

  .footer-link {
    padding: 6px 12px;
    font-size: 0.8125rem;
  }

  .footer-icon {
    width: 16px;
    height: 16px;
  }

  .footer-dot {
    display: none;
  }

  .footer-license {
    width: 100%;
    margin-top: 8px;
    font-size: 0.8125rem;
  }

  .footer-tagline {
    font-size: 0.875rem;
  }
}
</style>
