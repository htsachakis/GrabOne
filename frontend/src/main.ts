import "./styles.css";

import { Application } from "./app";

const host = document.getElementById("app");
if (!host) {
  throw new Error("the application host element is missing");
}

const application = new Application(host);
void application.start();
