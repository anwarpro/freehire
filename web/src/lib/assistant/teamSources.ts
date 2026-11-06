import type { ChatState } from './chat';
type Tool = ChatState['messages'][number]['tools'][number];
interface Source { title: string; jobLink: string; sourceURL: string; closed: boolean }
function safeLink(value: unknown, relative = false): string {
  if (typeof value !== 'string') return '';
  if (relative && /^\/jobs\/[^/]/.test(value) && !/[\\\r\n]/.test(value)) return value;
  try { const u = new URL(value); return ['http:', 'https:'].includes(u.protocol) && !u.username && !u.password ? u.href : ''; } catch { return ''; }
}
/** Sources come from successful tool results, independently of the model's prose. */
export function teamJobSources(tools: Tool[]): Source[] {
  const sources = new Map<string, Source>();
  for (const tool of tools) {
    if (tool.isError || !tool.result || !['search_jobs', 'get_job', 'course_jobs'].includes(tool.name)) continue;
    try {
      const result = JSON.parse(tool.result);
      for (const item of Array.isArray(result.jobs) ? result.jobs : [result]) {
        const job = item?.job ?? item?.posting;
        const link = safeLink(item?.job_link, true);
        if (link && typeof job?.title === 'string') sources.set(link, { title: job.title, jobLink: link, sourceURL: safeLink(item.source_url), closed: Boolean(job.closed_at) });
      }
    } catch { /* Truncated or malformed results cannot establish a verified source. */ }
  }
  return [...sources.values()];
}
