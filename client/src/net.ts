import type { ServerMsg } from "./types";

export class Net {
  private ws: WebSocket | null = null;
  onMessage: (m: ServerMsg) => void = () => {};
  onOpen: () => void = () => {};
  onClose: () => void = () => {};

  connect(): void {
    const proto = location.protocol === "https:" ? "wss:" : "ws:";
    const ws = new WebSocket(`${proto}//${location.host}/ws`);
    this.ws = ws;
    ws.onopen = () => this.onOpen();
    ws.onclose = () => {
      this.ws = null;
      this.onClose();
    };
    ws.onmessage = (ev) => {
      try {
        this.onMessage(JSON.parse(ev.data as string) as ServerMsg);
      } catch (e) {
        console.error("bad message", e);
      }
    };
  }

  get connected(): boolean {
    return this.ws !== null && this.ws.readyState === WebSocket.OPEN;
  }

  send(obj: unknown): void {
    if (this.connected) this.ws!.send(JSON.stringify(obj));
  }

  join(name: string): void {
    this.send({ t: "join", name });
  }
  input(dx: number, dy: number, aim: number, fire: boolean): void {
    this.send({ t: "in", dx, dy, aim, fire });
  }
  upgradeStat(i: number): void {
    this.send({ t: "stat", i });
  }
  upgradeClass(c: number): void {
    this.send({ t: "up", c });
  }
}
