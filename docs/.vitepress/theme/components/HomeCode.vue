<script setup lang="ts">
const codeExample = `---
theme: terminal
---

# Database Demo

\`\`\`sql {driver: 'sqlite'}
SELECT name, population
FROM cities
WHERE country = 'Japan'
ORDER BY population DESC
LIMIT 5;
\`\`\`

Results appear live as you present.
Execute any query, see real data.`
</script>

<template>
  <section class="home-code">
    <h2 class="code-heading">Write slides like code</h2>
    <div class="code-window">
      <div class="code-header">
        <span class="code-dot code-dot-red"></span>
        <span class="code-dot code-dot-yellow"></span>
        <span class="code-dot code-dot-green"></span>
        <span class="code-filename">slides.md</span>
      </div>
      <pre class="code-content"><code>{{ codeExample }}</code></pre>
    </div>
  </section>
</template>

<style scoped>
.home-code {
  padding: 64px 24px;
  max-width: 800px;
  margin: 0 auto;
}

.code-heading {
  font-size: 1.75rem;
  font-weight: 600;
  color: var(--vp-c-text-1);
  text-align: center;
  margin: 0 0 32px;
}

.code-window {
  background: #1e1e2e;
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.25);
}

.code-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
  background: #181825;
  border-bottom: 1px solid #313244;
}

.code-dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
}

.code-dot-red {
  background: #f38ba8;
}

.code-dot-yellow {
  background: #f9e2af;
}

.code-dot-green {
  background: #a6e3a1;
}

.code-filename {
  margin-left: 8px;
  font-size: 0.875rem;
  color: #a6adc8;
  font-family: var(--vp-font-family-mono);
}

.code-content {
  margin: 0;
  padding: 24px;
  overflow-x: auto;
  font-family: var(--vp-font-family-mono);
  font-size: 0.9rem;
  line-height: 1.6;
  color: #cdd6f4;
  background: transparent;
}

.code-content code {
  white-space: pre;
  color: inherit;
  background: transparent;
}

@media (max-width: 640px) {
  .home-code {
    padding: 48px 20px;
  }

  .code-heading {
    font-size: 1.5rem;
  }

  .code-content {
    font-size: 0.8rem;
    padding: 16px;
  }
}
</style>
