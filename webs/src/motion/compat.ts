// Minimal bridge for business views pending their visual migration.
import { useStagger } from "./index";
export function initSpotlight(container: HTMLElement | null) {
  // Spotlight decoration was retired; preserve existing ownership/cleanup calls.
  return () => {
    container?.style.removeProperty("--mouse-x");
    container?.style.removeProperty("--mouse-y");
  };
}
export const useLegacyStagger = useStagger;
