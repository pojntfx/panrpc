/* eslint-disable no-console */
import { exit, stdin, stdout } from "process";
import { createInterface } from "readline/promises";
// eslint-disable-next-line import/no-extraneous-dependencies
import { JSONParser } from "@streamparser/json-whatwg";
// eslint-disable-next-line import/no-extraneous-dependencies
import { WebSocket } from "ws";
import { ILocalContext, IRemoteContext, Registry } from "../index";

class RemoteControl {
  // eslint-disable-next-line class-methods-use-this
  async SetCoffeeMachineBrewing(ctx: ILocalContext, brewing: boolean) {
    if (brewing) {
      console.log("Coffee machine is now brewing");
    } else {
      console.log("Coffee machine has stopped brewing");
    }
  }
}

class CoffeeMachine {
  // eslint-disable-next-line class-methods-use-this
  async BrewCoffee(
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    ctx: IRemoteContext,
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    variant: string,
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    size: number,
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    onProgress: (ctx: ILocalContext, percentage: number) => Promise<void>
  ): Promise<number> {
    return 0;
  }
}

let clients = 0;

const registry = new Registry(
  new RemoteControl(),
  new CoffeeMachine(),

  {
    onClientConnect: () => {
      clients++;

      console.log(clients, "coffee machines connected");
    },
    onClientDisconnect: () => {
      clients--;

      console.log(clients, "coffee machines connected");
    },
  }
);

(async () => {
  console.log(`Enter one of the following numbers followed by <ENTER> to brew a coffee:

- 1: Brew small Cafè Latte
- 2: Brew large Cafè Latte

- 3: Brew small Americano
- 4: Brew large Americano`);

  const rl = createInterface({ input: stdin, output: stdout });

  // eslint-disable-next-line no-constant-condition
  while (true) {
    const line =
      // eslint-disable-next-line no-await-in-loop
      await rl.question("");

    // eslint-disable-next-line no-await-in-loop
    await registry.forRemotes(async (remoteID, remote) => {
      switch (line) {
        case "1":
        case "2":
          try {
            // eslint-disable-next-line no-await-in-loop
            const res = await remote.BrewCoffee(
              undefined,
              "latte",
              line === "1" ? 100 : 200,
              async (ctx, percentage) =>
                console.log(`Brewing Cafè Latte ... ${percentage}% done`)
            );

            console.log("Remaining water:", res, "ml");
          } catch (e) {
            console.error(`Couldn't brew Cafè Latte: ${e}`);
          }

          break;

        case "3":
        case "4":
          try {
            // eslint-disable-next-line no-await-in-loop
            const res = await remote.BrewCoffee(
              undefined,
              "americano",
              line === "3" ? 100 : 200,
              async (ctx, percentage) =>
                console.log(`Brewing Americano ... ${percentage}% done`)
            );

            console.log("Remaining water:", res, "ml");
          } catch (e) {
            console.error(`Couldn't brew Americano: ${e}`);
          }

          break;

        default:
          console.log(`Unknown letter ${line}, ignoring input`);
      }
    });
  }
})();

const socket = new WebSocket("ws://localhost:1337");

socket.addEventListener("error", (e) => {
  console.error("Disconnected with error:", e);

  exit(1);
});
socket.addEventListener("close", () => exit(0));

await new Promise<void>((res, rej) => {
  socket.addEventListener("open", () => res());
  socket.addEventListener("error", rej);
});

const encoder = new WritableStream({
  write(chunk) {
    socket.send(JSON.stringify(chunk));
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
socket.addEventListener("message", (m) => parserWriter.write(m.data as string));
socket.addEventListener("close", () => {
  parserReader.cancel();
  parserWriter.abort();
});

registry.linkStream(
  encoder,
  decoder,

  (v) => v,
  (v) => v
);

console.log("Connected to localhost:1337");
