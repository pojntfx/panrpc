/* eslint-disable no-console */
import { env, exit } from "process";
import { parse } from "url";
// eslint-disable-next-line import/no-extraneous-dependencies
import { JSONParser } from "@streamparser/json-node";
// eslint-disable-next-line import/no-extraneous-dependencies
import { DecoderStream, EncoderStream } from "cbor-x";
import { Socket, createServer } from "net";
import { Readable, Writable, Transform, TransformCallback } from "stream";
import { ILocalContext, Registry } from "../index";

class Local {
  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroInt(ctx: ILocalContext): Promise<number> {
    return 0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroInt8(ctx: ILocalContext): Promise<number> {
    return 0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroInt16(ctx: ILocalContext): Promise<number> {
    return 0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroInt32(ctx: ILocalContext): Promise<number> {
    return 0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroRune(ctx: ILocalContext): Promise<number> {
    return 0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroInt64(ctx: ILocalContext): Promise<number> {
    return 0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroUint(ctx: ILocalContext): Promise<number> {
    return 0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroUint8(ctx: ILocalContext): Promise<number> {
    return 0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroByte(ctx: ILocalContext): Promise<number> {
    return 0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroUint16(ctx: ILocalContext): Promise<number> {
    return 0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroUint32(ctx: ILocalContext): Promise<number> {
    return 0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroUint64(ctx: ILocalContext): Promise<number> {
    return 0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroUintptr(ctx: ILocalContext): Promise<number> {
    return 0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroFloat32(ctx: ILocalContext): Promise<number> {
    return 0.0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroFloat64(ctx: ILocalContext): Promise<number> {
    return 0.0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroComplex64(ctx: ILocalContext): Promise<number> {
    return 0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroComplex128(ctx: ILocalContext): Promise<number> {
    return 0;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroBool(ctx: ILocalContext): Promise<boolean> {
    return false;
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroString(ctx: ILocalContext): Promise<string> {
    return "";
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroArray(ctx: ILocalContext): Promise<any[]> {
    return [];
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroSlice(ctx: ILocalContext): Promise<number[]> {
    return [];
  }

  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars
  async ZeroStruct(ctx: ILocalContext): Promise<object> {
    return {};
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
