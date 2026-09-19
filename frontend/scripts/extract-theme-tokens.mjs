#!/usr/bin/env node
/**
 * Extracts every theme's root CSS custom properties and illustration style
 * into internal/themes/tokens.json, which internal/themes/themes.go embeds.
 *
 * For each theme, this reads the CSS custom properties declared directly on
 * the theme's root selector (`[data-theme='<slug>'] { ... }`) in its
 * theme.css - not properties set by other, more specific selectors further
 * down the file - and pairs them with the `illustration` object from that
 * theme's theme.json. `base` has no theme.json (see frontend/src/lib/themes/
 * loader.ts), so its illustration style is written here directly.
 *
 * Plain Node, no dependencies. Run with `npm run tokens` to write
 * internal/themes/tokens.json, or `npm run tokens:check` to verify the
 * committed file is still up to date without writing it.
 */

import { readFileSync, writeFileSync, readdirSync, existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const frontendRoot = path.resolve(__dirname, '..');
const repoRoot = path.resolve(frontendRoot, '..');
const themesDir = path.join(frontendRoot, 'src/lib/themes');
const outputPath = path.join(repoRoot, 'internal/themes/tokens.json');

const CHECK = process.argv.includes('--check');

/** Illustration style for `base`, which has no theme.json of its own. */
const BASE_ILLUSTRATION = {
	medium: 'flat vector on plain white, no texture, like a default slide template',
	line: '1.5 to 2px even strokes, no taper, rounded caps',
	shapes: 'simple rectangles, circles, and arrows, nothing ornamental',
	texture: 'none, completely flat fills',
	palette_use:
		'white background dominant, near-black ink, one blue accent used sparingly for the one thing that matters',
	mood: 'plain, neutral, unopinionated',
	avoid: 'gradients, 3D, photos, hand-drawn wobble, heavy shadows, any strong style'
};

/**
 * Finds every declaration block for `[data-theme='<slug>']` (or the
 * double-quoted spelling) as a standalone selector - not one combined with
 * a descendant class - and returns their raw declaration text joined in
 * document order, so a later block's declarations win over an earlier
 * block's for the same custom property, or null if no such block exists in
 * `css`.
 */
function findRootBlock(css, slug) {
	const selectorPattern = new RegExp(`\\[data-theme=(['"])${slug}\\1\\]\\s*\\{`, 'g');
	const blocks = [];
	let match;
	while ((match = selectorPattern.exec(css))) {
		const start = match.index + match[0].length;
		let depth = 1;
		let i = start;
		while (i < css.length && depth > 0) {
			if (css[i] === '{') depth++;
			else if (css[i] === '}') depth--;
			i++;
		}
		if (depth !== 0) continue;
		blocks.push(css.slice(start, i - 1));
		selectorPattern.lastIndex = i;
	}
	if (blocks.length === 0) return null;
	return blocks.join('\n');
}

/**
 * Parses every `--name: value;` custom property declared directly in a root
 * block (not inside a nested `@media` or similar), stripping trailing
 * inline comments from each value. Returns an ordered array of [name,
 * rawValue] pairs; when findRootBlock merged more than one block for the
 * same selector, a later declaration overwrites an earlier one for the
 * same name, matching the normal CSS cascade for same-specificity rules.
 */
function parseDeclarations(blockText) {
	// Strip block comments first so a comment never leaks into a value or
	// splits a declaration.
	const withoutComments = blockText.replace(/\/\*[\s\S]*?\*\//g, '');
	const declarationPattern = /--([\w-]+)\s*:\s*([^;]+);/g;
	const seen = new Map();
	let match;
	while ((match = declarationPattern.exec(withoutComments))) {
		const name = `--${match[1]}`;
		const value = match[2].trim();
		seen.set(name, value);
	}
	return seen;
}

/**
 * Resolves a `var(--other)` value one level deep against the same block's
 * declarations. A value that isn't a single var() reference, or whose
 * target isn't declared in the same block, is returned unchanged.
 */
function resolveOneLevel(value, declarations) {
	const varMatch = /^var\((--[\w-]+)\)$/.exec(value);
	if (!varMatch) return value;
	const target = declarations.get(varMatch[1]);
	return target !== undefined ? target : value;
}

/**
 * Every theme slug: `base` (which lives at themes/base.css, no folder) plus
 * every folder under src/lib/themes, sorted.
 */
function themeSlugs() {
	const folders = readdirSync(themesDir, { withFileTypes: true })
		.filter((entry) => entry.isDirectory())
		.map((entry) => entry.name);
	return [...folders, 'base'].sort();
}

function extractTheme(slug) {
	const isBase = slug === 'base';
	const cssPath = isBase ? path.join(themesDir, 'base.css') : path.join(themesDir, slug, 'theme.css');
	const css = readFileSync(cssPath, 'utf8');
	const block = findRootBlock(css, slug);
	if (block === null) {
		throw new Error(`extract-theme-tokens: no [data-theme='${slug}'] root block found in ${cssPath}`);
	}

	const declarations = parseDeclarations(block);
	const tokens = {};
	for (const [name, rawValue] of declarations) {
		tokens[name] = resolveOneLevel(rawValue, declarations);
	}

	let illustration;
	if (isBase) {
		illustration = BASE_ILLUSTRATION;
	} else {
		const jsonPath = path.join(themesDir, slug, 'theme.json');
		const definition = JSON.parse(readFileSync(jsonPath, 'utf8'));
		if (!definition.illustration) {
			throw new Error(`extract-theme-tokens: ${jsonPath} has no "illustration" object`);
		}
		illustration = definition.illustration;
	}

	return { tokens, illustration };
}

function sortedObject(obj) {
	const result = {};
	for (const key of Object.keys(obj).sort()) {
		result[key] = obj[key];
	}
	return result;
}

function buildOutput() {
	const output = {};
	for (const slug of themeSlugs()) {
		const { tokens, illustration } = extractTheme(slug);
		output[slug] = {
			tokens: sortedObject(tokens),
			illustration: sortedObject(illustration)
		};
	}
	return sortedObject(output);
}

function serialize(data) {
	return JSON.stringify(data, null, 2) + '\n';
}

function main() {
	const output = serialize(buildOutput());

	if (CHECK) {
		if (!existsSync(outputPath)) {
			console.error(`extract-theme-tokens --check: ${outputPath} does not exist. Run "npm run tokens".`);
			process.exit(1);
		}
		const onDisk = readFileSync(outputPath, 'utf8');
		if (onDisk !== output) {
			console.error(
				`extract-theme-tokens --check: ${outputPath} is stale. Run "npm run tokens" and commit the result.`
			);
			process.exit(1);
		}
		console.log('extract-theme-tokens --check: internal/themes/tokens.json is up to date.');
		return;
	}

	writeFileSync(outputPath, output);
	console.log(`extract-theme-tokens: wrote ${outputPath}`);
}

main();
