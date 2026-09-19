import { describe, expect, it } from 'vitest';
import { layoutRegistry, resolveLayout } from './registry';
import layoutSlots from '../../../../internal/layouts/layouts.json';

describe('layout registry', () => {
	it('registers every layout declared in layouts.json with matching slots', () => {
		for (const [name, slots] of Object.entries(layoutSlots)) {
			const definition = layoutRegistry.get(name);
			expect(definition).toBeDefined();
			expect(definition?.slots).toEqual(slots);
			expect(typeof definition?.component).toBe('function');
		}
	});

	it('has exactly the layouts declared in layouts.json, plus "component"', () => {
		expect(new Set(layoutRegistry.keys())).toEqual(new Set([...Object.keys(layoutSlots), 'component']));
	});

	it('registers "component" with an empty slot list: a whole-slide component owns all slot content itself', () => {
		const definition = layoutRegistry.get('component');
		expect(definition).toBeDefined();
		expect(definition?.slots).toEqual([]);
		expect(typeof definition?.component).toBe('function');
	});

	it('falls back to the default layout for an unknown name', () => {
		expect(resolveLayout('nope')).toBe(layoutRegistry.get('default'));
	});

	it('resolves a known layout to its own definition', () => {
		expect(resolveLayout('big-stat')).toBe(layoutRegistry.get('big-stat'));
	});
});
