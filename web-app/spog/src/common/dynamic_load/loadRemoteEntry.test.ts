import { loadRemoteEntry } from './loadRemoteEntry';

describe('dynamic_load/loadRemoteEntry', () => {
  beforeEach(() => {
    document.head.innerHTML = '';
  });

  afterEach(() => {
    jest.restoreAllMocks();
    // cleanup any container we created
    (window as any).mfeTest = undefined;
  });

  it('resolves immediately when a valid MF container is already on window', async () => {
    (window as any).mfeTest = { get: jest.fn(), init: jest.fn() };
    await expect(loadRemoteEntry('http://example/remoteEntry.js', 'mfeTest')).resolves.toBeUndefined();
  });

  it('does not short-circuit when window[scope] is truthy but not a valid container', async () => {
    // A plain object without .get (e.g. webpack chunk array) must NOT be treated as a ready container
    (window as any).mfeTest = []; // chunk-push array
    const appendSpy = jest.spyOn(document.head, 'appendChild');
    const promise = loadRemoteEntry('http://example/remoteEntry.js', 'mfeTest');
    // Should have appended a new script tag instead of short-circuiting
    expect(appendSpy).toHaveBeenCalledTimes(1);
    const script = appendSpy.mock.calls[0][0] as HTMLScriptElement;
    (script as any).onload();
    await expect(promise).resolves.toBeUndefined();
  });

  it('resolves immediately when a matching script already exists and container is a valid MF container', async () => {
    const existing = document.createElement('script');
    existing.setAttribute('data-mfe-scope', 'mfeTest');
    document.head.appendChild(existing);
    (window as any).mfeTest = { get: jest.fn(), init: jest.fn() };

    await expect(loadRemoteEntry('http://example/remoteEntry.js', 'mfeTest')).resolves.toBeUndefined();
  });

  it('waits for load event when script exists but container not yet registered', async () => {
    const existing = document.createElement('script');
    existing.setAttribute('data-mfe-scope', 'mfeTest');
    document.head.appendChild(existing);

    const promise = loadRemoteEntry('http://example/remoteEntry.js', 'mfeTest');

    // Simulate the container registering itself, then fire the load event
    (window as any).mfeTest = {};
    existing.dispatchEvent(new Event('load'));

    await expect(promise).resolves.toBeUndefined();
  });

  it('appends a script tag and resolves on load when container appears', async () => {
    const appendSpy = jest.spyOn(document.head, 'appendChild');

    const promise = loadRemoteEntry('http://example/remoteEntry.js', 'mfeTest');

    // The code should have created and appended a script
    expect(appendSpy).toHaveBeenCalledTimes(1);
    const script = appendSpy.mock.calls[0][0] as HTMLScriptElement;
    expect(script.tagName).toBe('SCRIPT');
    expect(script.getAttribute('data-mfe-scope')).toBe('mfeTest');
    expect(script.src).toBe('http://example/remoteEntry.js');

    // Simulate remote attaching itself to window, then script load
    (window as any).mfeTest = {};
    (script as any).onload();

    await expect(promise).resolves.toBeUndefined();
  });

  it('resolves on script load even when container is not yet set (polling is deferred to loadMfeComponent)', async () => {
    // loadRemoteEntry no longer validates the container — loadMfeComponent's polling handles that.
    // This allows async-initializing (MF v2) remotes to work correctly.
    const appendSpy = jest.spyOn(document.head, 'appendChild');

    const promise = loadRemoteEntry('http://example/remoteEntry.js', 'mfeTest');
    const script = appendSpy.mock.calls[0][0] as HTMLScriptElement;

    // Fire load without setting window.mfeTest — should still resolve
    (script as any).onload();

    await expect(promise).resolves.toBeUndefined();
  });

  it('rejects on script error', async () => {
    const appendSpy = jest.spyOn(document.head, 'appendChild');

    const promise = loadRemoteEntry('http://example/remoteEntry.js', 'mfeTest');
    const script = appendSpy.mock.calls[0][0] as HTMLScriptElement;
    document.head.appendChild(script); // ensure it's in DOM so removal can be verified

    (script as any).onerror(new Event('error'));

    await expect(promise).rejects.toThrow(
      'Failed to load remote mfeTest from http://example/remoteEntry.js'
    );
    expect(document.querySelector('script[data-mfe-scope="mfeTest"]')).toBeNull();
  });

  it('allows retry after script error by not leaving a stale script in DOM', async () => {
    const appendSpy = jest.spyOn(document.head, 'appendChild');

    const promise = loadRemoteEntry('http://example/remoteEntry.js', 'mfeTest');
    const script = appendSpy.mock.calls[0][0] as HTMLScriptElement;
    document.head.appendChild(script);

    (script as any).onerror(new Event('error'));
    await expect(promise).rejects.toThrow();

    // Second attempt should append a NEW script (not hang on the stale one)
    appendSpy.mockClear();
    const promise2 = loadRemoteEntry('http://example/remoteEntry.js', 'mfeTest');
    expect(appendSpy).toHaveBeenCalledTimes(1);
    const script2 = appendSpy.mock.calls[0][0] as HTMLScriptElement;
    (script2 as any).onload();
    await expect(promise2).resolves.toBeUndefined();
  });
});
