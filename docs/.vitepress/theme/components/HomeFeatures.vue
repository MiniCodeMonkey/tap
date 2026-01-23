<script setup lang="ts">
const features = [
  {
    title: 'Markdown-first',
    description: 'Write slides in the format you already know. Version control friendly, editor agnostic.'
  },
  {
    title: 'Live code execution',
    description: 'Run SQL, shell, or any language. Results render directly on your slides.'
  },
  {
    title: 'Single binary',
    description: 'No runtime dependencies. Install once, works everywhere.'
  }
]
</script>

<template>
  <section class="home-features">
    <div class="features-grid">
      <div v-for="feature in features" :key="feature.title" class="feature-card">
        <h3 class="feature-title">{{ feature.title }}</h3>
        <p class="feature-description">{{ feature.description }}</p>
      </div>
    </div>
  </section>
</template>

<style scoped>
.home-features {
  padding: 64px 24px;
  max-width: 1000px;
  margin: 0 auto;
}

.features-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 24px;
}

.feature-card {
  padding: 32px;
  background: var(--vp-c-bg-alt);
  border: 1px solid var(--vp-c-divider);
  border-radius: 12px;
  text-align: left;
}

.feature-title {
  font-size: 1.25rem;
  font-weight: 600;
  color: var(--vp-c-text-1);
  margin: 0 0 12px;
}

.feature-description {
  font-size: 1rem;
  color: var(--vp-c-text-2);
  line-height: 1.6;
  margin: 0;
}

@media (max-width: 768px) {
  .features-grid {
    grid-template-columns: 1fr;
  }

  .home-features {
    padding: 48px 20px;
  }

  .feature-card {
    padding: 24px;
  }

  .feature-title {
    font-size: 1.125rem;
  }

  .feature-description {
    font-size: 0.9375rem;
  }
}

@media (max-width: 480px) {
  .home-features {
    padding: 40px 16px;
  }

  .features-grid {
    gap: 16px;
  }

  .feature-card {
    padding: 20px;
  }
}
</style>
