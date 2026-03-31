import { ReactNode, useEffect, useRef } from "react";
import { createPortal } from "react-dom";

import { cn } from "@/lib/utils";

const FOCUSABLE_SELECTOR = [
  'a[href]',
  'area[href]',
  'input:not([disabled]):not([type="hidden"])',
  "select:not([disabled])",
  "textarea:not([disabled])",
  "button:not([disabled])",
  "iframe",
  '[tabindex]:not([tabindex="-1"])',
  '[contenteditable="true"]',
].join(",");

function getFocusableElements(container: HTMLElement | null): HTMLElement[] {
  if (!container) return [];
  return Array.from(container.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)).filter((element) => {
    if (element.hasAttribute("disabled")) return false;
    if (element.getAttribute("aria-hidden") === "true") return false;
    return element.offsetParent !== null || element === document.activeElement;
  });
}

type FocusRef = {
  current: HTMLElement | null;
};

export interface ModalProps {
  open: boolean;
  onClose: () => void;
  children: ReactNode;
  initialFocusRef?: FocusRef;
  className?: string;
  closeOnBackdropClick?: boolean;
}

export default function Modal({
  open,
  onClose,
  children,
  initialFocusRef,
  className,
  closeOnBackdropClick = true,
}: ModalProps) {
  const dialogRef = useRef<HTMLDivElement>(null);
  const previousFocusedRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (!open) return;
    previousFocusedRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";

    const focusTarget = initialFocusRef?.current ?? getFocusableElements(dialogRef.current)[0] ?? dialogRef.current;
    focusTarget?.focus();

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        onClose();
        return;
      }

      if (event.key !== "Tab") return;
      const focusableElements = getFocusableElements(dialogRef.current);
      if (focusableElements.length === 0) {
        event.preventDefault();
        dialogRef.current?.focus();
        return;
      }

      const first = focusableElements[0];
      const last = focusableElements[focusableElements.length - 1];
      const active = document.activeElement as HTMLElement | null;

      if (event.shiftKey) {
        if (!active || active === first || !dialogRef.current?.contains(active)) {
          event.preventDefault();
          last.focus();
        }
        return;
      }

      if (!active || active === last || !dialogRef.current?.contains(active)) {
        event.preventDefault();
        first.focus();
      }
    };

    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.body.style.overflow = previousOverflow;
      document.removeEventListener("keydown", onKeyDown);
      previousFocusedRef.current?.focus();
    };
  }, [initialFocusRef, onClose, open]);

  if (!open || typeof document === "undefined") return null;

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div
        className="absolute inset-0 bg-background/80 backdrop-blur-sm"
        onClick={() => {
          if (closeOnBackdropClick) onClose();
        }}
        aria-hidden="true"
      />
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        tabIndex={-1}
        className={cn(
          "relative z-[1] w-full max-w-md rounded-2xl border border-border/60 bg-card p-4 shadow-xl",
          className
        )}
      >
        {children}
      </div>
    </div>,
    document.body
  );
}
