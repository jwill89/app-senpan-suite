import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '@/views/HomeView.vue'

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
    // Unknown paths → home.
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
  scrollBehavior() {
    return { top: 0 }
  },
})

export default router
