/* eslint-disable no-console */
import { env, exit } from "process";
import { parse } from "url";
// eslint-disable-next-line import/no-extraneous-dependencies
import { DecoderStream, EncoderStream } from "cbor-x";
import { Socket, createServer } from "net";
import { ILocalContext, Registry } from "../index";

class Local {
  constructor(private buf: number[]) {
    this.GetBytes = this.GetBytes.bind(this);
  }

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  async GetBytes(ctx: ILocalContext): Promise<number[]> {
    return this.buf;
  }
}

class Remote {}

const buffer = parseInt(env.BUFFER || `${1 * 128 * 1024}`, 10); // We can't do larger buffer sizes than this in TypeScript

let clients = 0;

const registry = new Registry(
  new Local(new Array(buffer)),
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

    const decoder = new DecoderStream({
      useRecords: false,
    });
    socket.pipe(decoder);

    const encoder = new EncoderStream({
      useRecords: false,
    });
    encoder.pipe(socket);

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

  const decoder = new DecoderStream({
    useRecords: false,
  });
  socket.pipe(decoder);

  const encoder = new EncoderStream({
    useRecords: false,
  });
  encoder.pipe(socket);

  registry.linkStream(
    encoder,
    decoder,

    (v) => v,
    (v) => v
  );

  console.log("Connected to", addr);
}
