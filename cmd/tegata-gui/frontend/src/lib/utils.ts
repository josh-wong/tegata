import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatError(err: unknown, fallback: string): string {
  if (typeof err === "string") return err
  if (err instanceof Error) return err.message
  return fallback
}
