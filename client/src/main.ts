import { Interpolator } from "./interp";
import { Net } from "./net";
import { drawMinimap, Renderer } from "./render";
import type { ClassDef, Me, ServerMsg } from "./types";
import { UI } from "./ui";

const canvas = document.getElementById("game") as HTMLCanvasElement;
const renderer = new Renderer(canvas);
const ui = new UI();
const net = new Net();
const interp = new Interpolator();

let classes: ClassDef[] = [];
let world = { w: 6000, h: 6000 };
let me: Me | null = null;
let playing = false;
const cam = { x: 3000, y: 3000, zoom: 1 };

// ---------- input ----------
const keys = new Set<string>();
let mouseX = 0;
let mouseY = 0;
let mouseDown = false;
let autoFire = false;

window.addEventListener("keydown", (ev) => {
  if (ev.target instanceof HTMLInputElement) return;
  keys.add(ev.code);
  if (!playing) return;
  if (ev.code === "KeyE") autoFire = !autoFire;
  const statIdx = "12345678".indexOf(ev.key);
  if (statIdx >= 0) net.upgradeStat(statIdx);
  const clsIdx = "yuio".indexOf(ev.key.toLowerCase());
  if (clsIdx >= 0 && me && me.upg[clsIdx] !== undefined) net.upgradeClass(me.upg[clsIdx]);
});
window.addEventListener("keyup", (ev) => keys.delete(ev.code));
window.addEventListener("blur", () => keys.clear());
canvas.addEventListener("mousemove", (ev) => {
  mouseX = ev.clientX;
  mouseY = ev.clientY;
});
canvas.addEventListener("mousedown", (ev) => {
  if (ev.button === 0) mouseDown = true;
});
window.addEventListener("mouseup", (ev) => {
  if (ev.button === 0) mouseDown = false;
});
canvas.addEventListener("contextmenu", (ev) => ev.preventDefault());

let lastSent = { dx: 0, dy: 0, aim: 0, fire: false };
let lastSendTime = 0;
function sendInput(now: number): void {
  if (!playing || !me?.alive) return;
  const dx = (keys.has("KeyD") || keys.has("ArrowRight") ? 1 : 0) - (keys.has("KeyA") || keys.has("ArrowLeft") ? 1 : 0);
  const dy = (keys.has("KeyS") || keys.has("ArrowDown") ? 1 : 0) - (keys.has("KeyW") || keys.has("ArrowUp") ? 1 : 0);
  const aim = Math.round(Math.atan2(mouseY - renderer.height / 2, mouseX - renderer.width / 2) * 100) / 100;
  const fire = mouseDown || autoFire;
  const changed = dx !== lastSent.dx || dy !== lastSent.dy || aim !== lastSent.aim || fire !== lastSent.fire;
  if ((changed && now - lastSendTime > 1000 / 40) || now - lastSendTime > 500) {
    lastSent = { dx, dy, aim, fire };
    lastSendTime = now;
    net.input(dx, dy, aim, fire);
  }
}

// ---------- network ----------
const autoName = new URLSearchParams(location.search).get("play");
net.onOpen = () => {
  ui.setStatus("");
  ui.showStart();
  if (autoName !== null) {
    ui.nameInput.value = autoName;
    autoFire = true;
    play();
  }
};
net.onClose = () => {
  playing = false;
  ui.setStatus("Disconnected. Reconnecting…");
  setTimeout(() => net.connect(), 1500);
};
net.onMessage = (m: ServerMsg) => {
  switch (m.t) {
    case "init":
      world = { w: m.w, h: m.h };
      classes = m.classes;
      ui.setClasses(classes);
      ui.setTeams(m.teams);
      ui.buildStats(m.stats);
      break;
    case "state":
      interp.push(m);
      me = m.me;
      // the server only sends the leaderboard when it changes, so always apply it
      if (m.lb) ui.updateLeaderboard(m.lb);
      if (playing) ui.updateMe(m.me);
      break;
    case "dead":
      playing = false;
      autoFire = false;
      ui.showDeath(m.score, m.level, m.killer, m.kills);
      break;
  }
};

ui.onStat = (i) => net.upgradeStat(i);
ui.onClass = (c) => net.upgradeClass(c);

function play(): void {
  const name = ui.nameInput.value.trim();
  try {
    localStorage.setItem("name", name);
  } catch {
    /* ignore */
  }
  net.join(name);
  playing = true;
  ui.showGame();
}
ui.playBtn.onclick = play;
ui.respawnBtn.onclick = play;
ui.nameInput.addEventListener("keydown", (ev) => {
  if (ev.key === "Enter") play();
});
window.addEventListener("keydown", (ev) => {
  if (ev.key === "Enter" && ui.death.style.display === "flex") play();
});
try {
  ui.nameInput.value = localStorage.getItem("name") ?? "";
} catch {
  /* ignore */
}

// ---------- loop ----------
function frame(now: number): void {
  sendInput(now);
  const ents = interp.sample(now);
  if (me) {
    let tx = me.x;
    let ty = me.y;
    if (me.alive) {
      const mine = ents.find((e) => e.i === me!.ent);
      if (mine) {
        tx = mine.x;
        ty = mine.y;
      }
    }
    const targetZoom = renderer.zoomFor(me.fov || 1);
    cam.x += (tx - cam.x) * 0.6;
    cam.y += (ty - cam.y) * 0.6;
    cam.zoom += (targetZoom - cam.zoom) * 0.05;
  } else {
    cam.zoom = renderer.zoomFor(1);
  }
  renderer.draw(ents, cam, world, classes);
  if (playing) drawMinimap(ui.minimap, world, me && me.alive ? { x: cam.x, y: cam.y } : null);
  requestAnimationFrame(frame);
}

ui.setStatus("Connecting…");
net.connect();
requestAnimationFrame(frame);
