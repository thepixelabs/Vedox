/*
 * content.ts — The single source of truth for every word on vedox.pixelabs.net.
 *
 * Tech-writers: edit THIS file, not the markup. Markup is a dumb renderer.
 * Every string below is user-visible. Keep the voice: direct, technical,
 * slightly dry. No exclamation marks. No "revolutionize", "empower",
 * "unleash", "synergy", or "platform".
 */

export const site = {
	name: 'Vedox',
	domain: 'vedox.pixelabs.net',
	url: 'https://vedox.pixelabs.net',
	tagline: 'docs that live where the code lives.',
	description:
		'Vedox is a local-first documentation CMS for developers. Markdown in, Git history out. No server, no account, no telemetry.',
	github: 'https://github.com/thepixelabs/vedox',
	installCommand: 'npm install -g vedox',
	runCommand: 'vedox dev',
} as const;

export const nav = {
	anchors: [
		{ id: 'pitch', label: 'How it works' },
		{ id: 'pillars', label: 'Why' },
		{ id: 'editor', label: 'Editor' },
		{ id: 'features', label: 'Features' },
		{ id: 'compare', label: 'Compare' },
		{ id: 'faq', label: 'FAQ' },
		{ id: 'roadmap', label: 'Roadmap' },
	],
} as const;

export const hero = {
	eyebrow: 'Local-first documentation CMS',
	headline: 'docs that live where the code lives.',
	sub: 'markdown in, git history out. no server, no account, no asking permission.',
	primaryCta: { label: 'Copy install command', command: site.installCommand },
	secondaryCta: { label: 'View on GitHub', href: site.github },
	trustLine:
		'open source. PolyForm Shield 1.0.0 licensed. zero telemetry. zero outbound network calls. your words stay on your machine until you decide otherwise.',
} as const;

export const pitch = {
	id: 'pitch',
	kicker: 'The 15-second pitch',
	title: 'point it at a repo. edit like a document. commit like a developer.',
	body: 'Vedox is a single static binary. Run it against a folder of Markdown and it opens a local editor that reads and writes the same files Git already tracks. Close the tab, `git diff`, commit. That\u2019s the whole loop.',
	terminal: [
		{ kind: 'prompt', text: 'cd ~/code/my-project' },
		{ kind: 'prompt', text: 'vedox dev' },
		{ kind: 'output', text: 'scanning repo\u2026 42 markdown files indexed' },
		{ kind: 'output', text: 'editor running at http://127.0.0.1:4123' },
		{ kind: 'output', text: 'watching for changes. press ctrl-c to stop.' },
	],
} as const;

export const pillars = {
	id: 'pillars',
	kicker: 'Why Vedox exists',
	title: 'three principles, no compromises.',
	items: [
		{
			icon: 'git',
			title: 'your repo is the source of truth.',
			body: 'docs live next to the code they describe. not in a SaaS tool three clicks away, not in a wiki nobody updates. in the repo. versioned. reviewable. `git diff` tells you what changed. `git blame` tells you who.',
		},
		{
			icon: 'disk',
			title: 'local-first means you-first.',
			body: 'vedox runs on your machine. localhost only. no account creation, no team provisioning, no pricing page. works offline. works on an airplane. your docs are files on disk. they go where you go.',
		},
		{
			icon: 'lock',
			title: 'open source. no asterisks.',
			body: 'PolyForm Shield 1.0.0 licensed. read the code, fork the code, change the code. no "community edition" with the good parts removed. no telemetry. no data collection. the entire tool is yours to audit, modify, and self-host forever.',
		},
	],
} as const;

export const editor = {
	id: 'editor',
	kicker: 'The editor',
	title: 'dual-mode editing that respects your markdown.',
	body: 'WYSIWYG when you\u2019re writing. raw markdown when you mean it. switch mid-sentence \u2014 nothing lost, nothing reformatted. round-trip fidelity guaranteed: `serialize(parse(md)) === md`.',
	tabs: [
		{
			id: 'wysiwyg',
			label: 'WYSIWYG',
			description: 'Tiptap-powered rich editor with bubble toolbar, auto-save, and inline formatting.',
		},
		{
			id: 'source',
			label: 'Source',
			description: 'CodeMirror 6 with Shiki syntax highlighting. 15 languages, zero WASM.',
		},
		{
			id: 'mermaid',
			label: 'Mermaid',
			description:
				'Diagrams authored in text, rendered inline. Sequence, flowchart, class, state \u2014 with edit popover and SVG caching.',
		},
		{
			id: 'code',
			label: 'Code blocks',
			description: 'Syntax-highlighted code blocks with language detection. Shiki JS engine, no WASM overhead.',
		},
		{
			id: 'frontmatter',
			label: 'Frontmatter',
			description:
				'Structured metadata panel. 16 lint rules catch missing titles, malformed dates, invalid tags \u2014 before commit, not after deploy.',
		},
	],
	callouts: [
		{ label: 'Source \u2194 WYSIWYG, zero data loss' },
		{ label: 'Scoped to your repo \u2014 no sidebar bloat' },
		{ label: 'Search across every doc, instant' },
		{ label: 'Dark-first. Keyboard-first. Geist Sans + JetBrains Mono.' },
	],
} as const;

export const features = {
	id: 'features',
	kicker: 'Everything it does',
	title: 'the full list, no marketing fog.',
	groups: [
		{
			category: 'Editor',
			icon: 'edit',
			items: [
				{
					title: 'Dual-mode WYSIWYG',
					body: 'Tiptap on one side, CodeMirror 6 on the other. switch mid-sentence.',
				},
				{
					title: 'Round-trip fidelity',
					body: 'what you commit is exactly what vedox rendered. `serialize(parse(md)) === md`.',
				},
				{
					title: 'Mermaid diagrams',
					body: 'sequence, flowchart, class, state \u2014 authored in text, rendered inline with edit popover.',
				},
				{
					title: 'Shiki syntax highlighting',
					body: '15 preloaded languages. JS regex engine, zero WASM. code blocks that look right.',
				},
				{
					title: 'Bubble toolbar',
					body: 'floating inline formatting on text selection. bold, italic, link, code \u2014 no menu diving.',
				},
				{
					title: 'Auto-save',
					body: '800ms debounce. every edit is persisted. "Publish" means git commit with a customizable message.',
				},
			],
		},
		{
			category: 'Navigation & search',
			icon: 'search',
			items: [
				{
					title: 'Command palette',
					body: 'Cmd+K opens 4 modes: search, commands (>), tags (#), path navigation (/). one shortcut, no mouse.',
				},
				{
					title: 'Full-text search',
					body: 'SQLite FTS5 with BM25 ranking. the entire corpus indexed locally, scored by relevance.',
				},
				{
					title: 'Quick file open',
					body: 'Cmd+P. type a filename, open it. same muscle memory as your code editor.',
				},
				{
					title: 'Multi-pane layout',
					body: '1 to 4 panes. drag the dividers. per-pane reading width and editor mode. Cmd+\\ to split.',
				},
			],
		},
		{
			category: 'Design system',
			icon: 'palette',
			items: [
				{
					title: '5 curated themes',
					body: 'Graphite, Eclipse, Ember, Paper, Solar. OKLCH color space \u2014 perceptually uniform, not "dark mode with wrong blues."',
				},
				{
					title: '3 density modes',
					body: 'Compact, Comfortable, Cozy. a single multiplier scales all spacing to match your screen.',
				},
				{
					title: 'Variable fonts',
					body: 'Geist Sans, Fraunces, JetBrains Mono. self-hosted woff2 with metric-override FOUT prevention.',
				},
				{
					title: 'Motion system',
					body: '3 durations, 4 easing curves. `prefers-reduced-motion` kills all animation automatically.',
				},
			],
		},
		{
			category: 'AI & agents',
			icon: 'bolt',
			items: [
				{
					title: 'AI review queue',
					body: 'grammar, clarity, structure, style \u2014 flagged and queued. accept, reject, or dismiss. not auto-corrected.',
				},
				{
					title: 'Provider config drawer',
					body: 'manage Claude Code, Codex, and Gemini CLI configuration per-project from inside your docs editor.',
				},
				{
					title: 'Agent-safe API',
					body: 'HMAC-authed. agents propose changes to a staging branch. you review. agents touch docs, never code.',
				},
			],
		},
		{
			category: 'Developer experience',
			icon: 'terminal',
			items: [
				{
					title: 'Single binary',
					body: 'one `npm install -g vedox`. no runtime dependencies, no Docker, no daemon.',
				},
				{
					title: 'Zero config start',
					body: 'run `vedox dev` in any folder with markdown. framework detection handles the rest.',
				},
				{
					title: '5 document templates',
					body: 'ADR, API Reference, Runbook, README, How-To. opinionated structure, skip the blank page.',
				},
				{
					title: 'Frontmatter linter',
					body: '16 rules. missing titles, malformed dates, invalid tags \u2014 caught before commit.',
				},
				{
					title: 'Workspace scanner',
					body: 'detects Astro, MkDocs, Jekyll, Docusaurus, or bare README on first scan. no import wizard.',
				},
				{
					title: 'Locked down by default',
					body: 'loopback-only. path traversal protection. secret file blocklist. DOMPurify. CSP headers. zero outbound calls.',
				},
			],
		},
	],
} as const;

export const comparison = {
	id: 'compare',
	kicker: 'How it stacks up',
	title: 'vedox vs the alternatives.',
	tools: ['Vedox', 'Obsidian', 'VS Code', 'Confluence', 'GitBook', 'Docusaurus'] as const,
	rows: [
		{
			feature: 'Local-first (no server)',
			values: [true, true, true, false, false, true],
		},
		{
			feature: 'Git-native (files on disk)',
			values: [true, false, true, false, false, true],
		},
		{
			feature: 'WYSIWYG editor',
			values: [true, true, false, true, true, false],
		},
		{
			feature: 'Source mode',
			values: [true, true, true, false, false, true],
		},
		{
			feature: 'Round-trip fidelity',
			values: [true, 'partial', 'n/a', 'n/a', 'n/a', 'n/a'],
		},
		{
			feature: 'Full-text search (ranked)',
			values: [true, true, false, true, true, true],
		},
		{
			feature: 'Command palette (4 modes)',
			values: [true, false, false, false, false, false],
		},
		{
			feature: 'Multi-pane editing',
			values: [true, 'partial', true, false, false, false],
		},
		{
			feature: 'Scoped to a repo',
			values: [true, false, false, false, false, true],
		},
		{
			feature: 'AI review queue',
			values: [true, false, false, false, false, false],
		},
		{
			feature: 'Zero telemetry',
			values: [true, false, false, false, false, true],
		},
		{
			feature: 'No account required',
			values: [true, true, true, false, false, true],
		},
		{
			feature: 'Free and open source (PolyForm Shield 1.0.0)',
			values: [true, false, false, false, false, true],
		},
		{
			feature: 'Real-time collaboration',
			values: ['no \u2014 local-first; Git handles collaboration', false, false, true, true, false],
		},
		{
			feature: 'Plugin ecosystem',
			values: ['no \u2014 ships complete; extensions planned', true, true, true, false, true],
		},
		{
			feature: 'Mobile app',
			values: ['no \u2014 desktop workstation tool', true, false, true, true, false],
		},
	],
} as const;

export const workflow = {
	id: 'workflow',
	kicker: 'How it fits your workflow',
	title: 'it\u2019s just files. that\u2019s the whole point.',
	steps: [
		{
			n: '01',
			title: 'point vedox at a repo',
			body: 'run `vedox dev` in any folder that contains markdown. vedox scans, indexes, and opens a local editor. no config file required.',
		},
		{
			n: '02',
			title: 'edit in vedox',
			body: 'write, reorganize, link, and search across every doc from a fast local UI. nothing leaves your machine.',
		},
		{
			n: '03',
			title: 'commit like normal',
			body: '`git status`. `git diff`. `git commit`. the files on disk are the files vedox edited \u2014 no hidden state, no cache to flush, no export step.',
		},
	],
} as const;

export const faq = {
	id: 'faq',
	kicker: 'For the skeptics',
	title: 'fair questions.',
	items: [
		{
			q: 'Why not Obsidian?',
			a: 'Obsidian is a personal knowledge base optimized around a single vault. Vedox is a docs CMS scoped to a repo, Git-native, built for a team that already lives in pull requests. Different tool, different job. If you use Obsidian for personal notes and need something for project documentation \u2014 that\u2019s the gap.',
		},
		{
			q: 'Why not just write Markdown in VS Code?',
			a: 'You can. Many of us do. Vedox adds what editors don\u2019t: instant full-text search ranked by relevance across your entire doc corpus, a dual-mode WYSIWYG with round-trip fidelity, a frontmatter linter, document templates, and an AI review queue. If VS Code is enough, keep using it.',
		},
		{
			q: 'How does this compare to Confluence / Notion / GitBook?',
			a: 'Those are cloud services. Your content lives on their servers, in their format, behind their login. Vedox stores everything as Markdown files in a Git repo you own. No vendor lock-in. No subscription. No "exporting" your own work.',
		},
		{
			q: 'How does this compare to static site generators like Docusaurus?',
			a: 'Docusaurus builds a website from Markdown. Vedox is not a site generator \u2014 it is a local editor that reads and writes Markdown in your repo. If you need a published docs site, use Docusaurus to build it; use Vedox to write the content. They are complementary, not competing.',
		},
		{
			q: 'Can a team use this?',
			a: 'Yes. Everyone clones the repo, runs Vedox locally, edits docs, commits, and pushes. Collaboration happens through Git \u2014 pull requests, code review, merge conflicts, the same workflow your engineering team already uses. There\u2019s no real-time multiplayer cursor. There\u2019s `git pull`.',
		},
		{
			q: 'What does it cost?',
			a: 'Nothing. PolyForm Shield 1.0.0 licensed, free forever. No paid tier, no "pro" features behind a paywall. Pixelabs builds tools and releases them. This is one of them.',
		},
		{
			q: 'Does Vedox phone home?',
			a: 'No. Zero telemetry. Zero analytics in the binary. Zero outbound network calls to anywhere except localhost. The marketing site you\u2019re reading uses a privacy-respecting, cookieless analytics pixel \u2014 the product does not.',
		},
		{
			q: 'Can I use Vedox with my existing docs?',
			a: 'Yes. Run `vedox dev` in any folder that contains Markdown files. Vedox scans and indexes what is already there. No import wizard, no migration, no special file format. If your docs are Markdown in a Git repo, they are already Vedox-compatible.',
		},
	],
} as const;

export const roadmap = {
	id: 'roadmap',
	kicker: 'What\u2019s shipped, what\u2019s next',
	title: 'roadmap.',
	items: [
		{
			phase: 'Phase 1',
			status: 'shipped',
			title: 'Core editor and project shell',
			body: 'Tiptap dual-mode editor, project registry, CodeMirror source mode, dark-first design system, 5 themes, 3 density modes.',
		},
		{
			phase: 'Phase 2',
			status: 'shipped',
			title: 'Workspace scanner, import, and flagship UX',
			body: 'Point Vedox at any folder, detect frameworks, import existing docs, background indexing, multi-pane layout, command palette, task backlog.',
		},
		{
			phase: 'Phase 3',
			status: 'shipped',
			title: 'AI review queue and provider config',
			body: 'Agent-authed review queue for documentation changes. Provider config drawer for Claude Code, Codex, and Gemini CLI. Frontmatter linter with 16 rules.',
		},
	],
} as const;

export const waitlist = {
	id: 'waitlist',
	kicker: 'Ship day is coming',
	title: 'be the first to install on launch day.',
	body: 'One email, launch-day only. No newsletter, no drip, no sales sequence. Unsubscribe is a single click and we delete the address.',
	placeholder: 'you@yourdomain.dev',
	button: './install',
	success: 'Got it. We\u2019ll email once when Vedox is ready to install.',
	failure: 'Something went wrong. Try again or open an issue on GitHub.',
	disabled:
		'Waitlist endpoint not configured yet. Star the repo on GitHub and you\u2019ll see the launch there too.',
} as const;

export const footer = {
	copy: `\u00a9 ${new Date().getFullYear()} Pixelabs. PolyForm Shield 1.0.0 licensed.`,
	links: [
		{ label: 'GitHub', href: site.github },
		{ label: 'License', href: `${site.github}/blob/main/LICENSE` },
		{ label: 'Made with Vedox', href: site.github },
	],
} as const;

export const seo = {
	title: 'Vedox \u2014 Local-first documentation CMS for developers',
	description: site.description,
	ogImage: '/og.png',
	twitterHandle: '@thepixelabs',
	keywords: [
		'local-first documentation',
		'markdown CMS for developers',
		'self-hosted docs editor',
		'git-native docs',
		'open source documentation CMS',
		'WYSIWYG markdown editor',
	],
} as const;

export const jsonLd = {
	'@context': 'https://schema.org',
	'@type': 'SoftwareApplication',
	name: 'Vedox',
	description: site.description,
	applicationCategory: 'DeveloperApplication',
	operatingSystem: 'macOS, Linux, Windows',
	offers: { '@type': 'Offer', price: '0', priceCurrency: 'USD' },
	url: site.url,
	license: 'https://polyformproject.org/licenses/shield/1.0.0',
} as const;
