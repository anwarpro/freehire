import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';

// A SOURCE-TEXT AUDIT, not a mounted-component test — web/ has no component-test
// infrastructure at all (see jobActionStrip.test.ts's own comment: no Svelte
// plugin, no DOM in vitest.config.ts).
//
// The account shell (/my/*) needed a footer, but not the marketing one: the four
// link columns, the popular-collections strip and popular collections are noise
// next to an app surface with its own sidebar nav. `compact` keeps everything below
// the bottom bar (copyright, cookie settings, social links, open-source note) and
// drops the rest, reusing the bottom bar's own markup rather than forking a second
// component.
const SOURCE = readFileSync(join(import.meta.dirname, 'Footer.svelte'), 'utf8');

// The offset of the `{/if}` that closes the `{#if !compact}` block: before it is the
// marketing chrome the compact footer drops, after it the bottom bar every footer keeps.
// Three of the tests below need that same boundary, and spelling the nested indexOf out
// each time is how the second and third copies came to differ from the first.
const GUARD_END = SOURCE.indexOf('{/if}', SOURCE.indexOf('{#each popular as collection'));

describe('Footer compact mode', () => {
  it('declares a compact prop defaulting to false', () => {
    expect(SOURCE).toMatch(/let\s*\{\s*compact\s*=\s*false\s*\}/);
  });

  it('skips navigation and popular collections when compact', () => {
    const groupsAt = SOURCE.indexOf('{#each groups as group');
    expect(SOURCE.lastIndexOf('{#if !compact}', groupsAt)).toBeGreaterThan(-1);
    expect(GUARD_END).toBeGreaterThan(SOURCE.indexOf('{#each popular as collection'));
  });

  it('still renders the bottom bar (copyright, cookie settings, social, open-source note) unconditionally', () => {
    const bottomBar = SOURCE.slice(GUARD_END);
    expect(bottomBar).toContain('©');
    expect(bottomBar).toContain('Cookie settings');
    expect(bottomBar).toContain('SOCIAL_LINKS');
    expect(bottomBar).toContain('View source on GitHub');
  });

  it("only borders the bottom bar's own top when compact (no divider from a section that isn't rendered)", () => {
    const commentAt = SOURCE.indexOf('Bottom bar:');
    const innerDivAt = SOURCE.indexOf('mx-auto flex max-w-6xl flex-col');
    const wrapperOpenTag = SOURCE.slice(commentAt, innerDivAt);
    expect(wrapperOpenTag).toContain('compact ?');
  });
});
