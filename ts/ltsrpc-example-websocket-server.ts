/* eslint-disable no-console */
import { env, stdin, stdout } from "process";
import { createInterface } from "readline/promises";
import { parse } from "url";
// eslint-disable-next-line import/no-extraneous-dependencies
import { WebSocketServer } from "ws";
import { ILocalContext, IRemoteContext, linkWebSocket } from "./index";

const rl = createInterface({ input: stdin, output: stdout });

const laddr = env.LADDR || "tcp://127.0.0.1:1337";
const u = parse(laddr);

let clients = 0;
let counter = 0;
const server = new WebSocketServer({
  host: u.hostname as string,
  port: parseInt(u.port as string, 10),
});
server.on("connection", async (socket) => {
  socket.addEventListener("error", (e) => {
    console.error("Client disconnected with error:", e);
  });

  const remote = linkWebSocket(
    socket,

    {
      Increment: async (ctx: ILocalContext, delta: number): Promise<number> => {
        console.log(
          "Incrementing counter by",
          delta,
          "for remote with ID",
          ctx.remoteID
        );

        counter += delta;

        return counter;
      },
    },
    {
      // eslint-disable-next-line @typescript-eslint/no-unused-vars
      Println: async (ctx: IRemoteContext, msg: string) => {},
    },

    JSON.stringify,
    JSON.parse,

    (v) => v,
    (v) => v
  );
  socket.addEventListener("close", () => {
    clients--;

    console.log(clients, "clients connected");
  });

  clients++;

  console.log(clients, "clients connected");

  // eslint-disable-next-line no-constant-condition
  while (true) {
    const line =
      // eslint-disable-next-line no-await-in-loop
      await rl.question(`Enter one of the following letters followed by <ENTER> to run a function on the remote(s):

- a: Print "Hello, world!
`);

    switch (line) {
      case "a":
        try {
          // eslint-disable-next-line no-await-in-loop
          await remote.Println(undefined, "Hello, world!");
        } catch (e) {
          console.error(`Got error for Println func: ${e}`);
        }
        break;

      default:
        console.log(`Unknown letter ${line}, ignoring input`);
    }
  }
});

console.log("Listening on", laddr);
