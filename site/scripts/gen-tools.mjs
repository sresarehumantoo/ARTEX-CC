import { execFileSync } from 'node:child_process';
import { mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const repo = resolve(root, '..');
const data = resolve(root, 'src/data');
const out = resolve(root, 'src/content/docs/docs/tools');
const toolSource = 'https://github.com/SREsAreHumanToo/ARTEX-CC/tree/main/tools';

const tools = JSON.parse(
  execFileSync('go', ['run', './cmd/artex-cc', 'tools', '--json'], { cwd: repo, encoding: 'utf8' }),
);
const categories = JSON.parse(readFileSync(resolve(data, 'tool-categories.json'), 'utf8'));
const returns = JSON.parse(readFileSync(resolve(data, 'tool-returns.json'), 'utf8'));

const names = new Set(tools.map((t) => t.name));
const problems = [
  ...Object.keys(returns)
    .filter((n) => !names.has(n))
    .map((n) => `tool-returns.json names unknown tool ${n}`),
  ...[...new Set(tools.map((t) => t.category))]
    .filter((c) => !(c in categories))
    .map((c) => `category "${c}" is missing from tool-categories.json`),
];
if (problems.length) {
  throw new Error(problems.join('\n'));
}

const slug = (s) => s.toLowerCase().replace(/[^a-z0-9]+/g, '-');
const prose = (s) => s.replace(/</g, '&lt;');
const cell = (s) => prose(s).replace(/\|/g, '\\|').replace(/\s*\n\s*/g, ' ');
const capitalize = (s) => s.charAt(0).toUpperCase() + s.slice(1);
const summary = (s) => s.split('\n')[0].split(/(?<=\.)\s/)[0];

function typeOf(schema) {
  if (schema.type === 'array') {
    const item = schema.items ?? {};
    return item.type === 'object' ? 'array of objects' : `array of ${item.type ?? 'any'}`;
  }
  return schema.type ?? 'any';
}

function rows(schema, prefix = '') {
  const required = new Set(schema.required ?? []);
  return Object.entries(schema.properties ?? {})
    .sort(([a], [b]) => required.has(b) - required.has(a))
    .flatMap(([name, prop]) => {
    const path = prefix + name;
    let desc = capitalize(prop.description ?? '');
    if (prop.enum) {
      desc += `${desc ? '. ' : ''}One of ${prop.enum.map((v) => `\`${v}\``).join(', ')}`;
    }
    const row = `| ${path} | ${typeOf(prop)} | ${required.has(name) ? 'yes' : 'no'} | ${cell(desc)} |`;
    const nested =
      prop.type === 'object' ? prop : prop.type === 'array' && prop.items?.type === 'object' ? prop.items : null;
    return [row, ...(nested ? rows(nested, prop.type === 'array' ? `${path}[].` : `${path}.`) : [])];
  });
}

function toolSection(t) {
  const flags = [t.readOnly && 'Read-only.', t.requiresProxy && 'Registered only when `serve` runs with `--proxy`.']
    .filter(Boolean)
    .join(' ');
  const params = rows(t.inputSchema);
  return [
    `## \`${t.name}\``,
    flags && `_${flags}_`,
    prose(t.description).replace(/\n/g, '\n\n'),
    params.length
      ? ['| Parameter | Type | Required | Description |', '|---|---|---|---|', ...params].join('\n')
      : '_No parameters._',
    returns[t.name] && `**Returns:** ${returns[t.name]}`,
  ]
    .filter(Boolean)
    .join('\n\n');
}

const frontmatter = (fields) =>
  `---\n${Object.entries(fields)
    .map(([k, v]) => `${k}: ${typeof v === 'object' ? `\n${Object.entries(v).map(([a, b]) => `  ${a}: ${JSON.stringify(b)}`).join('\n')}` : JSON.stringify(v)}`)
    .join('\n')}\neditUrl: false\n---\n\n`;

const generated = `Generated from \`artex-cc tools --json\`. Descriptions and parameters come from the [tool definitions](${toolSource}).`;

const order = Object.keys(categories);
const byCategory = order.map((c) => ({ name: c, tools: tools.filter((t) => t.category === c) }));

rmSync(out, { recursive: true, force: true });
mkdirSync(out, { recursive: true });

byCategory.forEach(({ name, tools: list }, i) => {
  const body = [generated, ...list.map(toolSection)].join('\n\n');
  writeFileSync(
    resolve(out, `${slug(name)}.md`),
    frontmatter({ title: name, description: categories[name], sidebar: { order: i + 1 } }) + body + '\n',
  );
});

const index = [
  `ARTEX registers ${tools.length} MCP tools across ${order.length} categories. Claude calls them on its own based on what the engagement needs; you rarely call one by name. Each category page documents every parameter and return shape.`,
  generated,
  ...byCategory.map(({ name, tools: list }) =>
    [
      `## ${name}`,
      `${list.length} tools, [full parameters](/docs/tools/${slug(name)}/).`,
      [
        '| Tool | What it does |',
        '|---|---|',
        ...list.map((t) => `| [\`${t.name}\`](/docs/tools/${slug(name)}/#${t.name}) | ${cell(summary(t.description))} |`),
      ].join('\n'),
    ].join('\n\n'),
  ),
].join('\n\n');
writeFileSync(
  resolve(out, 'index.md'),
  frontmatter({
    title: 'Tool reference',
    description: `All ${tools.length} ARTEX MCP tools by category, with a one-line summary of each.`,
    sidebar: { label: 'Overview', order: 0 },
  }) +
    index +
    '\n',
);

mkdirSync(data, { recursive: true });
writeFileSync(
  resolve(data, 'tool-stats.json'),
  JSON.stringify({ tools: tools.length, categories: order.length, proxyOnly: tools.filter((t) => t.requiresProxy).length }) + '\n',
);
