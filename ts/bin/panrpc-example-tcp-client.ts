/* eslint-disable no-console */
import { env, exit, stdin, stdout } from "process";
import { createInterface } from "readline/promises";
import { parse } from "url";
// eslint-disable-next-line import/no-extraneous-dependencies
import { DecoderStream, EncoderStream } from "cbor-x";
import { Socket, createServer } from "net";
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
  const u = parse(`tcp://${addr}`);

  const server = createServer(async (socket) => {
    const marshaller = new EncoderStream({
      useRecords: false,
    });
    marshaller.pipe(socket as unknown as WritableStream);
    const encoder = new WritableStream({
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
    const decoder = new ReadableStream({
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

  const marshaller = new EncoderStream({
    useRecords: false,
  });
  marshaller.pipe(socket as unknown as WritableStream);
  const encoder = new WritableStream({
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
      marshaller.destroy(reason instanceof Error ? reason : new Error(reason));
    },
  });

  const parser = new DecoderStream({
    useRecords: false,
  });
  const decoder = new ReadableStream({
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

  registry.linkStream(
    encoder,
    decoder,

    (v) => v,
    (v) => v
  );

  console.log("Connected to", addr);
}
