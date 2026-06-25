import { useCallback, useEffect, useRef, useState } from "react";
import { TASK_TIMINGS } from "@/constants/tasks";

type Options = {
  entityIds: readonly string[];
  onDelete: (id: string) => Promise<void>;
};

/** Exit animation + delete timing shared by drafts and templates list pages. */
export function useDeleteWithExitAnimation({ entityIds, onDelete }: Options) {
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [exitingIds, setExitingIds] = useState<string[]>([]);
  const deleteTimerRef = useRef<number | null>(null);

  const entityIdsKey = entityIds.join("\0");

  useEffect(() => {
    const ids = new Set(entityIds);
    setExitingIds((current) => {
      const next = current.filter((id) => ids.has(id));
      if (
        next.length === current.length &&
        next.every((id, index) => id === current[index])
      ) {
        return current;
      }
      return next;
    });
  }, [entityIdsKey]);

  useEffect(() => {
    return () => {
      if (deleteTimerRef.current !== null) {
        window.clearTimeout(deleteTimerRef.current);
      }
    };
  }, []);

  const deleteWithExit = useCallback(
    async (id: string) => {
      setDeletingId(id);
      setExitingIds((current) =>
        current.includes(id) ? current : [...current, id],
      );
      await new Promise<void>((resolve) => {
        deleteTimerRef.current = window.setTimeout(() => {
          deleteTimerRef.current = null;
          resolve();
        }, TASK_TIMINGS.draftDeleteExitMs);
      });
      try {
        await onDelete(id);
      } catch {
        setExitingIds((current) => current.filter((item) => item !== id));
      } finally {
        setDeletingId((current) => (current === id ? null : current));
      }
    },
    [onDelete],
  );

  return { deletingId, exitingIds, deleteWithExit };
}

export function isSavedEntityRowActionExcluded(target: EventTarget | null): boolean {
  if (!(target instanceof Element)) return true;
  return Boolean(target.closest("button"));
}
