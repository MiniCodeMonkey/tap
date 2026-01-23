<script setup lang="ts">
const principles = [
  {
    title: 'Beautiful by default',
    description: 'Professional-looking slides without design skills. Every theme is polished and presentation-ready.'
  },
  {
    title: 'Markdown-first',
    description: '100% markdown for basic presentations. Write in the format you already know and love.'
  },
  {
    title: 'Simple yet powerful',
    description: 'Simple for common cases, powerful for advanced needs. Progressive enhancement when you need it.'
  },
  {
    title: 'Developer-first',
    description: 'Built for technical presentations. Syntax highlighting, live code execution, and terminal integration.'
  },
  {
    title: 'Zero config',
    description: 'Single binary, no runtime dependencies. Install once and start presenting immediately.'
  },
  {
    title: 'Version control friendly',
    description: 'Plain text files that work with git. Review changes, collaborate, and track history.'
  }
]
</script>

<template>
  <section class="home-philosophy">
    <h2 class="philosophy-heading">Philosophy</h2>
    <p class="philosophy-subheading">The principles that guide Tap</p>
    <div class="philosophy-grid">
      <div v-for="principle in principles" :key="principle.title" class="principle-item">
        <h3 class="principle-title">{{ principle.title }}</h3>
        <p class="principle-description">{{ principle.description }}</p>
      </div>
    </div>
  </section>
</template>

<style scoped>
.home-philosophy {
  padding: 64px 24px;
  max-width: 1000px;
  margin: 0 auto;
}

.philosophy-heading {
  font-size: 2rem;
  font-weight: 700;
  color: var(--vp-c-text-1);
  margin: 0 0 8px;
  text-align: center;
}

.philosophy-subheading {
  font-size: 1.125rem;
  color: var(--vp-c-text-2);
  margin: 0 0 40px;
  text-align: center;
}

.philosophy-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 32px 24px;
}

.principle-item {
  text-align: left;
}

.principle-title {
  font-size: 1.125rem;
  font-weight: 600;
  color: var(--vp-c-text-1);
  margin: 0 0 8px;
}

.principle-description {
  font-size: 0.9375rem;
  color: var(--vp-c-text-2);
  line-height: 1.6;
  margin: 0;
}

@media (max-width: 768px) {
  .philosophy-grid {
    grid-template-columns: repeat(2, 1fr);
    gap: 24px 20px;
  }

  .philosophy-heading {
    font-size: 1.75rem;
  }

  .philosophy-subheading {
    font-size: 1rem;
    margin-bottom: 32px;
  }
}

@media (max-width: 480px) {
  .home-philosophy {
    padding: 40px 16px;
  }

  .philosophy-grid {
    grid-template-columns: 1fr;
    gap: 24px;
  }

  .philosophy-heading {
    font-size: 1.5rem;
  }

  .philosophy-subheading {
    font-size: 0.9375rem;
  }

  .principle-title {
    font-size: 1rem;
  }

  .principle-description {
    font-size: 0.875rem;
  }
}
</style>
