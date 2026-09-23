import React, { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";
import { useMfeOperatorConfigSse } from "../../common/sse/hooks/useMfeOperatorConfigSse";
import { clearMfeCachesForScope } from "../../common/dynamic_load";

type MfeReloadContextValue = {
  globalToken: number;
  perMfeToken: Record<string, number>;
  registerActiveMfe: (mfeKey: string) => void;
  unregisterActiveMfe: (mfeKey: string) => void;
};

const noop = () => undefined;

export const MfeReloadContext = createContext<MfeReloadContextValue>({
  globalToken: 0,
  perMfeToken: {},
  registerActiveMfe: noop,
  unregisterActiveMfe: noop,
});

export const useMfeReloadToken = (mfeKey?: string): number => {
  const ctx = useContext(MfeReloadContext);
  if (mfeKey) {
    return ctx.perMfeToken[mfeKey] ?? 0;
  }
  return ctx.globalToken;
};

export const useMfeActivity = () => {
  const ctx = useContext(MfeReloadContext);
  return useMemo(
    () => ({ registerActiveMfe: ctx.registerActiveMfe, unregisterActiveMfe: ctx.unregisterActiveMfe }),
    [ctx.registerActiveMfe, ctx.unregisterActiveMfe]
  );
};

const fetchConfig = async (): Promise<AppConfig> => {
  // [SSE] mfeMetadataUpdate event received

  const res = await fetch("/config.json", { cache: "no-store" });
  if (!res.ok) {
    throw new Error(`Failed to fetch config.json: ${res.status} ${res.statusText}`);
  }

  const config: AppConfig = await res.json();
  (window as any).__APP_CONFIG__ = config;
  return config;
};

const getUpdatedMfeKeys = (prevCfg: AppConfig | undefined, nextCfg: AppConfig): string[] => {
  const prevMfes = prevCfg?.mfes ?? {};
  const nextMfes = nextCfg.mfes ?? {};

  const updated: string[] = Object.keys(nextMfes).filter(mfeKey => {
    const prev = prevMfes[mfeKey];
    const next = nextMfes[mfeKey];
    return !prev || String(prev.version ?? '') !== String(next.version ?? '');
  });

  return updated;
};

const SseMetadataProvider = ({ children }: { children: React.ReactNode }) => {
  const { event } = useMfeOperatorConfigSse();
  const [globalToken, setGlobalToken] = useState(0);
  const [perMfeToken, setPerMfeToken] = useState<Record<string, number>>({});

  const configRef = useRef<AppConfig | undefined>((window as any).__APP_CONFIG__);
  const activeCountsRef = useRef<Map<string, number>>(new Map());
  const pendingReloadRef = useRef<Set<string>>(new Set());

  const isActive = useCallback((mfeKey: string): boolean => {
    return (activeCountsRef.current.get(mfeKey) ?? 0) > 0;
  }, []);

  const scheduleReloadForMfeKey = useCallback((mfeKey: string) => {
    const cfg = configRef.current;
    const entry = cfg?.mfes?.[mfeKey];
    const scope = entry?.scope ?? mfeKey;
    clearMfeCachesForScope(scope);

    setPerMfeToken((prev) => ({ ...prev, [mfeKey]: (prev[mfeKey] ?? 0) + 1 }));
  }, []);

  const registerActiveMfe = useCallback((mfeKey: string) => {
    const prev = activeCountsRef.current.get(mfeKey) ?? 0;
    activeCountsRef.current.set(mfeKey, prev + 1);
  }, []);

  const unregisterActiveMfe = useCallback((mfeKey: string) => {
    const prev = activeCountsRef.current.get(mfeKey) ?? 0;
    const next = Math.max(0, prev - 1);
    if (next === 0) {
      activeCountsRef.current.delete(mfeKey);
    } else {
      activeCountsRef.current.set(mfeKey, next);
    }

    // If this MFE was updated while active, reload it only after it becomes inactive.
    if (next === 0 && pendingReloadRef.current.has(mfeKey)) {
      pendingReloadRef.current.delete(mfeKey);
      scheduleReloadForMfeKey(mfeKey);
    }
  }, [scheduleReloadForMfeKey]);

  useEffect(() => {
    if (!event) return;

    (async () => {
      const prevCfg = configRef.current;
      const nextCfg = await fetchConfig();
      configRef.current = nextCfg;

      // Always bump global token so non-MFE UI can react to config changes.
      setGlobalToken((t: number) => t + 1);

      const updatedKeys = getUpdatedMfeKeys(prevCfg, nextCfg);

      updatedKeys.forEach(mfeKey => {
        if (isActive(mfeKey)) {
          // Don't disrupt the user; reload later after they leave.
          pendingReloadRef.current.add(mfeKey);
        } else {
          scheduleReloadForMfeKey(mfeKey);
        }
      });
    })().catch((err) => console.error("[SSE] failed to refresh MFE config", err));
  }, [event]);

  const ctxValue = useMemo<MfeReloadContextValue>(
    () => ({ globalToken, perMfeToken, registerActiveMfe, unregisterActiveMfe }),
    [globalToken, perMfeToken, registerActiveMfe, unregisterActiveMfe]
  );

  return <MfeReloadContext.Provider value={ctxValue}>{children}</MfeReloadContext.Provider>;
};

export default SseMetadataProvider;