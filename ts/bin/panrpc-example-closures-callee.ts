/* eslint-disable no-console */
import { env, exit } from "process";
import { parse } from "url";
// eslint-disable-next-line import/no-extraneous-dependencies
import { Socket, createServer } from "net";
import Chain from "stream-chain";
import p from "stream-json/jsonl/Parser";
import Stringer from "stream-json/jsonl/Stringer";
import {
  ILocalContext,
  IRemoteContext,
  Registry,
  remoteClosure,
} from "../index";

class Local {
  // eslint-disable-next-line class-methods-use-this
  async Iterate(
    ctx: ILocalContext,
    length: number,
    @remoteClosure
    onIteration: (ctx: IRemoteContext, i: number, b: string) => Promise<string>
  ): Promise<number> {
    for (let i = 0; i < length; i++) {
      // eslint-disable-next-line no-await-in-loop
      const rv = await onIteration(undefined, i, "This is from the callee");

      console.log("Closure returned:", rv);
    }

    return length;
  }
}

class Remote {}

let clients = 0;

const registry = new Registry(
  new Local(),
  new Remote(),

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

const addr = env.ADDR || "127.0.0.1:1337";
const listen = env.LISTEN !== "false";

if (listen) {
  const u = parse(`tcp://${addr}`);

  const server = createServer(async (socket) => {
    socket.on("error", (e) => {
      console.error("Client disconnected with error:", e);
    });

    const decoder = new Chain([p.parser(), (v) => v.value]);
    socket.pipe(decoder);

    const encoder = new Stringer();
    encoder.pipe(socket);

    registry.linkStream(
      encoder,
      decoder,

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

  const decoder = new Chain([p.parser(), (v) => v.value]);
  socket.pipe(decoder);

  const encoder = new Stringer();
  encoder.pipe(socket);

  registry.linkStream(
    encoder,
    decoder,

    (v) => v,
    (v) => v
  );

  console.log("Connected to", addr);
}
