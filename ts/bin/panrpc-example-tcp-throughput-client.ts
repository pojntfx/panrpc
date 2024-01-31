/* eslint-disable no-console */
import { env, exit } from "process";
import { parse } from "url";
// eslint-disable-next-line import/no-extraneous-dependencies
import { JSONParser } from "@streamparser/json-node";
// eslint-disable-next-line import/no-extraneous-dependencies
import { DecoderStream, EncoderStream } from "cbor-x";
import { Socket, createServer } from "net";
import { Readable, Transform, TransformCallback, Writable } from "stream";
import { IRemoteContext, Registry } from "../index";

class Local {}

class Remote {
  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async GetBytes(ctx: IRemoteContext): Promise<number[]> {
    return [];
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

      (async () => {
        let bytesTransferred = 0;
        let currentRuns = 0;

        const interval = setInterval(() => {
          console.log(`${Math.round(bytesTransferred / (1024 * 1024))} MB/s`);
          bytesTransferred = 0;

          currentRuns++;
          if (currentRuns >= runs) {
            clearInterval(interval);
            process.exit(0);
          }
        }, 1000);

        await registry.forRemotes(async (remoteID, remote) => {
          for (let i = 0; i < concurrency; i++) {
            // eslint-disable-next-line @typescript-eslint/no-loop-func
            (async () => {
              // eslint-disable-next-line no-constant-condition
              while (true) {
                try {
                  // eslint-disable-next-line no-await-in-loop
                  const res = await remote.GetBytes(undefined);

                  bytesTransferred += res.length;
                } catch (e) {
                  console.error(`Got error for GetBytes func: ${e}`);

                  return;
                }
              }
            })();
          }
        });
      })();
    },
    onClientDisconnect: () => {
      clients--;

      console.log(clients, "clients connected");
    },
  }
);

const addr = env.ADDR || "127.0.0.1:1337";
const listen = env.LISTEN === "true";
const concurrency = parseInt(env.CONCURRENCY || "512", 10);
const runs = parseInt(env.RUNS || "10", 10);
const serializer = env.SERIALIZER || "json";

if (listen) {
  const u = parse(`tcp://${addr}`);

  const server = createServer(async (socket) => {
    let encoder: Writable;
    let decoder: Readable;

    switch (serializer) {
      case "json":
        encoder = new (class extends Transform {
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

        decoder = socket
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

        break;

      case "cbor":
        encoder = new EncoderStream({
          useRecords: false,
        });
        encoder.pipe(socket);

        decoder = socket.pipe(
          new DecoderStream({
            useRecords: false,
          })
        );

        break;

      default:
        throw new Error("unknown serializer");
    }

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

  let encoder: Writable;
  let decoder: Readable;

  switch (serializer) {
    case "json":
      encoder = new (class extends Transform {
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

      decoder = socket
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

      break;

    case "cbor":
      encoder = new EncoderStream({
        useRecords: false,
      });
      encoder.pipe(socket);

      decoder = socket.pipe(
        new DecoderStream({
          useRecords: false,
        })
      );

      break;

    default:
      throw new Error("unknown serializer");
  }

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
