import { GITHUB_URL } from './github.svelte';

// Profiles owned by this deployment.
export const SOCIAL_LINKS: { provider: string; label: string; href: string }[] = [
  { provider: 'github', label: 'PH Jobs source on GitHub', href: GITHUB_URL },
];
