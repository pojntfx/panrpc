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
  async ZeroInt(ctx: IRemoteContext): Promise<number> {
    return 0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroInt8(ctx: IRemoteContext): Promise<number> {
    return 0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroInt16(ctx: IRemoteContext): Promise<number> {
    return 0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroInt32(ctx: IRemoteContext): Promise<number> {
    return 0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroRune(ctx: IRemoteContext): Promise<number> {
    return 0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroInt64(ctx: IRemoteContext): Promise<number> {
    return 0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroUint(ctx: IRemoteContext): Promise<number> {
    return 0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroUint8(ctx: IRemoteContext): Promise<number> {
    return 0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroByte(ctx: IRemoteContext): Promise<number> {
    return 0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroUint16(ctx: IRemoteContext): Promise<number> {
    return 0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroUint32(ctx: IRemoteContext): Promise<number> {
    return 0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroUint64(ctx: IRemoteContext): Promise<number> {
    return 0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroUintptr(ctx: IRemoteContext): Promise<number> {
    return 0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroFloat32(ctx: IRemoteContext): Promise<number> {
    return 0.0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroFloat64(ctx: IRemoteContext): Promise<number> {
    return 0.0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroComplex64(ctx: IRemoteContext): Promise<number> {
    return 0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroComplex128(ctx: IRemoteContext): Promise<number> {
    return 0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroBool(ctx: IRemoteContext): Promise<boolean> {
    return false;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroString(ctx: IRemoteContext): Promise<string> {
    return "";
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroArray(ctx: IRemoteContext): Promise<any[]> {
    return [];
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroSlice(ctx: IRemoteContext): Promise<number[]> {
    return [];
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroStruct(ctx: IRemoteContext): Promise<object> {
    return {};
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
        let rps = 0;
        let currentRuns = 0;

        const interval = setInterval(() => {
          console.log(`${rps} requests/s`);
          rps = 0;

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
                  switch (dataType) {
                    case "int":
                      // eslint-disable-next-line no-await-in-loop
                      await remote.ZeroInt(undefined);

                      break;

                    case "int8":
                      // eslint-disable-next-line no-await-in-loop
                      await remote.ZeroInt8(undefined);

                      break;

                    case "int16":
                      // eslint-disable-next-line no-await-in-loop
                      await remote.ZeroInt16(undefined);

                      break;

                    case "int32":
                      // eslint-disable-next-line no-await-in-loop
                      await remote.ZeroInt32(undefined);

                      break;

                    case "rune":
                      // eslint-disable-next-line no-await-in-loop
                      await remote.ZeroRune(undefined);

                      break;

                    case "int64":
                      // eslint-disable-next-line no-await-in-loop
                      await remote.ZeroInt64(undefined);

                      break;

                    case "uint":
                      // eslint-disable-next-line no-await-in-loop
                      await remote.ZeroUint(undefined);

                      break;

                    case "uint8":
                      // eslint-disable-next-line no-await-in-loop
                      await remote.ZeroUint8(undefined);

                      break;

                    case "byte":
                      // eslint-disable-next-line no-await-in-loop
                      await remote.ZeroByte(undefined);

                      break;

                    case "uint16":
                      // eslint-disable-next-line no-await-in-loop
                      await remote.ZeroUint16(undefined);

                      break;

                    case "uint32":
                      // eslint-disable-next-line no-await-in-loop
                      await remote.ZeroUint32(undefined);

                      break;

                    case "uint64":
                      // eslint-disable-next-line no-await-in-loop
                      await remote.ZeroUint64(undefined);

                      break;

                    case "uintptr":
                      // eslint-disable-next-line no-await-in-loop
                      await remote.ZeroUintptr(undefined);

                      break;

                    case "float32":
                      // eslint-disable-next-line no-await-in-loop
                      await remote.ZeroFloat32(undefined);

                      break;

                    case "float64":
                      // eslint-disable-next-line no-await-in-loop
                      await remote.ZeroFloat64(undefined);

                      break;

                    case "complex64":
                      // eslint-disable-next-line no-await-in-loop
                      await remote.ZeroComplex64(undefined);

                      break;

                    case "complex128":
                      // eslint-disable-next-line no-await-in-loop
                      await remote.ZeroComplex128(undefined);

                      break;

                    case "bool":
                      // eslint-disable-next-line no-await-in-loop
                      await remote.ZeroBool(undefined);

                      break;

                    case "string":
                      // eslint-disable-next-line no-await-in-loop
                      await remote.ZeroString(undefined);

                      break;

                    case "array":
                      // eslint-disable-next-line no-await-in-loop
                      await remote.ZeroArray(undefined);

                      break;

                    case "slice":
                      // eslint-disable-next-line no-await-in-loop
                      await remote.ZeroSlice(undefined);

                      break;

                    case "struct":
                      // eslint-disable-next-line no-await-in-loop
                      await remote.ZeroStruct(undefined);

                      break;

                    default:
                      throw new Error("unknown data type");
                  }

                  rps += 1;
                } catch (e) {
                  console.error(`Got error for data type func: ${e}`);

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
const dataType = env.DATA_TYPE || "int";

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
    encoder,
    decoder,

    (v) => v,
    (v) => v
  );

  console.log("Connected to", addr);
}
