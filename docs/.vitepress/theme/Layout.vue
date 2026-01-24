<script setup lang="ts">
import DefaultTheme from 'vitepress/theme'
import { useData, useRoute } from 'vitepress'
import { computed } from 'vue'
import LLMActions from './components/LLMActions.vue'

const { Layout } = DefaultTheme
const { frontmatter } = useData()
const route = useRoute()

// Show LLM actions on doc pages (not homepage, not custom page layouts)
const showLLMActions = computed(() => {
  // Skip homepage
  if (route.path === '/') return false

  // Skip custom page layouts
  if (frontmatter.value.layout === 'page') return false

  // Skip if explicitly disabled
  if (frontmatter.value.llmActions === false) return false

  return true
})
</script>

<template>
  <Layout>
    <template #doc-before>
      <LLMActions v-if="showLLMActions" />
    </template>
  </Layout>
</template>
