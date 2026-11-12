import { mount } from "svelte";
import "./theme.css";
import Popup from "./Popup.svelte";
import Settings from "./Settings.svelte";

// Each Wails window loads the same bundle; the settings window opens "/?view=settings".
const view = new URLSearchParams(location.search).get("view");
const target = document.getElementById("app")!;

if (view === "settings") {
  document.title = "Yaku の設定";
  mount(Settings, { target });
} else {
  mount(Popup, { target });
}
