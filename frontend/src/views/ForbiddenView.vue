<template>
  <section class="state">
    <h1>You do not have access</h1>
    <p>This page is only available to administrators.</p>
    <RouterLink class="btn btn-primary" :to="{ name: 'tickets' }">Back to tickets</RouterLink>
  </section>
</template>
