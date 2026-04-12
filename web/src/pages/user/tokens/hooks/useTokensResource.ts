import { useCallback, useEffect, useState } from 'react';

import { services } from '@/services';
import { openSnackbar } from '@/store/slices/snackbar';
import { useDispatch } from '@/store';
import type { AccessToken } from '@/types/token';

export interface UseTokensResourceResult {
  tokens: AccessToken[];
  loading: boolean;
  error: string | undefined;
  refresh: () => Promise<void>;

  editOpen: boolean;
  editTarget: AccessToken | null;
  editLoading: boolean;
  editError: string | undefined;
  openEdit: (token: AccessToken) => void;
  closeEdit: () => void;
  submitEdit: (values: { tokenId: string; name: string; scopes: string[] }) => Promise<void>;

  revokeOpen: boolean;
  revokeTarget: AccessToken | null;
  revokeLoading: boolean;
  openRevoke: (token: AccessToken) => void;
  closeRevoke: () => void;
  confirmRevoke: () => Promise<void>;
}

export function useTokensResource(): UseTokensResourceResult {
  const dispatch = useDispatch();

  const [tokens, setTokens] = useState<AccessToken[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();

  const [editOpen, setEditOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<AccessToken | null>(null);
  const [editLoading, setEditLoading] = useState(false);
  const [editError, setEditError] = useState<string>();

  const [revokeOpen, setRevokeOpen] = useState(false);
  const [revokeTarget, setRevokeTarget] = useState<AccessToken | null>(null);
  const [revokeLoading, setRevokeLoading] = useState(false);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    try {
      const result = await services.token.list();
      setTokens(result.tokens);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const openEdit = useCallback((token: AccessToken) => {
    setEditTarget(token);
    setEditError(undefined);
    setEditOpen(true);
  }, []);

  const closeEdit = useCallback(() => {
    if (!editLoading) setEditOpen(false);
  }, [editLoading]);

  const submitEdit = useCallback(
    async (values: { tokenId: string; name: string; scopes: string[] }) => {
      setEditLoading(true);
      setEditError(undefined);
      try {
        await services.token.update(values);
        dispatch(
          openSnackbar({
            variant: 'alert',
            message: 'Token updated',
            alert: { color: 'success', variant: 'filled' },
          }),
        );
        setEditOpen(false);
        void refresh();
      } catch (err) {
        setEditError(err instanceof Error ? err.message : String(err));
      } finally {
        setEditLoading(false);
      }
    },
    [dispatch, refresh],
  );

  const openRevoke = useCallback((token: AccessToken) => {
    setRevokeTarget(token);
    setRevokeOpen(true);
  }, []);

  const closeRevoke = useCallback(() => {
    if (!revokeLoading) setRevokeOpen(false);
  }, [revokeLoading]);

  const confirmRevoke = useCallback(async () => {
    if (!revokeTarget) return;
    setRevokeLoading(true);
    try {
      await services.token.revoke(revokeTarget.id);
      dispatch(
        openSnackbar({
          variant: 'alert',
          message: 'Token revoked',
          alert: { color: 'success', variant: 'filled' },
        }),
      );
      setRevokeOpen(false);
      void refresh();
    } catch (err) {
      dispatch(
        openSnackbar({
          variant: 'alert',
          message: err instanceof Error ? err.message : String(err),
          alert: { color: 'error', variant: 'filled' },
        }),
      );
    } finally {
      setRevokeLoading(false);
    }
  }, [dispatch, refresh, revokeTarget]);

  return {
    tokens,
    loading,
    error,
    refresh,
    editOpen,
    editTarget,
    editLoading,
    editError,
    openEdit,
    closeEdit,
    submitEdit,
    revokeOpen,
    revokeTarget,
    revokeLoading,
    openRevoke,
    closeRevoke,
    confirmRevoke,
  };
}