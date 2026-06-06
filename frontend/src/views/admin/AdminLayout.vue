<script setup lang="ts">
/**
 * Admin shell: a tab bar over a nested <RouterView>. Only migrated tabs are
 * shown; the rest of the legacy dashboard (game, patterns, themes, raffle
 * admin) is still served by the old SPA and listed here as pending.
 */
import { useRouter } from 'vue-router'
import { useAuth } from '@/composables/useAuth'
import { useToast } from '@/composables/useToast'

const router = useRouter()
const { logout } = useAuth()
const { show } = useToast()

const tabs = [
  { to: { name: 'admin-game' }, label: 'Game' },
  { to: { name: 'admin-cards' }, label: 'Cards' },
  { to: { name: 'admin-winners' }, label: 'Winners Log' },
  { to: { name: 'admin-settings' }, label: 'Settings' },
]

const pending = ['Patterns', 'Themes', 'Raffle admin']

async function doLogout() {
  await logout()
  show('Logged out', 'info')
  void router.push('/')
}
</script>

<template>
  <div class="admin">
    <div class="admin-bar">
      <nav class="tabs">
        <RouterLink v-for="t in tabs" :key="t.label" :to="t.to" class="tab">
          {{ t.label }}
        </RouterLink>
      </nav>
      <button type="button" class="logout" @click="doLogout">Log out</button>
    </div>

    <p class="pending">Coming soon: {{ pending.join(' · ') }}</p>

    <RouterView />
  </div>
</template>

<style scoped>
.admin-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  border-bottom: 1px solid #26263a;
  padding-bottom: 12px;
  margin-bottom: 8px;
}
.tabs {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}
.tab {
  padding: 8px 14px;
  border-radius: 8px;
  text-decoration: none;
  color: #c4c4d4;
  font-weight: 600;
}
.tab:hover {
  background: #20202e;
}
.tab.router-link-active {
  background: var(--accent, #e53170);
  color: #fff;
}
.logout {
  border-color: #3a3a4a;
}
.pending {
  color: #6f6f86;
  font-size: 0.82rem;
  margin: 0 0 18px;
}
</style>
