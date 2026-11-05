import { error } from '@sveltejs/kit';
import { ApiError } from '#lib/api.js';
import { serverApi } from '#lib/server/api.js';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ url, fetch }) => {
  try {
    const result = await serverApi(fetch).closedJobs(url.searchParams);
    return { ...result, query: url.searchParams.toString() };
  } catch (err) {
    if (err instanceof ApiError && err.status === 400) error(400, err.message);
    error(503, 'Could not load closed jobs. Please try again.');
  }
};
