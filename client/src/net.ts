import { Decoder, encodeInput, encodeJoin, encodeStat, encodeUpgrade } from "./protocol";
import type { ServerMsg } from "./types";

export class Net {
  private ws: WebSocket | null = null;
  private decoder = new Decoder();
  onMessage: (m: ServerMsg) => void = () => {};
  onOpen: () => void = () => {};
  onClose: () => void = () => {};

  connect(): void {
    const proto = location.protocol === "https:" ? "wss:" : "ws:";
    const ws = new WebSocket(`${proto}//${location.host}/ws`);
    ws.binaryType = "arraybuffer";
    this.ws = ws;
    this.decoder.reset();
    ws.onopen = () => this.onOpen();
    ws.onclose = () => {
      this.ws = null;
      this.onClose();
    };
    ws.onmessage = (ev) => {
      if (!(ev.data instanceof ArrayBuffer)) return;
      let msg: ServerMsg;
      try {
        msg = this.decoder.decode(ev.data);
      } catch (e) {
        console.error("bad message", e);
        return;
      }
      this.onMessage(msg);
    };
  }

  get connected(): boolean {
    return this.ws !== null && this.ws.readyState === WebSocket.OPEN;
  }

  private send(buf: ArrayBuffer): void {
    if (this.connected) this.ws!.send(buf);
  }

  join(name: string): void {
    this.send(encodeJoin(name));
  }
  input(dx: number, dy: number, aim: number, fire: boolean): void {
    this.send(encodeInput(dx, dy, aim, fire));
  }
  upgradeStat(i: number): void {
    this.send(encodeStat(i));
  }
  upgradeClass(c: number): void {
    this.send(encodeUpgrade(c));
  }
}
