/* eslint-disable no-console */
import { env, exit } from "process";
import { parse } from "url";
// eslint-disable-next-line import/no-extraneous-dependencies
import { JSONParser } from "@streamparser/json-whatwg";
import { Socket, createServer } from "net";
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

    const encoder = new WritableStream({
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
    const decoder = new ReadableStream({
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

  const encoder = new WritableStream({
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
  const decoder = new ReadableStream({
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

  registry.linkStream(
    encoder,
    decoder,

    (v) => v,
    (v) => v
  );

  console.log("Connected to", addr);
}
