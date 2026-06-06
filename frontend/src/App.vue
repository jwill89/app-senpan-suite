<script setup lang="ts">
/**
 * App shell: a persistent header, the router outlet, and the toast host.
 * Individual screens live in src/views and are mounted by Vue Router.
 */
import { RouterLink, RouterView } from 'vue-router'
import ToastHost from '@/components/ToastHost.vue'
</script>

<template>
  <div class="app">
    <header class="topbar">
      <RouterLink to="/" class="brand">Bingo</RouterLink>
      <nav>
        <RouterLink to="/raffles">Raffles</RouterLink>
        <RouterLink to="/admin">Admin</RouterLink>
      </nav>
    </header>

    <main class="content">
      <RouterView />
    </main>

    <ToastHost />
  </div>
</template>

<style scoped>
.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 20px;
  border-bottom: 1px solid #26263a;
}
.brand {
  font-size: 1.4rem;
  font-weight: 700;
  color: var(--accent, #e53170);
  text-decoration: none;
}
.topbar nav {
  display: flex;
  gap: 16px;
}
.topbar nav a {
  color: #c4c4d4;
  text-decoration: none;
  font-size: 0.95rem;
}
.topbar nav a:hover {
  color: #fff;
}
.content {
  max-width: 760px;
  margin: 0 auto;
  padding: 28px 20px;
}
</style>
