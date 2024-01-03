/* eslint-disable no-console */
import { env, exit, stdin, stdout } from "process";
import { createInterface } from "readline/promises";
import { ILocalContext, IRemoteContext, Registry } from "./index";

let clients = 0;

const registry = new Registry(
  {
    Println: async (ctx: ILocalContext, msg: string) => {
      console.log("Printing message", msg, "for remote with ID", ctx.remoteID);

      console.log(msg);
    },
  },
  {
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    Increment: async (ctx: IRemoteContext, delta: number): Promise<number> => 0,
  },
  {
    onClientConnect: () => {
      clients++;

      console.log(clients, "clients connected");
    },
    onClientDisconnect: () => {
      clients--;

      console.log(clients, "clients connected");
    },
  }
);

(async () => {
  console.log(`Enter one of the following letters followed by <ENTER> to run a function on the remote(s):

- a: Increment remote counter by one
- b: Decrement remote counter by one`);

  const rl = createInterface({ input: stdin, output: stdout });

  // eslint-disable-next-line no-constant-condition
  while (true) {
    const line =
      // eslint-disable-next-line no-await-in-loop
      await rl.question("");

    // eslint-disable-next-line no-await-in-loop
    await registry.forRemotes(async (remoteID, remote) => {
      console.log("Calling functions for remote with ID", remoteID);

      switch (line) {
        case "a":
          try {
            // eslint-disable-next-line no-await-in-loop
            const res = await remote.Increment(undefined, 1);

            console.log(res);
          } catch (e) {
            console.error(`Got error for Increment func: ${e}`);
          }
          break;

        case "b":
          try {
            // eslint-disable-next-line no-await-in-loop
            const res = await remote.Increment(undefined, -1);

            console.log(res);
          } catch (e) {
            console.error(`Got error for Increment func: ${e}`);
          }

          break;

        default:
          console.log(`Unknown letter ${line}, ignoring input`);
      }
    });
  }
})();

const raddr = env.RADDR || "ws://127.0.0.1:1337";

const socket = new WebSocket(raddr);

socket.addEventListener("close", (e) => {
  console.error("Disconnected with error:", e.reason);

  exit(1);
});

await new Promise<void>((res, rej) => {
  socket.addEventListener("open", () => res());
  socket.addEventListener("error", rej);
});

registry.linkWebSocket(
  socket,

  JSON.stringify,
  JSON.parse,

  (v) => v,
  (v) => v
);

console.log("Connected to", raddr);
