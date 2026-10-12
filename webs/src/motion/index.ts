import type { App } from "vue";
import type { Router } from "vue-router";
import { vReveal, vMagnetic, vTilt } from "./directives";
import { setupSpotlight } from "./spotlight";
import { setupAutoReveal } from "./autoReveal";

export { default as CountUp } from "./CountUp.vue";

export function setupMotion(app: App, router: Router) {
  app.directive("reveal", vReveal);
  app.directive("magnetic", vMagnetic);
  app.directive("tilt", vTilt);
  setupSpotlight();
  setupAutoReveal(router);
}
