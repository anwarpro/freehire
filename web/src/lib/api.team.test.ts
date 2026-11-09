import { describe, expect, it, vi } from 'vitest';
import { createApi } from './api';

describe('team workspace HTTP contract', () => {
  const assignment = { assignee_id: 3, status: 'pending' as const, notes: 'testing', follow_up: '2026-11-12', expected_version: 0 };
  const reviewer = { name: 'Reviewer', email: 'reviewer@example.test', password: 'fixture-password', use_existing: false };
  const access = { name: 'Reviewer', active: false, expected_version: 1 };
  const mapping = { expected_version: 1, config: { tracks: [] } };
  const badge = { tracks: [], notes: 'Reviewed', expected_version: 0, mapping_version: 1, input_hash: 'fixture-hash' };
  const student = { student_ref: 'PH-1', student_name: 'Example', profile_url: '', reason: 'React experience' };
  const outcome = { student_name: 'Example', profile_url: '', reason: 'React experience', status: 'applied' as const, notes: 'Confirmed', follow_up: null, expected_version: 2 };
  const contact = { channel: 'other' as const, contacted_at: '2026-11-09T09:00:00Z', note: 'Shared manually', request_id: 'fixture-request', expected_version: 1 };
  const cases: {
    name: string; method: string; path: string; body: unknown; empty?: boolean;
    invoke: (client: ReturnType<typeof createApi>) => Promise<unknown>;
  }[] = [
    { name: 'assignment with notes and calendar follow-up', method: 'PUT', path: '/api/v1/team/jobs/intern', body: assignment, invoke: c => c.saveTeamReview('intern', assignment) },
    { name: 'unassignment without follow-up', method: 'PUT', path: '/api/v1/team/jobs/a%2Fb', body: { ...assignment, assignee_id: null, follow_up: null }, invoke: c => c.saveTeamReview('a/b', { ...assignment, assignee_id: null, follow_up: null }) },
    { name: 'reviewer creation', method: 'POST', path: '/api/v1/team/members', body: reviewer, invoke: c => c.createTeamMember(reviewer) },
    { name: 'reviewer update with empty response', method: 'PATCH', path: '/api/v1/team/members/4', body: access, empty: true, invoke: c => c.updateTeamMember(4, access) },
    { name: 'curriculum settings', method: 'PUT', path: '/api/v1/team/curriculum', body: mapping, invoke: c => c.saveCurriculumSettings(mapping) },
    { name: 'curriculum review', method: 'PUT', path: '/api/v1/team/curriculum/jobs/a%2Fb', body: badge, invoke: c => c.reviewCurriculum('a/b', badge) },
    { name: 'student reference creation', method: 'POST', path: '/api/v1/team/jobs/a%2Fb/students', body: student, invoke: c => c.createStudentFollowup('a/b', student) },
    { name: 'student outcome', method: 'PUT', path: '/api/v1/team/jobs/a%2Fb/students/7', body: outcome, invoke: c => c.updateStudentFollowup('a/b', 7, outcome) },
    { name: 'manual student contact', method: 'POST', path: '/api/v1/team/jobs/a%2Fb/students/7/contacts', body: contact, invoke: c => c.recordStudentContact('a/b', 7, contact) },
    { name: 'conversation creation', method: 'POST', path: '/api/v1/team/bot/sessions', body: {}, invoke: c => c.createTeamBotSession() },
  ];

  it.each(cases)('$name', async ({ method, path, body, invoke, empty }) => {
    // A real Request gives an untyped string body text/plain. Fiber cannot decode
    // that into a team JSON input, even though the body contains valid JSON text.
    const fetcher = vi.fn<typeof fetch>().mockImplementation(async (url, init) => {
      const request = new Request(`https://jobs.example.test${String(url)}`, init);
      if (request.headers.get('Content-Type') !== 'application/json') {
        return Response.json({ error: 'Invalid team request' }, { status: 400 });
      }
      expect(request.method).toBe(method);
      expect(new URL(request.url).pathname).toBe(path);
      expect(await request.json()).toEqual(body);
      expect(init?.credentials).toBe('include');
      return empty ? new Response(null, { status: 204 }) : Response.json({ data: { id: 7 } });
    });
    expect(await invoke(createApi(fetcher))).toEqual(empty ? undefined : { id: 7 });
  });

  it('deletes a conversation without parsing an empty 204 body', async () => {
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(new Response(null, { status: 204 }));
    await expect(createApi(fetcher).deleteTeamBotSession('session/id')).resolves.toBeUndefined();
    expect(fetcher).toHaveBeenCalledWith('/api/v1/team/bot/sessions/session%2Fid', expect.objectContaining({ method: 'DELETE', credentials: 'include' }));
  });

  it('preserves conflict errors for the review draft workflow', async () => {
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(Response.json({ error: 'Changed elsewhere' }, { status: 409 }));
    await expect(createApi(fetcher).saveTeamReview('intern', assignment)).rejects.toMatchObject({ status: 409, message: 'Changed elsewhere' });
  });
});
