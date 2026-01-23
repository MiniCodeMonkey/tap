<script setup lang="ts">
const themes = [
  {
    name: 'Minimal',
    vibe: 'Clean, spacious',
    gradient: 'linear-gradient(135deg, #f8fafc 0%, #e2e8f0 100%)'
  },
  {
    name: 'Gradient',
    vibe: 'Modern, colorful',
    gradient: 'linear-gradient(135deg, #6366f1 0%, #ec4899 50%, #f59e0b 100%)'
  },
  {
    name: 'Terminal',
    vibe: 'Hacker aesthetic',
    gradient: 'linear-gradient(135deg, #0f172a 0%, #1e293b 50%, #22c55e 100%)'
  },
  {
    name: 'Brutalist',
    vibe: 'Bold, geometric',
    gradient: 'linear-gradient(135deg, #000000 0%, #ffffff 50%, #ff0000 100%)'
  },
  {
    name: 'Keynote',
    vibe: 'Professional, polished',
    gradient: 'linear-gradient(135deg, #1e3a5f 0%, #2563eb 50%, #60a5fa 100%)'
  }
]
</script>

<template>
  <section class="home-themes">
    <h2 class="themes-heading">Built-in Themes</h2>
    <p class="themes-subheading">Choose a style that fits your presentation</p>
    <div class="themes-grid">
      <div v-for="theme in themes" :key="theme.name" class="theme-card">
        <div class="theme-preview" :style="{ background: theme.gradient }"></div>
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
  padding: 64px 24px;
  max-width: 1000px;
  margin: 0 auto;
}

.themes-heading {
  font-size: 2rem;
  font-weight: 700;
  color: var(--vp-c-text-1);
  margin: 0 0 8px;
  text-align: center;
}

.themes-subheading {
  font-size: 1.125rem;
  color: var(--vp-c-text-2);
  margin: 0 0 40px;
  text-align: center;
}

.themes-grid {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 20px;
}

.theme-card {
  background: var(--vp-c-bg-alt);
  border: 1px solid var(--vp-c-divider);
  border-radius: 12px;
  overflow: hidden;
  transition: transform 0.2s ease, box-shadow 0.2s ease;
}

.theme-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 12px 24px rgba(0, 0, 0, 0.1);
}

.theme-preview {
  height: 100px;
  width: 100%;
}

.theme-info {
  padding: 16px;
}

.theme-name {
  font-size: 1rem;
  font-weight: 600;
  color: var(--vp-c-text-1);
  margin: 0 0 4px;
}

.theme-vibe {
  font-size: 0.875rem;
  color: var(--vp-c-text-2);
  margin: 0;
}

@media (max-width: 900px) {
  .themes-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 480px) {
  .themes-grid {
    grid-template-columns: 1fr;
  }

  .home-themes {
    padding: 48px 20px;
  }
}
</style>
