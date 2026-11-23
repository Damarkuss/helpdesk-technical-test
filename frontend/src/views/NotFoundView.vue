<template>
  <section class="state">
    <h1>Page not found</h1>
    <p>The page you are looking for does not exist or has moved.</p>
    <RouterLink class="btn btn-primary" :to="{ name: 'tickets' }">Back to tickets</RouterLink>
  </section>
</template>
