/* eslint-disable no-console */
// eslint-disable-next-line import/no-extraneous-dependencies
import { JSONParser } from "@streamparser/json-whatwg";
// eslint-disable-next-line import/no-extraneous-dependencies
import { WebSocketServer } from "ws";
import {
  ILocalContext,
  IRemoteContext,
  Registry,
  remoteClosure,
} from "../index";

class CoffeeMachine {
  constructor(private supportedVariants: string[], private waterLevel: number) {
    this.BrewCoffee = this.BrewCoffee.bind(this);
  }

  async BrewCoffee(
    ctx: ILocalContext,
    variant: string,
    size: number,
    @remoteClosure
    onProgress: (ctx: IRemoteContext, percentage: number) => Promise<void>
  ): Promise<number> {
    if (!this.supportedVariants.includes(variant)) {
      throw new Error("unsupported variant");
    }

    if (this.waterLevel - size < 0) {
      throw new Error("not enough water");
    }

    console.log("Brewing coffee variant", variant, "in size", size, "ml");

    await onProgress(undefined, 0);

    await new Promise((r) => {
      setTimeout(r, 500);
    });
    await onProgress(undefined, 25);

    await new Promise((r) => {
      setTimeout(r, 500);
    });
    await onProgress(undefined, 50);

    await new Promise((r) => {
      setTimeout(r, 500);
    });
    await onProgress(undefined, 75);

    await new Promise((r) => {
      setTimeout(r, 500);
    });
    await onProgress(undefined, 100);

    this.waterLevel -= size;

    return this.waterLevel;
  }
}

class RemoteControl {
  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars, @typescript-eslint/no-empty-function
  async SetCoffeeMachineBrewing(ctx: IRemoteContext, brewing: boolean) {}
}

let clients = 0;

const registry = new Registry(
  new CoffeeMachine(["latte", "americano"], 1000),
  new RemoteControl(),

  {
    onClientConnect: () => {
      clients++;

      console.log(clients, "remote controls connected");
    },
    onClientDisconnect: () => {
      clients--;

      console.log(clients, "remote controls connected");
    },
  }
);

const server = new WebSocketServer({
  host: "localhost",
  port: 1337,
});

server.on("connection", (socket) => {
  socket.addEventListener("error", (e) => {
    console.error("Remote control disconnected with error:", e);
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
  socket.addEventListener("message", (m) =>
    parserWriter.write(m.data as string)
  );
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
});

console.log("Listening on localhost:1337");
