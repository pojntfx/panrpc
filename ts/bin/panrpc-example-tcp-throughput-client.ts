/* eslint-disable no-console */
import { env, exit } from "process";
import { parse } from "url";
// eslint-disable-next-line import/no-extraneous-dependencies
import { JSONParser } from "@streamparser/json-whatwg";
// eslint-disable-next-line import/no-extraneous-dependencies
import { DecoderStream, EncoderStream } from "cbor-x";
import { Socket, createServer } from "net";
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

      console.error(clients, "clients connected");

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

      console.error(clients, "clients connected");
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
    socket.on("error", (e) => {
      console.error("Client disconnected with error:", e);
    });

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
        socket.on("close", () => parser.destroy());

        break;
      }

      default:
        throw new Error("unknown serializer");
    }

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
    () => console.error("Listening on", addr)
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
      socket.on("close", () => parser.destroy());

      break;
    }

    default:
      throw new Error("unknown serializer");
  }

  registry.linkStream(
    encoder,
    decoder,

    (v) => v,
    (v) => v
  );

  console.error("Connected to", addr);
}
