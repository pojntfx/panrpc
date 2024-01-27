/* eslint-disable no-console */
// eslint-disable-next-line import/no-extraneous-dependencies
import { JSONParser } from "@streamparser/json-node";
import { env, exit, stdin, stdout } from "process";
import { createInterface } from "readline/promises";
import { Transform, TransformCallback } from "stream";
import { parse } from "url";
// eslint-disable-next-line import/no-extraneous-dependencies
import { WebSocket, WebSocketServer } from "ws";
import { ILocalContext, IRemoteContext, Registry } from "../index";

class Local {
  // eslint-disable-next-line class-methods-use-this
  async Println(ctx: ILocalContext, msg: string) {
    console.log("Printing message", msg, "for remote with ID", ctx.remoteID);

    console.log(msg);
  }
}

class Remote {
  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async Increment(ctx: IRemoteContext, delta: number): Promise<number> {
    return 0;
  }
}

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

const addr = env.ADDR || "127.0.0.1:1337";
const listen = env.LISTEN === "true";

if (listen) {
  const u = parse(`ws://${addr}`);

  const server = new WebSocketServer({
    host: u.hostname as string,
    port: parseInt(u.port as string, 10),
  });

  server.on("connection", (socket) => {
    socket.addEventListener("error", (e) => {
      console.error("Client disconnected with error:", e);
    });

    const encoder = new (class extends Transform {
      _transform(
        chunk: any,
        encoding: BufferEncoding,
        callback: TransformCallback
      ) {
        this.push(JSON.stringify(chunk));
        callback();
      }
    })({
      objectMode: true,
    });
    encoder.addListener("data", (chunk) => socket.send(chunk));

    const decoder = new (class extends Transform {
      _transform(
        chunk: any,
        encoding: BufferEncoding,
        callback: TransformCallback
      ) {
        this.push(chunk?.value);
        callback();
      }
    })({
      objectMode: true,
    });

    const parser = new JSONParser({
      paths: ["$"],
      separator: "",
    });
    parser.pipe(decoder);

    socket.addEventListener("message", (m) => parser.write(m.data));

    socket.addEventListener("close", () => {
      encoder.destroy();
      decoder.destroy();
    });

    registry.linkStream(
      encoder,
      decoder,

      (v) => v,
      (v) => v
    );
  });

  console.log("Listening on", addr);
} else {
  const socket = new WebSocket(`ws://${addr}`);

  socket.addEventListener("error", (e) => {
    console.error("Disconnected with error:", e);

    exit(1);
  });
  socket.addEventListener("close", () => exit(0));

  await new Promise<void>((res, rej) => {
    socket.addEventListener("open", () => res());
    socket.addEventListener("error", rej);
  });

  const encoder = new (class extends Transform {
    _transform(
      chunk: any,
      encoding: BufferEncoding,
      callback: TransformCallback
    ) {
      this.push(JSON.stringify(chunk));
      callback();
    }
  })({
    objectMode: true,
  });
  encoder.addListener("data", (chunk) => socket.send(chunk));

  const decoder = new (class extends Transform {
    _transform(
      chunk: any,
      encoding: BufferEncoding,
      callback: TransformCallback
    ) {
      this.push(chunk?.value);
      callback();
    }
  })({
    objectMode: true,
  });

  const parser = new JSONParser({
    paths: ["$"],
    separator: "",
  });
  parser.pipe(decoder);

  socket.addEventListener("message", (m) => parser.write(m.data));

  socket.addEventListener("close", () => {
    encoder.destroy();
    decoder.destroy();
  });

  registry.linkStream(
    encoder,
    decoder,

    (v) => v,
    (v) => v
  );

  console.log("Connected to", addr);
}
