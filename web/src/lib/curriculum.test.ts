import { describe, expect, it } from 'vitest';
import { curriculumLabel, formatSkillRules, parseSkillRules } from './curriculum';
describe('curriculum mapping drafts', () => {
  it('round trips editable names and literal aliases', () => {
    const skills = [{ name: 'Node.js', aliases: ['node.js', 'nodejs'] }, { name: 'C++', aliases: ['c++'] }];
    expect(parseSkillRules(formatSkillRules(skills))).toEqual(skills);
  });
  it('identifies incomplete rules before making a request', () => {
    expect(() => parseSkillRules('Python')).toThrow('Line 1');
    expect(() => parseSkillRules('Python | python,')).toThrow('nonempty');
    expect(() => parseSkillRules('')).toThrow('1–30');
  });
  it('keeps an archived unknown identifier readable in history', () => { expect(curriculumLabel('legacy', [])).toBe('legacy'); });
});
