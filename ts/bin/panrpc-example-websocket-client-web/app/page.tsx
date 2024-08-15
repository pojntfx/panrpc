"use client";

import { JSONParser } from "@streamparser/json-whatwg";
import { ILocalContext, IRemoteContext, Registry } from "panrpc";
import { env } from "process";
import { useEffect, useState } from "react";
import useAsyncEffect from "use-async";

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

const addr = env.ADDR || "127.0.0.1:1337";

export default function Home() {
  const [clients, setClients] = useState(0);
  useEffect(() => console.log(clients, "clients connected"), [clients]);

  const [registry] = useState(
    new Registry(
      new Local(),
      new Remote(),

      {
        onClientConnect: () => setClients((v) => v + 1),
        onClientDisconnect: () => setClients((v) => v - 1),
      }
    )
  );

  useAsyncEffect(async () => {
    const socket = new WebSocket(`ws://${addr}`);

    socket.addEventListener("error", (e) => {
      console.error("Disconnected with error:", e);
    });

    await new Promise<void>((res, rej) => {
      socket.addEventListener("open", () => res());
      socket.addEventListener("error", rej);
    });

    const linkSignal = new AbortController();

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

    return () => socket.close();
  }, []);

  const [log, setLog] = useState<string[]>([]);

  return clients > 0 ? (
    <main>
      <h1>panrpc WebSocket Client Example (Web)</h1>

      <section>
        <h2>Actions</h2>

        <button
          onClick={() =>
            registry.forRemotes(async (remoteID, remote) => {
              setLog((v) => [
                ...v,
                "Calling functions for remote with ID " + remoteID,
              ]);

              try {
                const res = await remote.Increment(undefined, 1);

                setLog((v) => [...v, `${res}`]);
              } catch (e) {
                console.error(`Got error for Increment func: ${e}`);
              }
            })
          }
        >
          Increment remote counter by one
        </button>
        <button
          onClick={() =>
            registry.forRemotes(async (remoteID, remote) => {
              setLog((v) => [
                ...v,
                "Calling functions for remote with ID " + remoteID,
              ]);

              try {
                const res = await remote.Increment(undefined, -1);

                setLog((v) => [...v, `${res}`]);
              } catch (e) {
                console.error(`Got error for Increment func: ${e}`);
              }
            })
          }
        >
          Decrement remote counter by one
        </button>
      </section>

      <section>
        <h2>Log</h2>

        <textarea
          style={{ width: "100%" }}
          rows={20}
          value={log.join("\n")}
        ></textarea>
      </section>
    </main>
  ) : (
    "Connecting ..."
  );
}
