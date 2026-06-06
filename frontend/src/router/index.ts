import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '@/views/HomeView.vue'
import { useAuth } from '@/composables/useAuth'

/**
 * Client-side routes. Non-home views are lazy-loaded so each becomes its own
 * chunk — a concrete payoff of the build step over the single legacy bundle.
 *
 * History mode (clean URLs) requires the production server to fall back to
 * index.html for unknown paths. With Apache: `FallbackResource /index.html`
 * (or a rewrite) on the SPA vhost. The dev server handles this automatically.
 */
const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: HomeView },
    {
      path: '/play/:cardId',
      name: 'play',
      component: () => import('@/views/PlayerView.vue'),
      props: true,
    },
    {
      path: '/raffles',
      name: 'raffles',
      component: () => import('@/views/RafflesListView.vue'),
    },
    {
      path: '/raffles/:id',
      name: 'raffle',
      component: () => import('@/views/RaffleDetailView.vue'),
      props: true,
    },

    // ── Admin ──
    {
      path: '/admin/login',
      name: 'admin-login',
      component: () => import('@/views/admin/AdminLoginView.vue'),
    },
    {
      path: '/admin',
      component: () => import('@/views/admin/AdminLayout.vue'),
      meta: { requiresAuth: true },
      redirect: { name: 'admin-game' },
      children: [
        {
          path: 'game',
          name: 'admin-game',
          component: () => import('@/views/admin/AdminGameView.vue'),
        },
        {
          path: 'cards',
          name: 'admin-cards',
          component: () => import('@/views/admin/AdminCardsView.vue'),
        },
        {
          path: 'winners',
          name: 'admin-winners',
          component: () => import('@/views/admin/AdminWinnersLogView.vue'),
        },
        {
          path: 'settings',
          name: 'admin-settings',
          component: () => import('@/views/admin/AdminSettingsView.vue'),
        },
      ],
    },

    // Unknown paths → home.
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
  scrollBehavior() {
    return { top: 0 }
  },
})

// Guard admin routes: confirm an authenticated session (cookie-based) before
// entry, redirecting to the login page with a return path otherwise.
router.beforeEach(async (to) => {
  if (!to.meta.requiresAuth) return true
  const { ensure } = useAuth()
  if (await ensure()) return true
  return { name: 'admin-login', query: { redirect: to.fullPath } }
})

export default router
