import { useCallback, useEffect, useMemo, useState } from 'react';

import { services } from '@/services';
import type { Application } from '@/types/application';

export type ApplicationsSortField = 'name' | 'created_at' | 'updated_at';
export type SortDir = 'asc' | 'desc';

export interface UseApplicationsResourceResult {
  rows: Application[];
  applications: Application[];
  loading: boolean;
  error: string | undefined;
  /** Set when apps load but environment list fails (non-fatal). */
  warning: string | undefined;
  sortField: ApplicationsSortField;
  sortDir: SortDir;
  setSortField: (field: ApplicationsSortField) => void;
  setSortDir: (dir: SortDir) => void;
  /** Table headers: toggle direction when clicking the same column. */
  requestSort: (field: ApplicationsSortField) => void;
  refresh: () => Promise<void>;
  /** False until `/api/v1/environments` list succeeds (needed for safe delete rules). */
  environmentCountsLoaded: boolean;
  canDelete: (applicationId: string) => boolean;
  environmentCount: (applicationId: string) => number | undefined;
  /** Environment names for this app (from list env API). Undefined if env list not loaded. */
  environmentNames: (applicationId: string) => string[] | undefined;
  create: (input: {
    name: string;
    description?: string;
    labels?: Record<string, string>;
  }) => Promise<Application>;
  update: (input: {
    id: string;
    name: string;
    updateName?: boolean;
    description?: string;
    labels?: Record<string, string>;
  }) => Promise<Application>;
  remove: (id: string) => Promise<void>;
}

function compareStrings(a: string | undefined, b: string | undefined, dir: SortDir): number {
  const av = a ?? '';
  const bv = b ?? '';
  const cmp = av.localeCompare(bv, undefined, { sensitivity: 'base' });
  return dir === 'asc' ? cmp : -cmp;
}

export function useApplicationsResource(): UseApplicationsResourceResult {
  const [applications, setApplications] = useState<Application[]>([]);
  const [envCountByAppId, setEnvCountByAppId] = useState<Map<string, number>>(new Map());
  const [envNamesByAppId, setEnvNamesByAppId] = useState<Map<string, string[]>>(new Map());
  const [environmentCountsLoaded, setEnvironmentCountsLoaded] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string>();
  const [warning, setWarning] = useState<string>();
  const [sortField, setSortField] = useState<ApplicationsSortField>('name');
  const [sortDir, setSortDir] = useState<SortDir>('asc');

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    setWarning(undefined);
    setEnvironmentCountsLoaded(false);
    try {
      const apps = await services.application.listAll();
      setApplications(apps);

      // Foo only hits /api/v1/applications; we need env counts for delete rules. If this
      // call fails (scope, partial deploy), still show the app list and warn.
      try {
        const envs = await services.environment.listAll();
        setEnvCountByAppId(services.environment.countByApplicationId(envs));
        setEnvNamesByAppId(services.environment.environmentNamesByApplicationId(envs));
        setEnvironmentCountsLoaded(true);
      } catch (envErr) {
        setEnvCountByAppId(new Map());
        setEnvNamesByAppId(new Map());
        setEnvironmentCountsLoaded(false);
        const msg = envErr instanceof Error ? envErr.message : String(envErr);
        setWarning(
          `Environments could not be loaded (${msg}). Delete is disabled until this succeeds. Use Refresh to retry.`,
        );
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const setSortFieldState = useCallback((field: ApplicationsSortField) => {
    setSortField(field);
  }, []);

  const setSortDirState = useCallback((dir: SortDir) => {
    setSortDir(dir);
  }, []);

  const requestSort = useCallback((field: ApplicationsSortField) => {
    setSortField((prev) => {
      if (prev === field) {
        setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
        return prev;
      }
      setSortDir('asc');
      return field;
    });
  }, []);

  const rows = useMemo(() => {
    const sorted = [...applications].sort((a, b) => {
      switch (sortField) {
        case 'name':
          return compareStrings(a.name, b.name, sortDir);
        case 'created_at':
          return compareStrings(a.created_at, b.created_at, sortDir);
        case 'updated_at':
          return compareStrings(a.updated_at, b.updated_at, sortDir);
        default:
          return 0;
      }
    });
    return sorted;
  }, [applications, sortField, sortDir]);

  const canDelete = useCallback(
    (applicationId: string) =>
      environmentCountsLoaded && (envCountByAppId.get(applicationId) ?? 0) === 0,
    [envCountByAppId, environmentCountsLoaded],
  );

  const environmentCount = useCallback(
    (applicationId: string) =>
      environmentCountsLoaded ? (envCountByAppId.get(applicationId) ?? 0) : undefined,
    [envCountByAppId, environmentCountsLoaded],
  );

  const environmentNames = useCallback(
    (applicationId: string) =>
      environmentCountsLoaded ? (envNamesByAppId.get(applicationId) ?? []) : undefined,
    [envNamesByAppId, environmentCountsLoaded],
  );

  const create = useCallback(
    async (input: {
      name: string;
      description?: string;
      labels?: Record<string, string>;
    }) => {
      const created = await services.application.create(input);
      setApplications((prev) => [...prev, created]);
      return created;
    },
    [],
  );

  const update = useCallback(
    async (input: {
      id: string;
      name: string;
      updateName?: boolean;
      description?: string;
      labels?: Record<string, string>;
    }) => {
      const updated = await services.application.update(input);
      setApplications((prev) => prev.map((a) => (a.id === updated.id ? updated : a)));
      return updated;
    },
    [],
  );

  const remove = useCallback(async (id: string) => {
    await services.application.remove(id);
    setApplications((prev) => prev.filter((a) => a.id !== id));
    setEnvCountByAppId((prev) => {
      const next = new Map(prev);
      next.delete(id);
      return next;
    });
    setEnvNamesByAppId((prev) => {
      const next = new Map(prev);
      next.delete(id);
      return next;
    });
  }, []);

  return {
    rows,
    applications,
    environmentCountsLoaded,
    loading,
    error,
    warning,
    sortField,
    sortDir,
    setSortField: setSortFieldState,
    setSortDir: setSortDirState,
    requestSort,
    refresh,
    canDelete,
    environmentCount,
    environmentNames,
    create,
    update,
    remove,
  };
}
