import { Moon, Sun } from "lucide-react";

import { cn } from "@/lib/utils";

type ThemeToggleProps = {
  className?: string;
};

export function ThemeToggle({ className }: ThemeToggleProps) {
  const isDark = document.documentElement.classList.contains("dark");

  const toggleTheme = () => {
    const html = document.documentElement;
    const newIsDark = !html.classList.contains("dark");

    if (newIsDark) {
      html.classList.add("dark");
      localStorage.setItem("genfu.ui.theme", "dark");
    } else {
      html.classList.remove("dark");
      localStorage.setItem("genfu.ui.theme", "light");
    }
  };

  return (
    <button
      type="button"
      className={cn(
        "inline-flex h-9 w-9 items-center justify-center rounded-lg border border-border bg-card text-foreground transition-colors hover:bg-accent hover:text-accent-foreground",
        className
      )}
      aria-label={isDark ? "Switch to light mode" : "Switch to dark mode"}
      onClick={toggleTheme}
    >
      {isDark ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
    </button>
  );
}

export function initTheme() {
  const stored = localStorage.getItem("genfu.ui.theme");
  if (stored === "dark") {
    document.documentElement.classList.add("dark");
  } else if (stored === "light") {
    document.documentElement.classList.remove("dark");
  } else if (window.matchMedia("(prefers-color-scheme: dark)").matches) {
    document.documentElement.classList.add("dark");
  }
}
