import { afterEach, describe, expect, it } from 'vitest';

import { DatabaseType } from './DatabaseType';
import { getDatabaseLogoFromType } from './getDatabaseLogoFromType';

describe('getDatabaseLogoFromType', () => {
  afterEach(() => {
    delete (globalThis as { window?: unknown }).window;
  });

  it('returns a root-absolute path when no base is injected', () => {
    expect(getDatabaseLogoFromType(DatabaseType.POSTGRES)).toBe('/icons/databases/postgresql.svg');
  });

  it('prefixes the logo path with the injected base', () => {
    (globalThis as { window?: unknown }).window = {
      __DATABASUS_BASE__: '/k8s/clusters/c-x/proxy/databasus',
    };
    expect(getDatabaseLogoFromType(DatabaseType.POSTGRES)).toBe(
      '/k8s/clusters/c-x/proxy/databasus/icons/databases/postgresql.svg',
    );
  });
});
