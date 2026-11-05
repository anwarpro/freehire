import { describe, expect, it, vi } from 'vitest';
import { createApi } from './api.js';
import { teamRole } from './teamReview.js';

describe('team access affordances', () => {
  it('recognizes both manager roles and only active reviewer grants', () => {
    expect(teamRole(null)).toBeNull();
    expect(teamRole({ role: 'user' })).toBeNull();
    expect(teamRole({ role: 'user', team_role: 'reviewer' })).toBe('reviewer');
    expect(teamRole({ role: 'moderator' })).toBe('admin');
    expect(teamRole({ role: 'admin' })).toBe('admin');
  });
  it('uses session credentials, encoded slugs, and versioned request bodies', async () => {
    const fetcher = vi.fn().mockResolvedValue(new Response('{"data":{"version":2}}', { status: 200 }));
    const client = createApi(fetcher, '', {});
    const input = { assignee_id: 2, notes: 'Internal note', status: 'in_review' as const, follow_up: null, expected_version: 1 };
    await client.saveTeamReview('job/with space', input);
    expect(fetcher).toHaveBeenCalledWith('/api/v1/team/jobs/job%2Fwith%20space', expect.objectContaining({ method: 'PUT', credentials: 'include', body: JSON.stringify(input) }));
  });
});
