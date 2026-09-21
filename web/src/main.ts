import { createApp } from "vue";
import App from "./App.vue";
import { followScrollActivity } from "./scrollbars";
import { followSystemTheme } from "./theme";
import "../styles.css";

document.addEventListener(
  "wheel",
  (event) => {
    const target = event.target;
    if (
      target instanceof HTMLInputElement &&
      target.type === "number" &&
      document.activeElement === target
    ) {
      target.blur();
    }
  },
  { capture: true },
);

const systemTheme = followSystemTheme();
const stopScrollActivity = followScrollActivity();
void systemTheme.ready.then(() => {
  createApp(App).mount("#app");
});
window.addEventListener("pagehide", (event: PageTransitionEvent) => {
  if (!event.persisted) {
    systemTheme.stop();
    stopScrollActivity();
  }
});
