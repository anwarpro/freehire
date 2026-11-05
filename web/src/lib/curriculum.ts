export interface CurriculumSkill { name: string; aliases: string[] }
export interface CurriculumTrack { id: string; label: string; enabled: boolean; minimum: number; skills: CurriculumSkill[] }
export interface CurriculumSettings { version: number; config: { tracks: CurriculumTrack[] }; progress: { mapping_version: number; cursor_id: number; upper_id: number; complete: boolean; processed: number; last_run_at: string | null; last_error: string } }
export interface CurriculumEvidence { skill: string; alias: string; kind: 'required' | 'preferred' | 'title' | 'mentioned'; text: string }
export interface CurriculumDetail {
  job_id: number; mapping_version: number; input_hash: string;
  evidence: { matches: { track_id: string; evidence: CurriculumEvidence[] }[]; truncated: boolean };
  tracks: CurriculumTrack[]; suggested_tracks: string[]; reviewed_tracks: string[] | null; effective_tracks: string[];
  review_version: number; review_notes: string; reviewed: boolean; review_stale: boolean;
}
export interface CurriculumReviewInput { tracks: string[] | null; notes: string; expected_version: number; mapping_version: number; input_hash: string }
export interface CurriculumRow { job_id: number; public_slug: string; title: string; company: string; location: string; source: string; closed_at: string | null; suggested_tracks: string[]; reviewed_tracks: string[] | null; effective_tracks: string[]; reviewed: boolean; review_stale: boolean; update_pending: boolean }
export interface CurriculumFilter { track?: string; source?: string; state?: string; review?: string; offset?: number }
export interface CurriculumQueue { data: CurriculumRow[]; meta: { has_more: boolean; offset: number; limit: number } }
export interface CurriculumEvent { id: number; tracks: string[] | null; notes: string; version: number; mapping_version: number; created_at: string; actor: string }
export interface CurriculumConfigEvent { version: number; created_at: string; actor: string; payload: { tracks: CurriculumTrack[] } }
export const curriculumLabel = (id: string, tracks: CurriculumTrack[]): string => tracks.find((t) => t.id === id)?.label ?? id;
export function formatSkillRules(skills: CurriculumSkill[]): string { return skills.map((s) => `${s.name} | ${s.aliases.join(', ')}`).join('\n'); }
export function parseSkillRules(text: string): CurriculumSkill[] {
  const lines = text.split('\n').map((s) => s.trim()).filter(Boolean);
  if (!lines.length || lines.length > 30) throw new Error('Enter 1–30 skill rules, one per line.');
  return lines.map((line, i) => {
    const parts = line.split('|');
    const name = parts[0]?.trim(); const aliasesText = parts[1];
    if (parts.length !== 2 || !name || aliasesText === undefined) throw new Error(`Line ${i + 1}: use Skill name | alias, alias.`);
    const aliases = aliasesText.split(',').map((s) => s.trim());
    if (aliases.some((s) => !s) || aliases.length > 8) throw new Error(`Line ${i + 1}: enter 1–8 nonempty aliases.`);
    return { name, aliases };
  });
}

export function curriculumTime(value: string): string { return new Date(value).toLocaleString('en-GB', { timeZone: 'Asia/Dhaka', dateStyle: 'medium', timeStyle: 'short' }); }
