/* eslint-disable no-console */
import { stdin, stdout } from "process";
// eslint-disable-next-line import/no-extraneous-dependencies
import { JSONParser } from "@streamparser/json-whatwg";
import { ILocalContext, IRemoteContext, Registry } from "../index";

class Local {
  // eslint-disable-next-line class-methods-use-this
  async Println(ctx: ILocalContext, msg: string) {
    console.error("Printing message", msg, "for remote with ID", ctx.remoteID);

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

      console.error(clients, "clients connected");
    },
    onClientDisconnect: () => {
      clients--;

      console.error(clients, "clients connected");
    },
  }
);

console.error(`Run one of the following commands to run a function on the remote(s):

- kill -SIGHUP ${process.pid}: Increment remote counter by one
- kill -SIGUSR2 ${process.pid}: Decrement remote counter by one`);

process.on("SIGHUP", async () => {
  await registry.forRemotes(async (remoteID, remote) => {
    console.error("Calling functions for remote with ID", remoteID);

    try {
      const res = await remote.Increment(undefined, 1);

      console.error(res);
    } catch (e) {
      console.error(`Got error for Increment func: ${e}`);
    }
  });
});

process.on("SIGUSR2", async () => {
  await registry.forRemotes(async (remoteID, remote) => {
    console.error("Calling functions for remote with ID", remoteID);

    try {
      const res = await remote.Increment(undefined, -1);

      console.error(res);
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
