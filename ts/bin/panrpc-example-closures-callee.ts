/* eslint-disable no-console */
import { env, exit } from "process";
import { parse } from "url";
// eslint-disable-next-line import/no-extraneous-dependencies
import { JSONParser } from "@streamparser/json-node";
import { Socket, createServer } from "net";
import { Readable, Transform, TransformCallback, Writable } from "stream";
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
    encoder.pipe(socket);

    const decoder = socket
      .pipe(
        new JSONParser({
          paths: ["$"],
          separator: "",
        })
      )
      .pipe(
        new (class extends Transform {
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
        })
      );

    socket.on("error", (e) => {
      console.error("Client disconnected with error:", e);
    });

    socket.on("close", () => {
      encoder.destroy();
      decoder.destroy();
    });

    registry.linkStream(
      Writable.toWeb(encoder),
      Readable.toWeb(decoder),

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
  encoder.pipe(socket);

  const decoder = socket
    .pipe(
      new JSONParser({
        paths: ["$"],
        separator: "",
      })
    )
    .pipe(
      new (class extends Transform {
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
      })
    );

  socket.on("close", () => {
    encoder.destroy();
    decoder.destroy();
  });

  registry.linkStream(
    Writable.toWeb(encoder),
    Readable.toWeb(decoder),

    (v) => v,
    (v) => v
  );

  console.log("Connected to", addr);
}
