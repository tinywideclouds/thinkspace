const fs = require('fs');
const path = require('path');

const DATA_DIR = path.join(__dirname, 'data');
const OUTPUT_FILE = path.join(__dirname, '../../../apps/llm/chat-app/src/app/mocks/mock-graph.json');

const graph = { 
  events: [], 
  digests: {}, 
  focus: { primaryFocus: ['ui'], relatedContext: ['angular'], dormantLenses: [] },
  scenarios: {} 
};

// 1. Parse JSONL Shards
const logsDir = path.join(DATA_DIR, 'logs');
if (fs.existsSync(logsDir)) {
  const shards = fs.readdirSync(logsDir).filter(f => f.endsWith('.jsonl')).sort();
  for (const shard of shards) {
    const lines = fs.readFileSync(path.join(logsDir, shard), 'utf8').split('\n');
    for (const line of lines) {
      if (line.trim()) {
        const ev = JSON.parse(line);
        if (!ev.tombstoned) graph.events.push(ev);
      }
    }
  }
}

// 2. Parse Digests
const digestsDir = path.join(DATA_DIR, 'digests');
if (fs.existsSync(digestsDir)) {
  const mdFiles = fs.readdirSync(digestsDir).filter(f => f.endsWith('.md'));
  for (const file of mdFiles) {
    const content = fs.readFileSync(path.join(digestsDir, file), 'utf8');
    const lines = content.split('\n');
    
    let isSticky = false;
    let summary = content;
    
    if (lines[0] === '---') {
      const endIdx = lines.indexOf('---', 1);
      const frontmatter = lines.slice(1, endIdx).join('\n');
      isSticky = frontmatter.includes('sticky: true');
      summary = lines.slice(endIdx + 1).join('\n').trim();
    }
    
    const id = file.replace('.md', '');
    graph.digests[id] = { id, summary, isSticky };
  }
}

// 3. Parse Scenarios
const scenariosDir = path.join(DATA_DIR, 'scenarios');
if (fs.existsSync(scenariosDir)) {
  const scenarioFiles = fs.readdirSync(scenariosDir).filter(f => f.endsWith('.json'));
  for (const file of scenarioFiles) {
    const scenarioName = file.replace('.json', '');
    try {
      const scenarioData = JSON.parse(fs.readFileSync(path.join(scenariosDir, file), 'utf8'));
      graph.scenarios[scenarioName] = scenarioData;
    } catch (e) {
      console.error(`Error parsing scenario ${file}:`, e);
    }
  }
}

fs.mkdirSync(path.dirname(OUTPUT_FILE), { recursive: true });
fs.writeFileSync(OUTPUT_FILE, JSON.stringify(graph, null, 2));

console.log(`✅ Generated mock graph with ${graph.events.length} events, ${Object.keys(graph.digests).length} digests, and ${Object.keys(graph.scenarios).length} scenarios.`);