// Reexport lib modules for convenient imports via $lib
export { api } from './api/client.js';

// Note: auth store uses Svelte 5 runes ($state) and must be imported directly:
// import { auth } from '$lib/stores/auth';
