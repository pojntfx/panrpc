/* eslint-disable no-console */
import { env, exit, stdin, stdout } from "process";
import { createInterface } from "readline/promises";
import { parse } from "url";
// eslint-disable-next-line import/no-extraneous-dependencies
import { Socket, createServer } from "net";
import { ILocalContext, IRemoteContext, Registry } from "./index";

let clients = 0;
let counter = 0;

const registry = new Registry(
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

- a: Print "Hello, world!
`);

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
            await remote.Println(undefined, "Hello, world!");
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

const addr = env.ADDR || "127.0.0.1:1337";
const listen = env.LISTEN !== "false";

if (listen) {
  const u = parse(`tcp://${addr}`);

  const server = createServer(async (socket) => {
    socket.on("error", (e) => {
      console.error("Client disconnected with error:", e);
    });

    registry.linkTCPSocket(
      socket,

      JSON.stringify,
      JSON.parse,

      (v) => v,
      (v) => v
    );
  });

  server.listen(
    {
      host: u.hostname as string,
      port: parseInt(u.port as string, 10),
    },
    () => console.log("Listening on", addr)
  );
} else {
  const u = parse(`tcp://${addr}`);

  const socket = new Socket();

  socket.on("error", (e) => {
    console.error("Disconnected with error:", e.cause);

    exit(1);
  });
  socket.on("close", () => exit(0));

  await new Promise<void>((res, rej) => {
    socket.connect(
      {
        host: u.hostname as string,
        port: parseInt(u.port as string, 10),
      },
      res
    );
    socket.on("error", rej);
  });

  registry.linkTCPSocket(
    socket,

    JSON.stringify,
    JSON.parse,

    (v) => v,
    (v) => v
  );

  console.log("Connected to", addr);
}
