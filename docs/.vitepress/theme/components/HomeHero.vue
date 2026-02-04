<script setup lang="ts">
import { ref, onMounted } from 'vue'
import CopyButton from './CopyButton.vue'

const installCommand = 'brew install MiniCodeMonkey/tap/tap'
const isVisible = ref(false)

onMounted(() => {
  setTimeout(() => {
    isVisible.value = true
  }, 100)
})
</script>

<template>
  <section class="home-hero">
    <!-- Animated background -->
    <div class="hero-bg">
      <div class="hero-gradient"></div>
      <div class="hero-orb hero-orb-1"></div>
      <div class="hero-orb hero-orb-2"></div>
      <div class="hero-orb hero-orb-3"></div>
      <div class="hero-grid"></div>
    </div>

    <div class="hero-content" :class="{ 'is-visible': isVisible }">
      <div class="hero-badge">
        <span class="badge-dot"></span>
        <span class="badge-text">Open source & free forever</span>
      </div>

      <h1 class="hero-headline">
        <span class="hero-headline-line">Presentations</span>
        <span class="hero-headline-line gradient-text">for developers</span>
      </h1>

      <p class="hero-subheadline">
        Markdown slides with live code, beautiful themes, and zero config.
        <br class="hero-br" />
        Write once. Present anywhere.
      </p>

      <div class="hero-install">
        <div class="install-wrapper">
          <span class="install-prompt">$</span>
          <code class="install-command">{{ installCommand }}</code>
          <CopyButton :text="installCommand" />
        </div>
      </div>

      <div class="hero-actions">
        <a href="/getting-started" class="hero-btn hero-btn-primary">
          <span class="btn-text">Get Started</span>
          <svg class="btn-arrow" viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M5 12h14M12 5l7 7-7 7"/>
          </svg>
        </a>
        <a href="https://github.com/tap-slides/tap" class="hero-btn hero-btn-secondary" target="_blank" rel="noopener">
          <svg class="btn-icon" viewBox="0 0 24 24" width="18" height="18" fill="currentColor">
            <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
          </svg>
          <span class="btn-text">View on GitHub</span>
        </a>
      </div>
    </div>
  </section>
</template>

<style scoped>
.home-hero {
  position: relative;
  min-height: 90vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 120px 24px 80px;
  overflow: hidden;
}

/* ========================================
   Animated Background
   ======================================== */

.hero-bg {
  position: absolute;
  inset: 0;
  overflow: hidden;
  z-index: 0;
}

.hero-gradient {
  position: absolute;
  inset: 0;
  background: radial-gradient(ellipse 80% 50% at 50% -20%, rgba(99, 102, 241, 0.15), transparent);
}

.dark .hero-gradient {
  background: radial-gradient(ellipse 80% 50% at 50% -20%, rgba(99, 102, 241, 0.25), transparent);
}

.hero-orb {
  position: absolute;
  border-radius: 50%;
  filter: blur(60px);
  opacity: 0.5;
  animation: float 20s ease-in-out infinite;
}

.hero-orb-1 {
  width: 400px;
  height: 400px;
  background: linear-gradient(135deg, #6366f1, #8b5cf6);
  top: -100px;
  right: -100px;
  animation-delay: 0s;
}

.hero-orb-2 {
  width: 300px;
  height: 300px;
  background: linear-gradient(135deg, #ec4899, #f472b6);
  bottom: -50px;
  left: -50px;
  animation-delay: -7s;
}

.hero-orb-3 {
  width: 200px;
  height: 200px;
  background: linear-gradient(135deg, #06b6d4, #22d3ee);
  top: 40%;
  left: 20%;
  animation-delay: -14s;
}

.dark .hero-orb {
  opacity: 0.3;
}

.hero-grid {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(to right, var(--vp-c-divider) 1px, transparent 1px),
    linear-gradient(to bottom, var(--vp-c-divider) 1px, transparent 1px);
  background-size: 60px 60px;
  opacity: 0.3;
  mask-image: radial-gradient(ellipse 60% 60% at 50% 50%, black, transparent);
  -webkit-mask-image: radial-gradient(ellipse 60% 60% at 50% 50%, black, transparent);
}

.dark .hero-grid {
  opacity: 0.15;
}

/* ========================================
   Content
   ======================================== */

.hero-content {
  position: relative;
  z-index: 1;
  max-width: 800px;
  text-align: center;
  opacity: 0;
  transform: translateY(30px);
  transition: all 1s cubic-bezier(0.16, 1, 0.3, 1);
}

.hero-content.is-visible {
  opacity: 1;
  transform: translateY(0);
}

/* Badge */
.hero-badge {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 6px 14px 6px 10px;
  background: var(--tap-glass-bg);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  border: 1px solid var(--tap-glass-border);
  border-radius: 100px;
  margin-bottom: 24px;
  animation: fadeInUp 0.8s ease forwards;
  animation-delay: 0.2s;
  opacity: 0;
}

.badge-dot {
  width: 8px;
  height: 8px;
  background: linear-gradient(135deg, #10b981, #34d399);
  border-radius: 50%;
  animation: pulse-glow 2s ease-in-out infinite;
  box-shadow: 0 0 10px rgba(16, 185, 129, 0.5);
}

.badge-text {
  font-size: 0.8125rem;
  font-weight: 500;
  color: var(--vp-c-text-2);
  letter-spacing: 0.01em;
}

/* Headline */
.hero-headline {
  font-size: clamp(2.5rem, 8vw, 4.5rem);
  font-weight: 800;
  line-height: 1.05;
  margin: 0 0 24px;
  letter-spacing: -0.03em;
  animation: fadeInUp 0.8s ease forwards;
  animation-delay: 0.3s;
  opacity: 0;
}

.hero-headline-line {
  display: block;
}

.hero-headline-line:first-child {
  color: var(--vp-c-text-1);
}

.gradient-text {
  background: linear-gradient(135deg, #6366f1 0%, #8b5cf6 30%, #ec4899 70%, #f472b6 100%);
  background-size: 200% 200%;
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  animation: text-gradient 8s ease infinite;
}

/* Subheadline */
.hero-subheadline {
  font-size: clamp(1rem, 2.5vw, 1.25rem);
  color: var(--vp-c-text-2);
  margin: 0 0 40px;
  line-height: 1.7;
  animation: fadeInUp 0.8s ease forwards;
  animation-delay: 0.4s;
  opacity: 0;
}

.hero-br {
  display: none;
}

/* Install command */
.hero-install {
  margin-bottom: 32px;
  animation: fadeInUp 0.8s ease forwards;
  animation-delay: 0.5s;
  opacity: 0;
}

.install-wrapper {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 14px 20px;
  background: var(--vp-c-bg-alt);
  border: 1px solid var(--vp-c-divider);
  border-radius: 12px;
  transition: all 0.3s ease;
  position: relative;
  overflow: hidden;
}

.install-wrapper::before {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(90deg, transparent, rgba(99, 102, 241, 0.05), transparent);
  transform: translateX(-100%);
  transition: transform 0.6s ease;
}

.install-wrapper:hover::before {
  transform: translateX(100%);
}

.install-wrapper:hover {
  border-color: var(--vp-c-brand-1);
  box-shadow: 0 0 20px rgba(99, 102, 241, 0.15);
}

.install-prompt {
  color: var(--vp-c-brand-1);
  font-family: var(--vp-font-family-mono);
  font-weight: 600;
  user-select: none;
}

.install-command {
  font-family: var(--vp-font-family-mono);
  font-size: 0.9375rem;
  color: var(--vp-c-text-1);
  background: transparent;
}

/* Action buttons */
.hero-actions {
  display: flex;
  gap: 16px;
  justify-content: center;
  flex-wrap: wrap;
  animation: fadeInUp 0.8s ease forwards;
  animation-delay: 0.6s;
  opacity: 0;
}

.hero-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 14px 28px;
  border-radius: 12px;
  font-size: 1rem;
  font-weight: 600;
  text-decoration: none;
  transition: all 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
  position: relative;
  overflow: hidden;
}

.hero-btn-primary {
  background: linear-gradient(135deg, #6366f1, #8b5cf6);
  color: #ffffff;
  box-shadow: 0 4px 15px rgba(99, 102, 241, 0.4);
}

.hero-btn-primary::before {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(135deg, #8b5cf6, #a855f7);
  opacity: 0;
  transition: opacity 0.3s ease;
}

.hero-btn-primary:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 25px rgba(99, 102, 241, 0.5);
}

.hero-btn-primary:hover::before {
  opacity: 1;
}

.hero-btn-primary .btn-text,
.hero-btn-primary .btn-arrow {
  position: relative;
  z-index: 1;
}

.btn-arrow {
  transition: transform 0.3s ease;
}

.hero-btn-primary:hover .btn-arrow {
  transform: translateX(4px);
}

.hero-btn-secondary {
  background: var(--tap-glass-bg);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  color: var(--vp-c-text-1);
  border: 1px solid var(--vp-c-divider);
}

.hero-btn-secondary:hover {
  background: var(--vp-c-bg-alt);
  border-color: var(--vp-c-text-3);
  transform: translateY(-2px);
}

.btn-icon {
  flex-shrink: 0;
}

/* ========================================
   Responsive
   ======================================== */

@media (min-width: 640px) {
  .hero-br {
    display: block;
  }
}

@media (max-width: 768px) {
  .home-hero {
    min-height: auto;
    padding: 100px 24px 60px;
  }

  .hero-orb-1 {
    width: 250px;
    height: 250px;
  }

  .hero-orb-2 {
    width: 200px;
    height: 200px;
  }

  .hero-orb-3 {
    width: 150px;
    height: 150px;
  }

  .hero-badge {
    margin-bottom: 20px;
  }

  .badge-text {
    font-size: 0.75rem;
  }

  .hero-subheadline {
    margin-bottom: 32px;
  }
}

@media (max-width: 480px) {
  .home-hero {
    padding: 80px 16px 48px;
  }

  .hero-headline {
    margin-bottom: 20px;
  }

  .hero-subheadline {
    font-size: 1rem;
    margin-bottom: 28px;
  }

  .install-wrapper {
    padding: 12px 16px;
    width: 100%;
    justify-content: center;
  }

  .install-command {
    font-size: 0.875rem;
  }

  .hero-install {
    margin-bottom: 28px;
  }

  .hero-actions {
    flex-direction: column;
    width: 100%;
  }

  .hero-btn {
    width: 100%;
    padding: 16px 24px;
  }
}

/* Keyframes */
@keyframes fadeInUp {
  from {
    opacity: 0;
    transform: translateY(20px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@keyframes float {
  0%, 100% {
    transform: translate(0, 0) scale(1);
  }
  33% {
    transform: translate(30px, -30px) scale(1.05);
  }
  66% {
    transform: translate(-20px, 20px) scale(0.95);
  }
}

@keyframes text-gradient {
  0%, 100% {
    background-position: 0% 50%;
  }
  50% {
    background-position: 100% 50%;
  }
}

@keyframes pulse-glow {
  0%, 100% {
    opacity: 1;
    box-shadow: 0 0 10px rgba(16, 185, 129, 0.5);
  }
  50% {
    opacity: 0.7;
    box-shadow: 0 0 20px rgba(16, 185, 129, 0.8);
  }
}
</style>
