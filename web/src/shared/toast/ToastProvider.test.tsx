import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider, useOptionalToast, useToast } from "./ToastProvider";

function Harness({ onMount }: { onMount: (api: ReturnType<typeof useToast>) => void }) {
  const api = useToast();
  onMount(api);
  return <div data-testid="harness" />;
}

function renderWithToast(onMount: (api: ReturnType<typeof useToast>) => void) {
  return render(
    <ToastProvider>
      <Harness onMount={onMount} />
    </ToastProvider>,
  );
}

describe("ToastProvider", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    if (vi.isFakeTimers()) {
      act(() => {
        vi.runOnlyPendingTimers();
      });
      vi.useRealTimers();
    }
  });

  // The minimum viable contract: pushing an error toast surfaces a
  // status row with role="status" so screen readers announce it
  // politely. A regression here would silently break the
  // accessibility guarantee documented on the provider.
  it("renders error toasts with role=status and the message text", () => {
    let api!: ReturnType<typeof useToast>;
    renderWithToast((a) => {
      api = a;
    });
    act(() => {
      api.error("Couldn't save - reverted.");
    });
    const toast = screen.getByRole("status");
    expect(toast).toHaveTextContent("Couldn't save - reverted.");
    expect(toast.className).toContain("toast--error");
  });

  // Auto-dismiss after AUTO_DISMISS_MS is what keeps the surface
  // ephemeral; without this pin a refactor that flipped the duration
  // to 0 (or removed the timer entirely) would leave toasts piling
  // up forever.
  it("auto-dismisses each toast after 4 seconds", () => {
    let api!: ReturnType<typeof useToast>;
    renderWithToast((a) => {
      api = a;
    });
    act(() => {
      api.info("just-a-blip");
    });
    expect(screen.getByRole("status")).toBeInTheDocument();
    act(() => {
      vi.advanceTimersByTime(4_000);
    });
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });

  // Dedupe within the 5s window prevents a systemic failure (e.g.
  // every PATCH returning 500) from flooding the viewport with
  // identical toasts. We assert push 5x = render 1x.
  it("dedupes identical (kind,message) within the dedupe window", () => {
    let api!: ReturnType<typeof useToast>;
    renderWithToast((a) => {
      api = a;
    });
    act(() => {
      for (let i = 0; i < 5; i++) {
        api.error("Couldn't save - reverted.");
      }
    });
    expect(screen.getAllByRole("status")).toHaveLength(1);
  });

  // The MAX_TOASTS=3 cap keeps the stack small. We push four
  // *distinct* messages (so dedupe doesn't kick in) and assert only
  // the most recent three remain — preserving the most-recent
  // bias documented on the provider.
  it("caps the stack at 3 toasts, dropping the oldest", () => {
    let api!: ReturnType<typeof useToast>;
    renderWithToast((a) => {
      api = a;
    });
    act(() => {
      api.error("first");
      api.error("second");
      api.error("third");
      api.error("fourth");
    });
    const items = screen.getAllByRole("status");
    expect(items).toHaveLength(3);
    expect(items[0]).toHaveTextContent("second");
    expect(items[2]).toHaveTextContent("fourth");
  });

  // The dismiss button is the keyboard escape hatch when the
  // auto-dismiss is too slow (or disabled by reduced-motion users
  // pausing). Without this users would have to wait the full 4s
  // even when they explicitly want the surface gone.
  it("clicking the close button dismisses the toast immediately", async () => {
    vi.useRealTimers();
    let api!: ReturnType<typeof useToast>;
    renderWithToast((a) => {
      api = a;
    });
    act(() => {
      api.info("dismiss me");
    });
    const close = screen.getByRole("button", { name: /dismiss notification/i });
    await userEvent.click(close);
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });

  // useToast outside a provider must throw — a missed mount is a
  // bug we want to catch loudly during development, not have it
  // silently swallow every rollback notification in production.
  it("useToast throws a descriptive error when no provider is mounted", () => {
    function Bare() {
      useToast();
      return null;
    }
    const errSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    expect(() => render(<Bare />)).toThrow(/ToastProvider/);
    errSpy.mockRestore();
  });

  // useOptionalToast is the escape hatch for hooks that fire during
  // boot or in tests that skip the provider. The contract is "no
  // provider == no-op API"; without this pin, switching to the
  // strict useToast in those callers would crash early-boot paths.
  it("useOptionalToast returns a no-op API when no provider is mounted", () => {
    let captured: ReturnType<typeof useOptionalToast> | null = null;
    function Probe() {
      captured = useOptionalToast();
      return null;
    }
    render(<Probe />);
    expect(captured).not.toBeNull();
    expect(() => {
      captured!.error("ignored");
      captured!.success("ignored");
      captured!.dismiss(0);
    }).not.toThrow();
    expect(captured!.toasts).toEqual([]);
  });
});
