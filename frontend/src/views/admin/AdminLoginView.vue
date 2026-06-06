<script setup lang="ts">
/** Admin password login. On success, redirects to the originally requested
 *  admin route (or the dashboard). */
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuth } from '@/composables/useAuth'

const router = useRouter()
const route = useRoute()
const { login } = useAuth()

const password = ref('')
const error = ref('')
const busy = ref(false)

async function submit() {
  error.value = ''
  busy.value = true
  try {
    await login(password.value)
    password.value = ''
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/admin'
    await router.replace(redirect)
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Login failed'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="login">
    <h1>Admin login</h1>
    <form @submit.prevent="submit">
      <label>
        Password
        <input v-model="password" type="password" autocomplete="current-password" autofocus />
      </label>
      <p v-if="error" class="error">{{ error }}</p>
      <button type="submit" :disabled="busy || !password">
        {{ busy ? 'Signing in…' : 'Sign in' }}
      </button>
    </form>
  </div>
</template>

<style scoped>
.login {
  max-width: 360px;
}
form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
label {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
input {
  width: 100%;
}
.error {
  color: #ff6b6b;
  margin: 0;
}
</style>
