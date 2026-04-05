import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useReducer,
  useRef,
  type MutableRefObject,
  type ReactNode,
} from "react";

/** Latest escape target for one open modal; read `.current` when handling Escape. */
export type ModalEscapeRef = MutableRefObject<{
  busy: boolean;
  onClose: () => void;
}>;

type ModalStackApi = {
  register: (ref: ModalEscapeRef) => void;
  unregister: (ref: ModalEscapeRef) => void;
};

const ModalStackContext = createContext<ModalStackApi | null>(null);

export function useModalStackOptional(): ModalStackApi | null {
  return useContext(ModalStackContext);
}

/**
 * Wraps the app (or a subtree) so stacked {@link Modal}s only close the top layer on Escape.
 * While any modal is open, sets `inert` on `#root` so background UI is not focusable or
 * interactive (modals portal to `document.body`, outside `#root`).
 */
export function ModalStackProvider({ children }: { children: ReactNode }) {
  const stackRef = useRef<ModalEscapeRef[]>([]);
  const [stackTick, bumpStack] = useReducer((n: number) => n + 1, 0);

  const api = useMemo(
    () => ({
      register(ref: ModalEscapeRef) {
        stackRef.current.push(ref);
        bumpStack();
      },
      unregister(ref: ModalEscapeRef) {
        const i = stackRef.current.lastIndexOf(ref);
        if (i >= 0) stackRef.current.splice(i, 1);
        bumpStack();
      },
    }),
    [],
  );

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "Escape") return;
      const stack = stackRef.current;
      if (stack.length === 0) return;
      const top = stack[stack.length - 1]!.current;
      if (top.busy) return;
      e.preventDefault();
      e.stopPropagation();
      top.onClose();
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, []);

  useEffect(() => {
    const el = document.getElementById("root");
    if (!el) return;
    if (stackRef.current.length > 0) {
      el.setAttribute("inert", "");
    } else {
      el.removeAttribute("inert");
    }
    return () => {
      el.removeAttribute("inert");
    };
  }, [stackTick]);

  return (
    <ModalStackContext.Provider value={api}>{children}</ModalStackContext.Provider>
  );
}
