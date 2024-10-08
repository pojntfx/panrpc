/* eslint-disable no-console */
import { env, exit } from "process";
import { parse } from "url";
// eslint-disable-next-line import/no-extraneous-dependencies
import { JSONParser } from "@streamparser/json-whatwg";
import { Socket, createServer } from "net";
import { ILocalContext, IRemoteContext, Registry } from "../index";

class Local {
  #counter = 0;

  public forRemotes?: (
    cb: (remoteID: string, remote: Remote) => Promise<void>
  ) => Promise<void>;

  constructor() {
    this.Increment = this.Increment.bind(this);
  }

  async Increment(ctx: ILocalContext, delta: number): Promise<number> {
    const { remoteID: targetID } = ctx;

    await this.forRemotes?.(async (remoteID, remote) => {
      if (remoteID === targetID) {
        await remote.Println(undefined, `Incrementing counter by ${delta}`);
      }
    });

    this.#counter += delta;

    return this.#counter;
  }
}

class Remote {
  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars, @typescript-eslint/no-empty-function
  async Println(ctx: IRemoteContext, msg: string) {}
}

const service = new Local();

let clients = 0;

const registry = new Registry(
  service,
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

service.forRemotes = registry.forRemotes;

const addr = env.ADDR || "127.0.0.1:1337";
const listen = env.LISTEN !== "false";

if (listen) {
  const u = parse(`tcp://${addr}`);

  const server = createServer(async (socket) => {
    socket.on("error", (e) => {
      console.error("Client disconnected with error:", e);
    });

    const linkSignal = new AbortController();

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
      linkSignal.abort();
    });

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
    linkSignal.abort();
  });

  registry.linkStream(
    linkSignal.signal,

    encoder,
    decoder,

    (v) => v,
    (v) => v
  );

  console.log("Connected to", addr);
}
