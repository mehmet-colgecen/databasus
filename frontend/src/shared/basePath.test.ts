import { afterEach, describe, expect, it } from 'vitest';

import { asset, getBasePath } from './basePath';

describe('basePath', () => {
  afterEach(() => {
    delete (globalThis as { window?: unknown }).window;
  });

  it('returns empty string when no base is injected', () => {
    expect(getBasePath()).toBe('');
    expect(asset('/icons/x.svg')).toBe('/icons/x.svg');
  });

  it('returns the injected proxy prefix', () => {
    (globalThis as { window?: unknown }).window = {
      __DATABASUS_BASE__: '/k8s/clusters/c-x/proxy/databasus',
    };
    expect(getBasePath()).toBe('/k8s/clusters/c-x/proxy/databasus');
  });

  it('asset() prefixes root-absolute paths with the base', () => {
    (globalThis as { window?: unknown }).window = {
      __DATABASUS_BASE__: '/k8s/clusters/c-x/proxy/databasus',
    };
    expect(asset('/icons/x.svg')).toBe('/k8s/clusters/c-x/proxy/databasus/icons/x.svg');
  });
});
