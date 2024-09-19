/* eslint-disable no-console */
import { env, exit } from "process";
import { parse } from "url";
// eslint-disable-next-line import/no-extraneous-dependencies
import { JSONParser } from "@streamparser/json-whatwg";
// eslint-disable-next-line import/no-extraneous-dependencies
import { DecoderStream, EncoderStream } from "cbor-x";
import { Socket, createServer } from "net";
import { ILocalContext, Registry } from "../index";

class Local {
  #buf: number[];

  constructor(buf: number[]) {
    this.#buf = buf;

    this.GetBytes = this.GetBytes.bind(this);
  }

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  async GetBytes(ctx: ILocalContext): Promise<number[]> {
    return this.#buf;
  }
}

class Remote {}

const buffer = parseInt(env.BUFFER || `${1 * 128 * 1024}`, 10); // We can't do larger buffer sizes than this in TypeScript

let clients = 0;

const registry = new Registry(
  new Local(new Array(buffer)),
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
    socket.on("error", (e) => {
      console.error("Client disconnected with error:", e);
    });

    const linkSignal = new AbortController();

    let encoder: WritableStream;
    let decoder: ReadableStream;

    switch (serializer) {
      case "json": {
        encoder = new WritableStream({
          write(chunk) {
            return new Promise<void>((res) => {
              const isDrained = socket.write(JSON.stringify(chunk));

              if (!isDrained) {
                socket.once("drain", res);
              } else {
                res();
              }
            });
          },
          close() {
            return new Promise((res) => {
              socket.end(res);
            });
          },
          abort(reason) {
            socket.destroy(
              reason instanceof Error ? reason : new Error(reason)
            );
          },
        });

        const parser = new JSONParser({
          paths: ["$"],
          separator: "",
        });
        const parserWriter = parser.writable.getWriter();
        const parserReader = parser.readable.getReader();
        decoder = new ReadableStream({
          start(controller) {
            parserReader
              .read()
              .then(async function process({ done, value }) {
                if (done) {
                  controller.close();

                  return;
                }

                controller.enqueue(value?.value);

                parserReader
                  .read()
                  .then(process)
                  .catch((e) => controller.error(e));
              })
              .catch((e) => controller.error(e));
          },
        });
        socket.on("data", (m) => parserWriter.write(m));
        socket.on("close", () => {
          parserReader.cancel();
          parserWriter.abort();
          linkSignal.abort();
        });

        break;
      }

      case "cbor": {
        const marshaller = new EncoderStream({
          useRecords: false,
        });
        marshaller.pipe(socket);
        encoder = new WritableStream({
          write(chunk) {
            return new Promise<void>((res) => {
              const isDrained = marshaller.write(chunk, "utf8");

              if (!isDrained) {
                marshaller.once("drain", res);
              } else {
                res();
              }
            });
          },
          close() {
            return new Promise((res) => {
              marshaller.end(res);
            });
          },
          abort(reason) {
            marshaller.destroy(
              reason instanceof Error ? reason : new Error(reason)
            );
          },
        });

        const parser = new DecoderStream({
          useRecords: false,
        });
        decoder = new ReadableStream({
          start(controller) {
            parser.on("data", (chunk) => {
              controller.enqueue(chunk);

              if (controller.desiredSize && controller.desiredSize <= 0) {
                parser.pause();
              }
            });

            parser.on("close", () => controller.close());
            parser.on("error", (err) => controller.error(err));
          },
          pull() {
            parser.resume();
          },
          cancel() {
            parser.destroy();
          },
        });
        socket.on("data", (m) => parser.write(m));
        socket.on("close", () => {
          parser.destroy();
          linkSignal.abort();
        });

        break;
      }

      default:
        throw new Error("unknown serializer");
    }

    registry.linkStream(
      linkSignal.signal,

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

  const linkSignal = new AbortController();

  let encoder: WritableStream;
  let decoder: ReadableStream;

  switch (serializer) {
    case "json": {
      encoder = new WritableStream({
        write(chunk) {
          return new Promise<void>((res) => {
            const isDrained = socket.write(JSON.stringify(chunk));

            if (!isDrained) {
              socket.once("drain", res);
            } else {
              res();
            }
          });
        },
        close() {
          return new Promise((res) => {
            socket.end(res);
          });
        },
        abort(reason) {
          socket.destroy(reason instanceof Error ? reason : new Error(reason));
        },
      });

      const parser = new JSONParser({
        paths: ["$"],
        separator: "",
      });
      const parserWriter = parser.writable.getWriter();
      const parserReader = parser.readable.getReader();
      decoder = new ReadableStream({
        start(controller) {
          parserReader
            .read()
            .then(async function process({ done, value }) {
              if (done) {
                controller.close();

                return;
              }

              controller.enqueue(value?.value);

              parserReader
                .read()
                .then(process)
                .catch((e) => controller.error(e));
            })
            .catch((e) => controller.error(e));
        },
      });
      socket.on("data", (m) => parserWriter.write(m));
      socket.on("close", () => {
        parserReader.cancel();
        parserWriter.abort();
        linkSignal.abort();
      });

      break;
    }

    case "cbor": {
      const marshaller = new EncoderStream({
        useRecords: false,
      });
      marshaller.pipe(socket);
      encoder = new WritableStream({
        write(chunk) {
          return new Promise<void>((res) => {
            const isDrained = marshaller.write(chunk, "utf8");

            if (!isDrained) {
              marshaller.once("drain", res);
            } else {
              res();
            }
          });
        },
        close() {
          return new Promise((res) => {
            marshaller.end(res);
          });
        },
        abort(reason) {
          marshaller.destroy(
            reason instanceof Error ? reason : new Error(reason)
          );
        },
      });

      const parser = new DecoderStream({
        useRecords: false,
      });
      decoder = new ReadableStream({
        start(controller) {
          parser.on("data", (chunk) => {
            controller.enqueue(chunk);

            if (controller.desiredSize && controller.desiredSize <= 0) {
              parser.pause();
            }
          });

          parser.on("close", () => controller.close());
          parser.on("error", (err) => controller.error(err));
        },
        pull() {
          parser.resume();
        },
        cancel() {
          parser.destroy();
        },
      });
      socket.on("data", (m) => parser.write(m));
      socket.on("close", () => {
        parser.destroy();
        linkSignal.abort();
      });

      break;
    }

    default:
      throw new Error("unknown serializer");
  }

  registry.linkStream(
    linkSignal.signal,

    encoder,
    decoder,

    (v) => v,
    (v) => v
  );

  console.log("Connected to", addr);
}
