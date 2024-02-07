/* eslint-disable no-console */
import { env, exit, stdin, stdout } from "process";
import { createInterface } from "readline/promises";
import { parse } from "url";
// eslint-disable-next-line import/no-extraneous-dependencies
import { JSONParser } from "@streamparser/json-whatwg";
import { Socket, createServer } from "net";
import { ILocalContext, IRemoteContext, Registry } from "../index";

class Local {}

class Remote {
  // eslint-disable-next-line class-methods-use-this
  async Iterate(
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    ctx: IRemoteContext,
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    length: number,
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    onIteration: (ctx: ILocalContext, i: number, b: string) => Promise<string>
  ): Promise<number> {
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

- a: Iterate over 5
- b: Iterate over 10`);

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
            const length = await remote.Iterate(
              undefined,
              5,
              async (_, i, b) => {
                console.log("In iteration", i, b);

                return "This is from the caller";
              }
            );

            console.log(length);
          } catch (e) {
            console.error(`Got error for Iterate func: ${e}`);
          }

          break;

        case "b":
          try {
            // eslint-disable-next-line no-await-in-loop
            const length = await remote.Iterate(
              undefined,
              10,
              async (_, i, b) => {
                console.log("In iteration", i, b);

                return "This is from the caller";
              }
            );

            console.log(length);
          } catch (e) {
            console.error(`Got error for Iterate func: ${e}`);
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
