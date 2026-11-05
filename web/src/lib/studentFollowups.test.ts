import { describe, expect, it } from 'vitest';
import { safeStudentLink, studentMessage, bangladeshDateTime } from './studentFollowups.js';
describe('student follow-up helpers', () => {
  it('prepares a source-linked message without claiming eligibility', () => {
    const message = studentMessage({ student_name: 'Example', title: 'React developer', company: 'Company', job_url: 'https://company.test/job' });
    expect(message).toContain('Hi Example,'); expect(message).toContain('https://company.test/job'); expect(message).toContain('check the original requirements and deadline');
  });
  it('refuses executable and credentialed links', () => {
    for (const value of ['javascript:alert(1)', 'data:text/html,hello', 'https://user:pass@school.test/42', '/relative']) expect(safeStudentLink(value)).toBe('');
    expect(safeStudentLink('https://school.test/42')).toBe('https://school.test/42');
  });
  it('uses Bangladesh time across the UTC day boundary', () => { expect(bangladeshDateTime(new Date('2026-11-05T20:00:00Z'))).toBe('2026-11-06T02:00'); });
});
