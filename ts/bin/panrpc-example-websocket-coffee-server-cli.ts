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
  public forRemotes?: (
    cb: (remoteID: string, remote: RemoteControl) => Promise<void>
  ) => Promise<void>;

  #waterLevel: number;

  constructor(private supportedVariants: string[], waterLevel: number) {
    this.#waterLevel = waterLevel;

    this.BrewCoffee = this.BrewCoffee.bind(this);
  }

  async BrewCoffee(
    ctx: ILocalContext,
    variant: string,
    size: number,
    @remoteClosure
    onProgress: (ctx: IRemoteContext, percentage: number) => Promise<void>
  ): Promise<number> {
    const { remoteID: targetID } = ctx;

    try {
      await this.forRemotes?.(async (remoteID, remote) => {
        if (remoteID === targetID) {
          return;
        }

        await remote.SetCoffeeMachineBrewing(undefined, true);
      });

      if (!this.supportedVariants.includes(variant)) {
        throw new Error("unsupported variant");
      }

      if (this.#waterLevel - size < 0) {
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
    } finally {
      await this.forRemotes?.(async (remoteID, remote) => {
        if (remoteID === targetID) {
          return;
        }

        await remote.SetCoffeeMachineBrewing(undefined, false);
      });
    }

    this.#waterLevel -= size;

    return this.#waterLevel;
  }
}

class RemoteControl {
  // eslint-disable-next-line class-methods-use-this, @typescript-eslint/no-unused-vars, @typescript-eslint/no-empty-function
  async SetCoffeeMachineBrewing(ctx: IRemoteContext, brewing: boolean) {}
}

const service = new CoffeeMachine(["latte", "americano"], 1000);

let clients = 0;

const registry = new Registry(
  service,
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

service.forRemotes = registry.forRemotes;

// Create WebSocket server
const server = new WebSocketServer({
  host: "127.0.0.1",
  port: 1337,
});

server.on("connection", (socket) => {
  socket.addEventListener("error", (e) => {
    console.error("Remote control disconnected with error:", e);
  });

  const linkSignal = new AbortController();

  // Set up streaming JSON encoder
  const encoder = new WritableStream({
    write(chunk) {
      socket.send(JSON.stringify(chunk));
    },
  });

  // Set up streaming JSON decoder
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

console.log("Listening on localhost:1337");
