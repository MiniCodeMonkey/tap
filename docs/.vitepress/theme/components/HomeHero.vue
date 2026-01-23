<template>
  <section class="home-hero">
    <div class="hero-content">
      <h1 class="hero-headline">Presentations for developers</h1>
      <p class="hero-subheadline">
        Markdown slides with live code, beautiful themes, and zero config.
      </p>
      <div class="hero-install">
        <code class="install-command">brew install tap-slides</code>
      </div>
      <div class="hero-actions">
        <a href="/getting-started" class="hero-btn hero-btn-primary">
          Get Started
        </a>
        <a href="#" class="hero-btn hero-btn-secondary">
          View on GitHub
        </a>
      </div>
    </div>
  </section>
</template>

<style scoped>
.home-hero {
  padding: 80px 24px;
  text-align: center;
  max-width: 800px;
  margin: 0 auto;
}

.hero-headline {
  font-size: 3.5rem;
  font-weight: 700;
  line-height: 1.1;
  margin: 0 0 24px;
  color: var(--vp-c-text-1);
  letter-spacing: -0.02em;
}

.hero-subheadline {
  font-size: 1.25rem;
  color: var(--vp-c-text-2);
  margin: 0 0 40px;
  line-height: 1.6;
}

.hero-install {
  margin-bottom: 40px;
}

.install-command {
  display: inline-block;
  padding: 12px 24px;
  background: var(--vp-c-bg-alt);
  border: 1px solid var(--vp-c-divider);
  border-radius: 8px;
  font-family: var(--vp-font-family-mono);
  font-size: 1rem;
  color: var(--vp-c-text-1);
}

.hero-actions {
  display: flex;
  gap: 16px;
  justify-content: center;
  flex-wrap: wrap;
}

.hero-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 12px 24px;
  border-radius: 8px;
  font-size: 1rem;
  font-weight: 500;
  text-decoration: none;
  transition: all 0.2s ease;
}

.hero-btn-primary {
  background: var(--vp-c-brand-1);
  color: #ffffff;
}

.hero-btn-primary:hover {
  background: var(--vp-c-brand-2);
}

.hero-btn-secondary {
  background: transparent;
  color: var(--vp-c-text-1);
  border: 1px solid var(--vp-c-divider);
}

.hero-btn-secondary:hover {
  border-color: var(--vp-c-text-2);
}

@media (max-width: 640px) {
  .home-hero {
    padding: 48px 20px;
  }

  .hero-headline {
    font-size: 2.5rem;
  }

  .hero-subheadline {
    font-size: 1.125rem;
  }
}
</style>
