/* eslint-disable no-alert */
/* eslint-disable no-console */
import { env, exit } from "process";
import { linkWebSocket } from "./index";

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

const remote = linkWebSocket(
  socket,

  {
    Println: async (msg: string) => {
      console.log(msg);
    },
  },
  {
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    Increment: async (delta: number): Promise<number> => 0,
  },

  1000 * 10,

  JSON.stringify,
  JSON.parse,

  (v) => v,
  (v) => v
);

console.log("Connected to", raddr);

// eslint-disable-next-line no-constant-condition
while (true) {
  const line =
    prompt(`Enter one of the following letters followed by <ENTER> to run a function on the remote(s):

- a: Increment remote counter by one
- b: Decrement remote counter by one
`);

  switch (line) {
    case "a":
      try {
        // eslint-disable-next-line no-await-in-loop
        await remote.Increment(1);
      } catch (e) {
        console.error(`Got error for Increment func: ${e}`);
      }

      break;

    case "b":
      try {
        // eslint-disable-next-line no-await-in-loop
        await remote.Increment(-1);
      } catch (e) {
        console.error(`Got error for Increment func: ${e}`);
      }

      break;

    default:
      console.log(`Unknown letter ${line}, ignoring input`);
  }
}
