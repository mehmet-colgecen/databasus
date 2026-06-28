declare global {
  interface Window {
    __DATABASUS_BASE__?: string;
  }
}

/**
 * The path prefix the app is mounted under, injected by the reverse proxy as
 * window.__DATABASUS_BASE__, or '' when served at the web root (standalone).
 */
export function getBasePath(): string {
  return (typeof window !== 'undefined' && window.__DATABASUS_BASE__) || '';
}

/** Prefix a root-absolute asset path (e.g. '/icons/x.svg') with the mount base. */
export function asset(path: string): string {
  return getBasePath() + path;
}
