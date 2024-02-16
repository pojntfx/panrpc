/* eslint-disable no-console */
import { stdin, stdout } from "process";
// eslint-disable-next-line import/no-extraneous-dependencies
import { JSONParser } from "@streamparser/json-whatwg";
import { ILocalContext, IRemoteContext, Registry } from "../index";

class Local {
  private counter = 0;

  constructor() {
    this.Increment = this.Increment.bind(this);
  }

  async Increment(ctx: ILocalContext, delta: number): Promise<number> {
    console.log(
      "Incrementing counter by",
      delta,
      "for remote with ID",
      ctx.remoteID
    );

    this.counter += delta;

    return this.counter;
  }
}

class Remote {
  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars, @typescript-eslint/no-empty-function
  async Println(ctx: IRemoteContext, msg: string) {}
}

let clients = 0;

const registry = new Registry(
  new Local(),
  new Remote(),

  {
    onClientConnect: () => {
      clients++;

      console.error(clients, "clients connected");
    },
    onClientDisconnect: () => {
      clients--;

      console.error(clients, "clients connected");
    },
  }
);

console.error(`Run one of the following commands to run a function on the remote(s):

- kill -SIGHUP ${process.pid}: Increment remote counter by one`);

process.on("SIGHUP", async () => {
  await registry.forRemotes(async (remoteID, remote) => {
    console.error("Calling functions for remote with ID", remoteID);

    try {
      await remote.Println(undefined, "Hello, world!");
    } catch (e) {
      console.error(`Got error for Increment func: ${e}`);
    }
  });
});

const encoder = new WritableStream({
  write(chunk) {
    return new Promise<void>((res) => {
      const isDrained = stdout.write(JSON.stringify(chunk));

      if (!isDrained) {
        stdout.once("drain", res);
      } else {
        res();
      }
    });
  },
  close() {
    return new Promise((res) => {
      stdout.end(res);
    });
  },
  abort(reason) {
    stdout.destroy(reason instanceof Error ? reason : new Error(reason));
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
stdin.on("data", (m) => parserWriter.write(m));
stdin.on("close", () => {
  parserReader.cancel();
  parserWriter.abort();
});

registry.linkStream(
  encoder,
  decoder,

  (v) => v,
  (v) => v
);

console.error("Connected to stdin and stdout");
