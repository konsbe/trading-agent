import {
  clearAllMfeCaches,
  clearMfeCachesForScope,
  setCachedModulePromise,
  getCachedModulePromise,
} from './mfeCache';

describe('dynamic_load/mfeCache', () => {
  afterEach(() => {
    document.head.innerHTML = '';
    (window as any).mfeTest = undefined;
    (window as any).webpackChunkmfeTest = undefined;
    clearAllMfeCaches();
  });

  it('clears cached module promises', () => {
    setCachedModulePromise('k', Promise.resolve(123));
    expect(getCachedModulePromise('k')).toBeDefined();

    clearAllMfeCaches();
    expect(getCachedModulePromise('k')).toBeUndefined();
  });

  it('removes remoteEntry script tags, containers, and webpack chunk globals', () => {
    const script = document.createElement('script');
    script.setAttribute('data-mfe-scope', 'mfeTest');
    document.head.appendChild(script);

    (window as any).mfeTest = { marker: 'container' };
    (window as any).webpackChunkmfeTest = [{ marker: 'chunk' }];

    expect(document.querySelectorAll('script[data-mfe-scope]').length).toBe(1);

    clearAllMfeCaches();

    expect(document.querySelectorAll('script[data-mfe-scope]').length).toBe(0);
    expect((window as any).mfeTest).toBeUndefined();
    expect((window as any).webpackChunkmfeTest).toBeUndefined();
  });

  it('clears caches only for the specified scope', () => {
    // two cached module promises in different scopes
    setCachedModulePromise('mfeA::./X', Promise.resolve('a'));
    setCachedModulePromise('mfeB::./Y', Promise.resolve('b'));

    const scriptA = document.createElement('script');
    scriptA.setAttribute('data-mfe-scope', 'mfeA');
    document.head.appendChild(scriptA);

    const scriptB = document.createElement('script');
    scriptB.setAttribute('data-mfe-scope', 'mfeB');
    document.head.appendChild(scriptB);

    (window as any).mfeA = {};
    (window as any).mfeB = {};
    (window as any).webpackChunkmfeA = [];
    (window as any).webpackChunkmfeB = [];

    clearMfeCachesForScope('mfeA');

    expect(getCachedModulePromise('mfeA::./X')).toBeUndefined();
    expect(getCachedModulePromise('mfeB::./Y')).toBeDefined();

    expect(document.querySelectorAll('script[data-mfe-scope="mfeA"]').length).toBe(0);
    expect(document.querySelectorAll('script[data-mfe-scope="mfeB"]').length).toBe(1);

    expect((window as any).mfeA).toBeUndefined();
    expect((window as any).webpackChunkmfeA).toBeUndefined();

    expect((window as any).mfeB).toBeDefined();
    expect((window as any).webpackChunkmfeB).toBeDefined();
  });
});
