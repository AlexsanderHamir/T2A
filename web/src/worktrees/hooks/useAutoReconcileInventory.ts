import { useEffect, useRef } from "react";

type Options = {
  enabled: boolean;
  inventoryUnreachable: boolean;
  reconcilePending: boolean;
  reconcileBlocked: boolean;
  onReconcile: () => void;
};

/** Fire reconcile at most once per modal-open session when live inventory is unreachable. */
export function useAutoReconcileInventory({
  enabled,
  inventoryUnreachable,
  reconcilePending,
  reconcileBlocked,
  onReconcile,
}: Options): void {
  const attemptedRef = useRef(false);

  useEffect(() => {
    if (!enabled) {
      attemptedRef.current = false;
      return;
    }
    if (
      !inventoryUnreachable ||
      reconcilePending ||
      reconcileBlocked ||
      attemptedRef.current
    ) {
      return;
    }
    attemptedRef.current = true;
    onReconcile();
  }, [
    enabled,
    inventoryUnreachable,
    reconcilePending,
    reconcileBlocked,
    onReconcile,
  ]);
}
