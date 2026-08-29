// docs/PLANE.md is synced verbatim from an external template (ralph-loop's
// output/plane/PLANE.md) and must stay byte-identical to it. oxfmt reformats
// markdown (e.g. collapsing italics, mangling underscores inside inline-code
// spans like `PR_CI_CHECK_PATTERNS` into `PR*CI*CHECK*PATTERNS`), corrupting
// literal command syntax the ralph agent copies verbatim from that file.
const EXCLUDE = "docs/PLANE.md";

export default {
  "*.{js,jsx,ts,tsx,cjs,mjs,cts,mts,json,css,md}": (files) => {
    const matches = files.filter((file) => !file.endsWith(EXCLUDE));
    return matches.length ? [`pnpm exec oxfmt --no-error-on-unmatched-pattern ${matches.join(" ")}`] : [];
  },
  "*.{js,jsx,ts,tsx,cjs,mjs,cts,mts}": ["pnpm exec oxlint --fix --deny-warnings"],
};
